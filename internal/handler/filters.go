package handler

import (
	"context"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository"
	"hackton-treino/internal/repository/filters"
	"hackton-treino/internal/util"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type FilterHandler struct {
	*BaseHandler
	Filters filters.Repository
}

func NewFilterHandler(logger *zap.Logger, conn *pgxpool.Pool) *FilterHandler {
	return &FilterHandler{
		BaseHandler: NewBaseHandler(logger),
		Filters:     filters.New(conn),
	}
}

func (h *FilterHandler) ListFilters(w http.ResponseWriter, r *http.Request) {
	GenericList(
		func(userID pgtype.UUID, q PaginationParams) db.QuerySelectFiltersForUserWithIDParams {
			return db.QuerySelectFiltersForUserWithIDParams{
				UserID: userID,
				Cursor: q.Cursor,
				Size:   q.Size,
			}
		},
		h.Filters.QuerySelectFiltersForUserWithID,
		func(row db.QuerySelectFiltersForUserWithIDRow) Filter {
			return Filter{
				Base: Base{
					ID: row.ID.String(),
				},
				UserID:  row.UserID.String(),
				Keyword: row.Keyword,
			}
		},
		func(j db.QuerySelectFiltersForUserWithIDRow) int32 { return int32(j.TotalCount) },
	)(w, r)
}

func (h *FilterHandler) InsertFilter(w http.ResponseWriter, r *http.Request) {
	GenericCreate(
		func(ctx context.Context, userID pgtype.UUID, body Filter) *repository.RepositoryError {
			_, err := h.Filters.QueryInsertFilter(ctx, db.QueryInsertFilterParams{
				UserID:  userID,
				Keyword: body.Keyword,
			})
			return err
		},
		TypedValidate(TypedRequired[Filter]("keyword")),
	)(w, r)
}

func (h *FilterHandler) DeleteFilter(w http.ResponseWriter, r *http.Request) {
	GenericDelete(
		func(userID pgtype.UUID, r *http.Request) (db.QueryDeleteFilterParams, *repository.RepositoryError) {
			id, errR := util.ParseUUID(r.PathValue("id"))
			return db.QueryDeleteFilterParams{ID: id, UserID: userID}, errR
		},
		h.Filters.QueryDeleteFilter,
	)(w, r)
}
