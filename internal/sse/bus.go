package sse

import (
	"hackton-treino/internal/jobfit"
	"hackton-treino/internal/resume"
	"sync"
)

type JobEvent struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Quality     string `json:"quality,omitempty"`
	CompanyName string `json:"company_name,omitempty"`
	JobTitle    string `json:"job_title,omitempty"`
	// Fit e Analysis vão no evento de concluído: a aba Qualidade abre sem recarregar.
	Fit      *jobfit.Fit         `json:"fit,omitempty"`
	Analysis *resume.FitAnalysis `json:"analysis,omitempty"`
}

type Bus struct {
	mu      sync.RWMutex
	clients map[string]map[chan JobEvent]struct{}
}

func NewBus() *Bus {
	return &Bus{clients: make(map[string]map[chan JobEvent]struct{})}
}

func (b *Bus) Subscribe(userID string) chan JobEvent {
	ch := make(chan JobEvent, 8)
	b.mu.Lock()
	if b.clients[userID] == nil {
		b.clients[userID] = make(map[chan JobEvent]struct{})
	}
	b.clients[userID][ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Bus) Unsubscribe(userID string, ch chan JobEvent) {
	b.mu.Lock()
	delete(b.clients[userID], ch)
	if len(b.clients[userID]) == 0 {
		delete(b.clients, userID)
	}
	b.mu.Unlock()
	close(ch)
}

func (b *Bus) Publish(userID string, event JobEvent) {
	b.mu.RLock()
	channels := b.clients[userID]
	b.mu.RUnlock()
	for ch := range channels {
		select {
		case ch <- event:
		default:
		}
	}
}
