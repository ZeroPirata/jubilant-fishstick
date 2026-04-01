package postgres

func GetPoolStats() map[string]any {
	if dbPool == nil {
		return map[string]any{"error": "pool não inicializado"}
	}

	stat := dbPool.Stat()
	return map[string]any{
		"total_conns":            stat.TotalConns(),
		"idle_conns":             stat.IdleConns(),
		"acquired_conns":         stat.AcquiredConns(),
		"constructing_conns":     stat.ConstructingConns(),
		"max_conns":              stat.MaxConns(),
		"acquire_count":          stat.AcquireCount(),
		"acquire_duration":       stat.AcquireDuration(),
		"canceled_acquire_count": stat.CanceledAcquireCount(),
	}
}
