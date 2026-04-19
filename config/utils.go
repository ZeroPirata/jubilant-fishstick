package config

import (
	"time"
)

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

func GetUnsignedIntOrDefault(configValue uint32, defaultValue uint32) uint32 {
	if configValue > 0 {
		return configValue
	}
	return defaultValue
}

func GetUnsignedInt8OrDefault(configValue uint8, defaultValue uint8) uint8 {
	if configValue > 0 {
		return configValue
	}
	return defaultValue
}

func GetStringOrDefault(configValue string, defaultValue string) string {
	if configValue != "" {
		return configValue
	}
	return defaultValue
}
