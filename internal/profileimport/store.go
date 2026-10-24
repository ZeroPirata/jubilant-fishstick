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
)

var (
	ErrBusy     = errors.New("já existe um currículo sendo lido; aguarde terminar")
	ErrQuota    = errors.New("limite de envios atingido; tente de novo em uma hora")
	ErrNotFound = errors.New("importação não encontrada ou expirada; envie o arquivo de novo")
)

type State struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	Draft  *Draft `json:"draft,omitempty"`
}

type Store struct {
	rds *redis.Client
}

func NewStore(rds *redis.Client) *Store { return &Store{rds: rds} }

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
		err = s.save(ctx, userID, id, State{Status: StatusProcessing})
	}
	if err != nil {
		s.rds.Del(ctx, lockKey(userID))
	}
	return err
}

// Finish grava o resultado e libera a vez do usuário.
func (s *Store) Finish(ctx context.Context, userID, id string, st State) error {
	defer s.rds.Del(ctx, lockKey(userID))
	return s.save(ctx, userID, id, st)
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
	err = json.Unmarshal(raw, &st)
	return st, err
}

// Delete apaga o rascunho depois de aplicado: os dados pessoais não ficam no Redis à toa.
func (s *Store) Delete(ctx context.Context, userID, id string) {
	s.rds.Del(ctx, stateKey(userID, id))
}

func (s *Store) save(ctx context.Context, userID, id string, st State) error {
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return s.rds.Set(ctx, stateKey(userID, id), raw, stateTTL).Err()
}
