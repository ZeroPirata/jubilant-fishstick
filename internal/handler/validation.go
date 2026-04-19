package handler

import (
	"fmt"
	"reflect"
	"strings"
)

// TypedValidator é um validator que opera sobre um tipo T já decodificado.
type TypedValidator[T any] func(T) error

// TypedValidate combina múltiplos TypedValidator[T] e retorna uma função de validação.
func TypedValidate[T any](validators ...TypedValidator[T]) func(T) error {
	return func(body T) error {
		for _, v := range validators {
			if err := v(body); err != nil {
				return err
			}
		}
		return nil
	}
}

// TypedRequired valida que os campos informados (por json tag ou nome) não são zero value.
func TypedRequired[T any](fields ...string) TypedValidator[T] {
	return func(body T) error {
		v := reflect.ValueOf(body)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		t := v.Type()
		for _, field := range fields {
			found := false
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				tag := strings.Split(f.Tag.Get("json"), ",")[0]
				if tag == field || strings.EqualFold(f.Name, field) {
					found = true
					if v.Field(i).IsZero() {
						return fmt.Errorf("%s is required", field)
					}
					break
				}
			}
			if !found {
				return fmt.Errorf("field %s not found", field)
			}
		}
		return nil
	}
}

type Validator func() error

func Validate(validators ...Validator) error {
	for _, v := range validators {
		if err := v(); err != nil {
			return err
		}
	}
	return nil
}

func Required(key, value string) Validator {
	return func() error {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s required", key)
		}
		return nil
	}
}

func MinLength(key, value string, min int) Validator {
	return func() error {
		if len(value) < min {
			return fmt.Errorf("%s must be at least %d characters long", key, min)
		}
		return nil
	}
}

func MaxLength(key, value string, max int) Validator {
	return func() error {
		if len(value) > max {
			return fmt.Errorf("%s must be at most %d characters long", key, max)
		}
		return nil
	}
}

func ValidateEmail(key, value string) Validator {
	return func() error {
		if !strings.Contains(value, "@") || !strings.Contains(value, ".") {
			return fmt.Errorf("%s must be a valid email address", key)
		}
		return nil
	}
}
