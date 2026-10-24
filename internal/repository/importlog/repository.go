// Package importlog guarda o histórico das importações de currículo (painel admin).
// Falhar ao gravar aqui nunca derruba a importação: quem chama só registra um aviso.
package importlog

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Attempt é uma tentativa da cadeia de IA, com o erro completo (só o admin vê).
type Attempt struct {
	Model    string  `json:"model"`
	Provider string  `json:"provider"`
	Status   string  `json:"status"`
	Seconds  float64 `json:"seconds"`
	Error    string  `json:"error,omitempty"`
}

// Counts é quanto a IA encontrou no currículo (sem o conteúdo).
type Counts struct {
	Experiences  int `json:"experiences"`
	Academic     int `json:"academic"`
	Skills       int `json:"skills"`
	Projects     int `json:"projects"`
	Certificates int `json:"certificates"`
	Languages    int `json:"languages"`
}

type Entry struct {
	ID         string     `json:"id"`
	UserEmail  string     `json:"user_email"`
	FileKind   string     `json:"file_kind"`
	FileSize   int        `json:"file_size"`
	Status     string     `json:"status"` // processing | done | error | canceled | interrupted
	Error      string     `json:"error,omitempty"`
	Attempts   []Attempt  `json:"attempts"`
	Result     *Counts    `json:"result,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	AppliedAt  *time.Time `json:"applied_at,omitempty"`
}

type Repository struct {
	db *pgxpool.Pool
	// StaleAfter: "processando" há mais que isso morreu com o processo (vira interrupted).
	StaleAfter time.Duration
}

func New(db *pgxpool.Pool, staleAfter time.Duration) *Repository {
	return &Repository{db: db, StaleAfter: staleAfter}
}

func (r *Repository) Start(ctx context.Context, id, userID, kind string, size int) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO profile_imports (id, user_id, file_kind, file_size) VALUES ($1, $2::uuid, $3, $4)`,
		id, userID, kind, size)
	return err
}

func (r *Repository) SetAttempts(ctx context.Context, id string, attempts []Attempt) error {
	raw, err := json.Marshal(attempts)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `UPDATE profile_imports SET attempts = $2 WHERE id = $1`, id, raw)
	return err
}

// Finish grava o resultado; uma importação já cancelada continua cancelada.
func (r *Repository) Finish(ctx context.Context, id, status, errMsg string, result *Counts) error {
	var raw []byte
	if result != nil {
		var err error
		if raw, err = json.Marshal(result); err != nil {
			return err
		}
	}
	_, err := r.db.Exec(ctx,
		`UPDATE profile_imports SET status = $2, error = NULLIF($3, ''), result = $4, finished_at = now()
		 WHERE id = $1 AND status = 'processing'`,
		id, status, errMsg, raw)
	return err
}

func (r *Repository) Cancel(ctx context.Context, id, userID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE profile_imports SET status = 'canceled', finished_at = COALESCE(finished_at, now())
		 WHERE id = $1 AND user_id = $2::uuid AND status IN ('processing', 'done')`,
		id, userID)
	return err
}

func (r *Repository) Applied(ctx context.Context, id, userID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE profile_imports SET applied_at = now() WHERE id = $1 AND user_id = $2::uuid`, id, userID)
	return err
}

// List devolve as importações mais recentes, com o e-mail de quem enviou.
func (r *Repository) List(ctx context.Context, limit int) ([]Entry, error) {
	rows, err := r.db.Query(ctx,
		`SELECT p.id, COALESCE(u.email, ''), p.file_kind, p.file_size,
		        CASE WHEN p.status = 'processing' AND p.started_at < now() - make_interval(secs => $2) THEN 'interrupted' ELSE p.status END,
		        COALESCE(p.error, ''), p.attempts, p.result, p.started_at, p.finished_at, p.applied_at
		 FROM profile_imports p LEFT JOIN user_accounts u ON u.id = p.user_id
		 ORDER BY p.started_at DESC LIMIT $1`,
		limit, r.StaleAfter.Seconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Entry{}
	for rows.Next() {
		var (
			e                Entry
			attempts, result []byte
		)
		if err := rows.Scan(&e.ID, &e.UserEmail, &e.FileKind, &e.FileSize, &e.Status, &e.Error,
			&attempts, &result, &e.StartedAt, &e.FinishedAt, &e.AppliedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(attempts, &e.Attempts); err != nil {
			return nil, err
		}
		if len(result) > 0 {
			e.Result = &Counts{}
			if err := json.Unmarshal(result, e.Result); err != nil {
				return nil, err
			}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Prune apaga o que passou da retenção.
func (r *Repository) Prune(ctx context.Context, retention time.Duration) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM profile_imports WHERE started_at < $1`, time.Now().Add(-retention))
	return tag.RowsAffected(), err
}
