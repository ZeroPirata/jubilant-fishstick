package handler

import (
	"encoding/json"
	"hackton-treino/internal/db"
	"hackton-treino/internal/repository/users"
	"hackton-treino/internal/security"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type AuthHandler struct {
	*BaseHandler
	Users  users.Repository
	Hasher *security.Hasher
	Jwt    security.TokenProvider
}

func NewAuthHandler(logger *zap.Logger, conn *pgxpool.Pool, hasher *security.Hasher, jwt security.TokenProvider) *AuthHandler {
	return &AuthHandler{
		BaseHandler: NewBaseHandler(logger),
		Users:       users.New(conn),
		Hasher:      hasher,
		Jwt:         jwt,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {

	var req Auth
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.Logger.Error("failed to decode register request", zap.Error(err))
		writeError(w, http.StatusBadRequest, ErrInvalidInputData.Error())
		return
	}

	if err := Validate(
		Required("email", req.Email),
		ValidateEmail("email", req.Email),
		Required("password", req.Password),
		MinLength("password", req.Password, 8),
	); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	hashedPassword, err := h.Hasher.Hash(req.Password)
	if err != nil {
		h.Logger.Error("failed to hash password", zap.Error(err))
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}

	row, errR := h.Users.QueryInsertAccount(r.Context(), db.QueryInsertAccountParams{Email: req.Email, PasswordHash: hashedPassword})
	if errR != nil {
		h.Logger.Error("failed to insert account", zap.Error(errR))
		writeError(w, http.StatusConflict, "email already in use")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"id": row.ID})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {

	var req Auth
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.Logger.Error("failed to decode login request", zap.Error(err))
		writeError(w, http.StatusBadRequest, ErrInvalidInputData.Error())
		return
	}

	if err := Validate(
		Required("email", req.Email),
		ValidateEmail("email", req.Email),
		Required("password", req.Password),
	); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	row, errR := h.Users.QuerySelectAccountByEmail(r.Context(), req.Email)
	if errR != nil {
		h.Logger.Error("failed to get user by email", zap.Error(errR))
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}

	valid, err := h.Hasher.Verify(req.Password, row.PasswordHash)
	if err != nil {
		h.Logger.Error("failed to verify password", zap.Error(err))
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}
	if !valid {
		writeError(w, http.StatusUnauthorized, ErrNotAuthorized.Error())
		return
	}

	token, err := h.Jwt.Generate(row.ID.String())
	if err != nil {
		h.Logger.Error("failed to generate token", zap.Error(err))
		writeError(w, http.StatusInternalServerError, ErrInternalServerError.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"token": token})
}
