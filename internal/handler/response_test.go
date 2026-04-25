package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// =============================================================================
// getPaginationParams
// =============================================================================

func TestGetPaginationParams_Defaults(t *testing.T) {
	// Sem query params → offset=0, size=10
	req := httptest.NewRequest(http.MethodGet, "/items", nil)
	offset, size := getPaginationParams(req)

	if offset != 0 {
		t.Errorf("offset = %d, want 0", offset)
	}
	if size != 10 {
		t.Errorf("size = %d, want 10", size)
	}
}

func TestGetPaginationParams_ValoresExplicitos(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/items?offset=20&size=5", nil)
	offset, size := getPaginationParams(req)

	if offset != 20 {
		t.Errorf("offset = %d, want 20", offset)
	}
	if size != 5 {
		t.Errorf("size = %d, want 5", size)
	}
}

func TestGetPaginationParams_ValoresInvalidos(t *testing.T) {
	// Strings não numéricas → usa defaults (0 e 10)
	req := httptest.NewRequest(http.MethodGet, "/items?offset=abc&size=xyz", nil)
	offset, size := getPaginationParams(req)

	if offset != 0 {
		t.Errorf("offset com valor inválido = %d, want 0", offset)
	}
	if size != 10 {
		t.Errorf("size com valor inválido = %d, want 10", size)
	}
}

func TestGetPaginationParams_SoUmParametro(t *testing.T) {
	// Só size informado → offset usa default
	req := httptest.NewRequest(http.MethodGet, "/items?size=25", nil)
	offset, size := getPaginationParams(req)

	if offset != 0 {
		t.Errorf("offset = %d, want 0", offset)
	}
	if size != 25 {
		t.Errorf("size = %d, want 25", size)
	}
}

func TestGetPaginationParams_Zero(t *testing.T) {
	// offset=0 explícito é válido (primeira página)
	req := httptest.NewRequest(http.MethodGet, "/items?offset=0&size=10", nil)
	offset, size := getPaginationParams(req)

	if offset != 0 {
		t.Errorf("offset = %d, want 0", offset)
	}
	if size != 10 {
		t.Errorf("size = %d, want 10", size)
	}
}

// =============================================================================
// getSearchParam
// =============================================================================

func TestGetSearchParam_Presente(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/items?search=golang", nil)
	got := getSearchParam(req)

	if got == nil {
		t.Fatal("getSearchParam: esperava ponteiro, recebeu nil")
	}
	if *got != "golang" {
		t.Errorf("*search = %q, want %q", *got, "golang")
	}
}

func TestGetSearchParam_Ausente(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/items", nil)
	got := getSearchParam(req)

	if got != nil {
		t.Errorf("getSearchParam: esperava nil, recebeu %q", *got)
	}
}

func TestGetSearchParam_Vazio(t *testing.T) {
	// search= sem valor → tratado como ausente
	req := httptest.NewRequest(http.MethodGet, "/items?search=", nil)
	got := getSearchParam(req)

	if got != nil {
		t.Errorf("getSearchParam com valor vazio: esperava nil, recebeu %q", *got)
	}
}

// =============================================================================
// writeJSON / writeError
// =============================================================================

func TestWriteJSON_StatusEContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusCreated, map[string]string{"id": "123"})

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestWriteJSON_EnvelopeData(t *testing.T) {
	// writeJSON sempre embrulha em {"data": ...}
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, "payload")

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["data"]; !ok {
		t.Error("resposta não contém campo \"data\"")
	}
}

func TestWriteError_EnvelopeError(t *testing.T) {
	// writeError sempre embrulha em {"error": "..."}
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "campo obrigatório")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "campo obrigatório" {
		t.Errorf("error = %q, want \"campo obrigatório\"", body["error"])
	}
}

func TestWriteList_IncluiMeta(t *testing.T) {
	// writeList deve incluir o campo "meta" com paginação
	rec := httptest.NewRecorder()
	writeList(rec, http.StatusOK, []string{"a", "b"}, 2, 0, 10)

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["meta"]; !ok {
		t.Error("resposta não contém campo \"meta\"")
	}
	if _, ok := body["data"]; !ok {
		t.Error("resposta não contém campo \"data\"")
	}
}
