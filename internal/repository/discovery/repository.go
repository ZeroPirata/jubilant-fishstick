package discovery

import (
	"context"
	"errors"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"

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

func (r *discoveryRepository) DeleteStale(ctx context.Context, cutoff pgtype.Timestamptz) (int64, *repository.RepositoryError) {
	n, err := r.Q.DiscoveryDeleteStale(ctx, cutoff)
	return n, repository.HandleDatabaseError(err)
}

func (r *discoveryRepository) ListForUser(ctx context.Context, args db.DiscoveryListForUserParams) ([]db.DiscoveryListForUserRow, *repository.RepositoryError) {
	rows, err := r.Q.DiscoveryListForUser(ctx, args)
	return rows, repository.HandleDatabaseError(err)
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
