package handler

import (
	"encoding/json"
	"hackton-treino/internal/repository"
	"net/http"
	"strconv"
)

type meta struct {
	Size   int32 `json:"size"`
	Cursor int32 `json:"cursor"`
	Total  int32 `json:"total,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(map[string]any{"data": data})
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func writeList(w http.ResponseWriter, status int, data any, size, cursor, total int32) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(map[string]any{
		"data": data,
		"meta": meta{Size: size, Cursor: cursor, Total: total},
	})
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(map[string]string{"error": msg})
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func writeRepositoryError(w http.ResponseWriter, err *repository.RepositoryError) {
	writeError(w, err.StatusCode, err.Message)
}

func handlerRepositoryError(w http.ResponseWriter, err *repository.RepositoryError) {
	statusCode := err.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusInternalServerError
	}
	message := err.Message
	if message == "" {
		message = "Internal server error"
	}
	http.Error(w, message, statusCode)
}

func getPaginationParams(r *http.Request) (int32, int32) {
	offSize := r.URL.Query().Get("offset")
	if offSize == "" {
		offSize = "0"
	}
	size := r.URL.Query().Get("size")
	if size == "" {
		size = "10"
	}

	offSizeInt, err := strconv.Atoi(offSize)
	if err != nil {
		offSizeInt = 0
	}
	sizeInt, err := strconv.Atoi(size)
	if err != nil {
		sizeInt = 10
	}

	return int32(offSizeInt), int32(sizeInt)
}

func getSearchParam(r *http.Request) *string {
	s := r.URL.Query().Get("search")
	if s == "" {
		return nil
	}
	return &s
}
