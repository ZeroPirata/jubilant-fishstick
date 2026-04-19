package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

func Load() (*Config, error) {
	_ = godotenv.Load()

	var cfg Config
	if err := parseEnv(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func parseEnv(cfg any) error {
	ptr := reflect.ValueOf(cfg)
	if ptr.Kind() != reflect.Pointer {
		return errors.New("expected a pointer to a struct")
	}

	val := ptr.Elem()
	if val.Kind() != reflect.Struct {
		return errors.New("expected a pointer to a struct")
	}

	for i := range val.NumField() {
		field := val.Type().Field(i)
		fieldValue := val.Field(i)

		if fieldValue.Kind() == reflect.Struct {
			if err := parseEnv(fieldValue.Addr().Interface()); err != nil {
				return err
			}
			continue
		}

		tag := field.Tag.Get("env")
		if tag == "" {
			continue
		}

		tagParts := strings.Split(tag, ",")
		envKey := tagParts[0]

		var isRequired bool
		defaultValue := ""

		if len(tagParts) > 1 {
			for _, part := range tagParts[1:] {
				if part == "required" {
					isRequired = true
				}
				if after, ok := strings.CutPrefix(part, "envDefault="); ok {
					defaultValue = after
				}
			}
		}

		envValue, found := os.LookupEnv(envKey)
		if !found {
			if isRequired {
				return fmt.Errorf("required environment variable not set: %s", envKey)
			}
			envValue = defaultValue
			// Fall back to the separate envDefault struct tag if nothing else set a value
			if envValue == "" {
				envValue = field.Tag.Get("envDefault")
			}
		}

		if envValue == "" {
			continue
		}

		switch fieldValue.Kind() {
		case reflect.String:
			fieldValue.SetString(envValue)
		case reflect.Bool:
			b, err := strconv.ParseBool(envValue)
			if err != nil {
				return fmt.Errorf("could not parse env var %s as bool: %w", envKey, err)
			}
			fieldValue.SetBool(b)
		case reflect.Int, reflect.Int64:
			if fieldValue.Type().String() == "time.Duration" {
				d, err := time.ParseDuration(envValue)
				if err != nil {
					return fmt.Errorf("invalid duration %s: %w", envKey, err)
				}
				fieldValue.SetInt(int64(d))
			} else {
				i, err := strconv.ParseInt(envValue, 10, 64)
				if err != nil {
					return fmt.Errorf("could not parse %s as int: %w", envKey, err)
				}
				fieldValue.SetInt(i)
			}
		case reflect.Uint, reflect.Uint64:
			u, err := strconv.ParseUint(envValue, 10, 64)
			if err != nil {
				return fmt.Errorf("could not parse %s as uint: %w", envKey, err)
			}
			fieldValue.SetUint(u)
		case reflect.Uint32:
			u, err := strconv.ParseUint(envValue, 10, 32)
			if err != nil {
				return fmt.Errorf("could not parse %s as uint32: %w", envKey, err)
			}
			fieldValue.SetUint(u)
		case reflect.Uint8:
			u, err := strconv.ParseUint(envValue, 10, 8)
			if err != nil {
				return fmt.Errorf("could not parse %s as uint8: %w", envKey, err)
			}
			fieldValue.SetUint(u)
		}
	}

	return nil
}
