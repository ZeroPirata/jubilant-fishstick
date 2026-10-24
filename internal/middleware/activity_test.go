package middleware

import (
	"testing"
	"time"
)

func TestActivityTracker_UmaEscritaPorIntervalo(t *testing.T) {
	now := time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)
	tr := newActivityTracker(nil, 5*time.Minute, func() time.Time { return now })

	if !tr.due("u1") {
		t.Fatal("primeira requisição deveria gravar")
	}
	if tr.due("u1") {
		t.Error("segunda requisição no mesmo instante não deveria gravar")
	}
	if !tr.due("u2") {
		t.Error("outro usuário tem o próprio intervalo")
	}

	now = now.Add(4 * time.Minute)
	if tr.due("u1") {
		t.Error("antes de 5 min não deveria gravar")
	}
	now = now.Add(time.Minute)
	if !tr.due("u1") {
		t.Error("com 5 min passados deveria gravar de novo")
	}
}
