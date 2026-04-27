package admin

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

func (r *adminRepo) InsertErrorLog(ctx context.Context, p InsertErrorLogParams) {
	jobID := pgtype.UUID{}
	if p.JobID != nil {
		_ = jobID.Scan(*p.JobID)
	}
	userID := pgtype.UUID{}
	if p.UserID != nil {
		_ = userID.Scan(*p.UserID)
	}
	url := pgtype.Text{}
	if p.URL != nil {
		url = pgtype.Text{String: *p.URL, Valid: true}
	}

	_, err := r.db.Exec(ctx,
		`INSERT INTO error_logs (job_id, user_id, error_type, error_message, url)
		 VALUES ($1, $2, $3, $4, $5)`,
		jobID, userID, p.ErrorType, p.ErrorMessage, url,
	)
	if err != nil {
		zap.L().Warn("admin: falha ao inserir error_log", zap.Error(err))
	}
}

func (r *adminRepo) ListErrorLogs(ctx context.Context, limit, offset int) ([]ErrorLog, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM error_logs`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx,
		`SELECT el.id, el.job_id, ua.email, el.error_type, el.error_message, el.url, el.created_at
		 FROM error_logs el
		 LEFT JOIN user_accounts ua ON ua.id = el.user_id
		 ORDER BY el.created_at DESC
		 LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []ErrorLog
	for rows.Next() {
		var l ErrorLog
		var jobID pgtype.UUID
		var email pgtype.Text
		var url pgtype.Text
		if err := rows.Scan(&l.ID, &jobID, &email, &l.ErrorType, &l.ErrorMessage, &url, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		if jobID.Valid {
			s := jobID.Bytes[:]
			formatted := formatUUID(jobID.Bytes)
			_ = s
			l.JobID = &formatted
		}
		if email.Valid {
			l.UserEmail = &email.String
		}
		if url.Valid {
			l.URL = &url.String
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}

func (r *adminRepo) ListUsers(ctx context.Context) ([]UserAccount, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, email, is_admin, created_at
		 FROM user_accounts
		 WHERE deleted_at IS NULL
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []UserAccount
	for rows.Next() {
		var u UserAccount
		var id pgtype.UUID
		var createdAt pgtype.Timestamptz
		if err := rows.Scan(&id, &u.Email, &u.IsAdmin, &createdAt); err != nil {
			return nil, err
		}
		u.ID = formatUUID(id.Bytes)
		u.CreatedAt = createdAt.Time
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *adminRepo) SetAdmin(ctx context.Context, userID string, isAdmin bool) error {
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx,
		`UPDATE user_accounts SET is_admin = $1, updated_at = NOW() WHERE id = $2`,
		isAdmin, uid,
	)
	return err
}

func (r *adminRepo) IsAdmin(ctx context.Context, userID string) (bool, error) {
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		return false, err
	}
	var isAdmin bool
	err := r.db.QueryRow(ctx,
		`SELECT is_admin FROM user_accounts WHERE id = $1 AND deleted_at IS NULL`, uid,
	).Scan(&isAdmin)
	return isAdmin, err
}

func formatUUID(b [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
