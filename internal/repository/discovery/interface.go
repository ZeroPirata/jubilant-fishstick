package discovery

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type discoveryRepository struct {
	repository.Base
}

type Repository interface {
	SelectAllKeywords(ctx context.Context) ([]string, *repository.RepositoryError)
	SelectAllTechnologies(ctx context.Context) ([]string, *repository.RepositoryError)
	SelectUserKeywords(ctx context.Context, userID pgtype.UUID) ([]string, *repository.RepositoryError)
	UpsertJob(ctx context.Context, args db.DiscoveryUpsertJobParams) (db.DiscoveryUpsertJobRow, *repository.RepositoryError)
	LinkKeyword(ctx context.Context, args db.DiscoveryLinkKeywordParams) *repository.RepositoryError
	DeleteStale(ctx context.Context, args db.DiscoveryDeleteStaleParams) ([]db.DiscoveryDeleteStaleRow, *repository.RepositoryError)
	ListCandidates(ctx context.Context, args db.DiscoveryListCandidatesParams) ([]db.DiscoveryListCandidatesRow, *repository.RepositoryError)
	SkillHits(ctx context.Context, args db.DiscoverySkillHitsParams) ([]db.DiscoverySkillHitsRow, *repository.RepositoryError)
	PageDetails(ctx context.Context, args db.DiscoveryPageDetailsParams) ([]db.DiscoveryPageDetailsRow, *repository.RepositoryError)
	SelectTechsPending(ctx context.Context, args db.DiscoverySelectTechsPendingParams) ([]db.DiscoverySelectTechsPendingRow, *repository.RepositoryError)
	UpdateTechs(ctx context.Context, args db.DiscoveryUpdateTechsParams) *repository.RepositoryError
	CleanIndexPending(ctx context.Context) *repository.RepositoryError
	SelectSettings(ctx context.Context) (db.DiscoverySelectSettingsRow, *repository.RepositoryError)
	UpdateSettings(ctx context.Context, args db.DiscoveryUpdateSettingsParams) *repository.RepositoryError
	InsertRun(ctx context.Context, args db.DiscoveryInsertRunParams) (pgtype.UUID, *repository.RepositoryError)
	FinishRun(ctx context.Context, args db.DiscoveryFinishRunParams) *repository.RepositoryError
	InsertRunSource(ctx context.Context, args db.DiscoveryInsertRunSourceParams) *repository.RepositoryError
	FailInterruptedRuns(ctx context.Context) (int64, *repository.RepositoryError)
	LastFullRun(ctx context.Context) (pgtype.Timestamptz, *repository.RepositoryError)
	ListRuns(ctx context.Context, maxCount int32) ([]db.DiscoveryListRunsRow, *repository.RepositoryError)
	ListRunSources(ctx context.Context, runIDs []pgtype.UUID) ([]db.DiscoveryListRunSourcesRow, *repository.RepositoryError)
	LastSourceResults(ctx context.Context) ([]db.DiscoveryLastSourceResultsRow, *repository.RepositoryError)
	CatalogStats(ctx context.Context, activeSince pgtype.Timestamptz) ([]db.DiscoveryCatalogStatsRow, *repository.RepositoryError)
	PruneRuns(ctx context.Context, cutoff pgtype.Timestamptz) (int64, *repository.RepositoryError)
	SelectPreferences(ctx context.Context, userID pgtype.UUID) (db.DiscoverySelectPreferencesRow, *repository.RepositoryError)
	UpsertPreferences(ctx context.Context, args db.DiscoveryUpsertPreferencesParams) *repository.RepositoryError
	SelectByURL(ctx context.Context, url string) (db.DiscoverySelectByURLRow, bool, *repository.RepositoryError)
	SelectAlerts(ctx context.Context, userID pgtype.UUID) (db.AlertsSelectRow, *repository.RepositoryError)
	UpsertAlertSettings(ctx context.Context, args db.AlertsUpsertSettingsParams) *repository.RepositoryError
	MarkAlertsSeen(ctx context.Context, userID pgtype.UUID) *repository.RepositoryError
}

func New(conn *pgxpool.Pool) Repository {
	return &discoveryRepository{Base: repository.NewBase(conn)}
}
