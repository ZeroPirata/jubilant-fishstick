package profileimport

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// O processamento roda em segundo plano e o front consulta o estado: com o fallback para
// um modelo lento a IA pode passar dos 100s que o túnel da Cloudflare segura uma resposta.
const (
	StatusProcessing = "processing"
	StatusDone       = "done"
	StatusError      = "error"

	stateTTL    = 30 * time.Minute // tempo para a pessoa conferir o rascunho
	lockTTL     = 8 * time.Minute  // um envio por vez por usuário (maior que o aiTimeout)
	quotaWindow = time.Hour
	quotaMax    = 10 // envios por hora por usuário: cada um custa uma chamada de IA

	// StaleAfter: "processando" há mais que isso é uma leitura que morreu com o processo
	// (deploy, restart do air). Maior que o aiTimeout, que sempre encerra a leitura viva.
	StaleAfter = aiTimeout + 90*time.Second
)

var (
	ErrBusy     = errors.New("já existe um currículo sendo lido; aguarde terminar ou cancele")
	ErrQuota    = errors.New("limite de envios atingido; tente de novo em uma hora")
	ErrNotFound = errors.New("importação não encontrada ou expirada; envie o arquivo de novo")
)

const msgInterrupted = "A leitura foi interrompida porque o servidor reiniciou. Envie o arquivo de novo."

// Attempt é uma tentativa da cadeia de IA, mostrada ao vivo na tela de processamento.
type Attempt struct {
	Model   string  `json:"model"`
	Status  string  `json:"status"` // ok | rate_limit | timeout | ... (services.ClassifyError)
	Seconds float64 `json:"seconds"`
}

type State struct {
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	Draft     *Draft    `json:"draft,omitempty"`
	StartedAt time.Time `json:"started_at"`
	Current   string    `json:"current,omitempty"`    // modelo tentando agora
	CurrentAt time.Time `json:"current_at,omitempty"` // desde quando
	Attempts  []Attempt `json:"attempts,omitempty"`
}

// UserView é o que a própria pessoa vê do andamento: quantas tentativas e como foram,
// sem o nome dos modelos (detalhe interno, só no painel admin).
type UserView struct {
	Status      string        `json:"status"`
	Error       string        `json:"error,omitempty"`
	Draft       *Draft        `json:"draft,omitempty"`
	StartedAt   time.Time     `json:"started_at"`
	Trying      bool          `json:"trying"`
	TryingSince *time.Time    `json:"trying_since,omitempty"`
	Attempts    []UserAttempt `json:"attempts"`
}

type UserAttempt struct {
	Status  string  `json:"status"`
	Seconds float64 `json:"seconds"`
}

func (st State) ForUser() UserView {
	v := UserView{Status: st.Status, Error: st.Error, Draft: st.Draft, StartedAt: st.StartedAt, Trying: st.Current != "",
		Attempts: make([]UserAttempt, 0, len(st.Attempts))}
	if v.Trying && !st.CurrentAt.IsZero() {
		at := st.CurrentAt
		v.TryingSince = &at
	}
	for _, a := range st.Attempts {
		v.Attempts = append(v.Attempts, UserAttempt{Status: a.Status, Seconds: a.Seconds})
	}
	return v
}

type Store struct {
	rds *redis.Client
	now func() time.Time
}

func NewStore(rds *redis.Client) *Store { return &Store{rds: rds, now: time.Now} }

func stateKey(userID, id string) string { return "profile_import:" + userID + ":" + id }
func lockKey(userID string) string      { return "profile_import_lock:" + userID }
func quotaKey(userID string) string     { return "profile_import_quota:" + userID }

// Begin reserva a vez do usuário e grava o estado inicial.
func (s *Store) Begin(ctx context.Context, userID, id string) error {
	ok, err := s.rds.SetNX(ctx, lockKey(userID), id, lockTTL).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrBusy
	}
	n, err := s.rds.Incr(ctx, quotaKey(userID)).Result()
	if err == nil && n == 1 {
		err = s.rds.Expire(ctx, quotaKey(userID), quotaWindow).Err()
	}
	if err == nil && n > quotaMax {
		err = ErrQuota
	}
	if err == nil {
		err = s.write(ctx, userID, id, State{Status: StatusProcessing, StartedAt: s.now()}, false)
	}
	if err != nil {
		s.release(ctx, userID, id)
	}
	return err
}

// Update grava o andamento só se a importação ainda existe (cancelada = não ressuscita).
func (s *Store) Update(ctx context.Context, userID, id string, st State) error {
	return s.write(ctx, userID, id, st, true)
}

// Finish grava o resultado (se não foi cancelada) e libera a vez do usuário.
func (s *Store) Finish(ctx context.Context, userID, id string, st State) error {
	defer s.release(ctx, userID, id)
	return s.write(ctx, userID, id, st, true)
}

// Cancel apaga a importação e libera a vez; o rascunho em andamento é descartado.
func (s *Store) Cancel(ctx context.Context, userID, id string) {
	s.rds.Del(ctx, stateKey(userID, id))
	s.release(ctx, userID, id)
}

func (s *Store) Get(ctx context.Context, userID, id string) (State, error) {
	raw, err := s.rds.Get(ctx, stateKey(userID, id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return State{}, ErrNotFound
	}
	if err != nil {
		return State{}, err
	}
	var st State
	if err := json.Unmarshal(raw, &st); err != nil {
		return State{}, err
	}
	// Leitura órfã (sem started_at = gravada antes desta versão, ou velha demais).
	if st.Status == StatusProcessing && (st.StartedAt.IsZero() || s.now().Sub(st.StartedAt) > StaleAfter) {
		st = State{Status: StatusError, Error: msgInterrupted, StartedAt: st.StartedAt, Attempts: st.Attempts}
		_ = s.write(ctx, userID, id, st, true)
		s.release(ctx, userID, id)
	}
	return st, nil
}

// Delete apaga o rascunho depois de aplicado: os dados pessoais não ficam no Redis à toa.
func (s *Store) Delete(ctx context.Context, userID, id string) {
	s.rds.Del(ctx, stateKey(userID, id))
}

func (s *Store) write(ctx context.Context, userID, id string, st State, onlyIfExists bool) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	if !onlyIfExists {
		return s.rds.Set(ctx, stateKey(userID, id), raw, stateTTL).Err()
	}
	err = s.rds.SetArgs(ctx, stateKey(userID, id), raw, redis.SetArgs{Mode: "XX", TTL: stateTTL}).Err()
	if errors.Is(err, redis.Nil) { // não existe mais: foi cancelada
		return nil
	}
	return err
}

// release libera a vez só se ela é desta importação (outra pode ter começado depois).
func (s *Store) release(ctx context.Context, userID, id string) {
	if cur, _ := s.rds.Get(ctx, lockKey(userID)).Result(); cur == id {
		s.rds.Del(ctx, lockKey(userID))
	}
}
