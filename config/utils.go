package config

import "time"

func GetConfigIntOrDefault(configValue, defaultValue int) int {
	if configValue > 0 {
		return configValue
	}
	return defaultValue
}

func GetConfigDurationOrDefault(configValue time.Duration, defaultValue time.Duration) time.Duration {
	if configValue > 0 {
		return configValue
	}
	return defaultValue
}
