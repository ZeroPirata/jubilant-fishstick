package discovery

import (
	"context"
	"errors"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (r *discoveryRepository) SelectAllKeywords(ctx context.Context) ([]string, *repository.RepositoryError) {
	rows, err := r.Q.DiscoverySelectAllKeywords(ctx)
	return rows, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) SelectAllTechnologies(ctx context.Context) ([]string, *repository.RepositoryError) {
	rows, err := r.Q.DiscoverySelectAllTechnologies(ctx)
	return rows, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) SelectUserKeywords(ctx context.Context, userID pgtype.UUID) ([]string, *repository.RepositoryError) {
	rows, err := r.Q.DiscoverySelectUserKeywords(ctx, userID)
	return rows, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) UpsertJob(ctx context.Context, args db.DiscoveryUpsertJobParams) (db.DiscoveryUpsertJobRow, *repository.RepositoryError) {
	id, err := r.Q.DiscoveryUpsertJob(ctx, args)
	return id, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) LinkKeyword(ctx context.Context, args db.DiscoveryLinkKeywordParams) *repository.RepositoryError {
	return repository.HandleDatabaseError(r.Q.DiscoveryLinkKeyword(ctx, args))
}

func (r *discoveryRepository) DeleteStale(ctx context.Context, args db.DiscoveryDeleteStaleParams) ([]db.DiscoveryDeleteStaleRow, *repository.RepositoryError) {
	rows, err := r.Q.DiscoveryDeleteStale(ctx, args)
	return rows, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) ListCandidates(ctx context.Context, args db.DiscoveryListCandidatesParams) ([]db.DiscoveryListCandidatesRow, *repository.RepositoryError) {
	rows, err := r.Q.DiscoveryListCandidates(ctx, args)
	return rows, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) SkillHits(ctx context.Context, args db.DiscoverySkillHitsParams) ([]db.DiscoverySkillHitsRow, *repository.RepositoryError) {
	if len(args.Terms) == 0 {
		return nil, nil
	}
	rows, err := r.Q.DiscoverySkillHits(ctx, args)
	return rows, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) PageDetails(ctx context.Context, args db.DiscoveryPageDetailsParams) ([]db.DiscoveryPageDetailsRow, *repository.RepositoryError) {
	if len(args.Ids) == 0 {
		return nil, nil
	}
	rows, err := r.Q.DiscoveryPageDetails(ctx, args)
	return rows, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) SelectTechsPending(ctx context.Context, args db.DiscoverySelectTechsPendingParams) ([]db.DiscoverySelectTechsPendingRow, *repository.RepositoryError) {
	rows, err := r.Q.DiscoverySelectTechsPending(ctx, args)
	return rows, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) CleanIndexPending(ctx context.Context) *repository.RepositoryError {
	return repository.HandleDatabaseError(r.Q.DiscoveryCleanIndexPending(ctx))
}

func (r *discoveryRepository) UpdateTechs(ctx context.Context, args db.DiscoveryUpdateTechsParams) *repository.RepositoryError {
	return repository.HandleDatabaseError(r.Q.DiscoveryUpdateTechs(ctx, args))
}

// SelectPreferences devolve preferências vazias para quem nunca salvou.
func (r *discoveryRepository) SelectPreferences(ctx context.Context, userID pgtype.UUID) (db.DiscoverySelectPreferencesRow, *repository.RepositoryError) {
	row, err := r.Q.DiscoverySelectPreferences(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Mesmos valores padrão da tabela: sem filtro, vagas sem nível incluídas.
		return db.DiscoverySelectPreferencesRow{SeniorityUnknown: true}, nil
	}
	return row, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) UpsertPreferences(ctx context.Context, args db.DiscoveryUpsertPreferencesParams) *repository.RepositoryError {
	return repository.HandleDatabaseError(r.Q.DiscoveryUpsertPreferences(ctx, args))
}

func (r *discoveryRepository) SelectSettings(ctx context.Context) (db.DiscoverySelectSettingsRow, *repository.RepositoryError) {
	row, err := r.Q.DiscoverySelectSettings(ctx)
	return row, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) UpdateSettings(ctx context.Context, args db.DiscoveryUpdateSettingsParams) *repository.RepositoryError {
	return repository.HandleDatabaseError(r.Q.DiscoveryUpdateSettings(ctx, args))
}

func (r *discoveryRepository) InsertRun(ctx context.Context, args db.DiscoveryInsertRunParams) (pgtype.UUID, *repository.RepositoryError) {
	id, err := r.Q.DiscoveryInsertRun(ctx, args)
	return id, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) FinishRun(ctx context.Context, args db.DiscoveryFinishRunParams) *repository.RepositoryError {
	return repository.HandleDatabaseError(r.Q.DiscoveryFinishRun(ctx, args))
}

func (r *discoveryRepository) InsertRunSource(ctx context.Context, args db.DiscoveryInsertRunSourceParams) *repository.RepositoryError {
	return repository.HandleDatabaseError(r.Q.DiscoveryInsertRunSource(ctx, args))
}

func (r *discoveryRepository) FailInterruptedRuns(ctx context.Context) (int64, *repository.RepositoryError) {
	n, err := r.Q.DiscoveryFailInterruptedRuns(ctx)
	return n, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) LastFullRun(ctx context.Context) (pgtype.Timestamptz, *repository.RepositoryError) {
	ts, err := r.Q.DiscoveryLastFullRun(ctx)
	return ts, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) ListRuns(ctx context.Context, maxCount int32) ([]db.DiscoveryListRunsRow, *repository.RepositoryError) {
	rows, err := r.Q.DiscoveryListRuns(ctx, maxCount)
	return rows, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) ListRunSources(ctx context.Context, runIDs []pgtype.UUID) ([]db.DiscoveryListRunSourcesRow, *repository.RepositoryError) {
	rows, err := r.Q.DiscoveryListRunSources(ctx, runIDs)
	return rows, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) LastSourceResults(ctx context.Context) ([]db.DiscoveryLastSourceResultsRow, *repository.RepositoryError) {
	rows, err := r.Q.DiscoveryLastSourceResults(ctx)
	return rows, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) CatalogStats(ctx context.Context, activeSince pgtype.Timestamptz) ([]db.DiscoveryCatalogStatsRow, *repository.RepositoryError) {
	rows, err := r.Q.DiscoveryCatalogStats(ctx, activeSince)
	return rows, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) PruneRuns(ctx context.Context, cutoff pgtype.Timestamptz) (int64, *repository.RepositoryError) {
	n, err := r.Q.DiscoveryPruneRuns(ctx, cutoff)
	return n, repository.HandleDatabaseError(err)
}

// SelectByURL devolve false quando a URL não está no catálogo.
func (r *discoveryRepository) SelectByURL(ctx context.Context, url string) (db.DiscoverySelectByURLRow, bool, *repository.RepositoryError) {
	row, err := r.Q.DiscoverySelectByURL(ctx, url)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, false, nil
	}
	return row, err == nil, repository.HandleDatabaseError(err)
}

// SelectAlerts devolve a configuração dos alertas; sem linha, os padrões da tabela,
// com as últimas 24h como "novas" (o primeiro acesso não fica vazio).
func (r *discoveryRepository) SelectAlerts(ctx context.Context, userID pgtype.UUID) (db.AlertsSelectRow, *repository.RepositoryError) {
	row, err := r.Q.AlertsSelect(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return db.AlertsSelectRow{
			Enabled:    true,
			MinQuality: string(db.JobQualityHigh),
			SeenAt:     pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true},
		}, nil
	}
	return row, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) UpsertAlertSettings(ctx context.Context, args db.AlertsUpsertSettingsParams) *repository.RepositoryError {
	return repository.HandleDatabaseError(r.Q.AlertsUpsertSettings(ctx, args))
}

func (r *discoveryRepository) MarkAlertsSeen(ctx context.Context, userID pgtype.UUID) *repository.RepositoryError {
	return repository.HandleDatabaseError(r.Q.AlertsMarkSeen(ctx, userID))
}
