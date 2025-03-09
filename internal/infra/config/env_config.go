package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

var (
	// ErrInvalidConfig is returned when the provided config is not a pointer to a struct
	// that embeds EnvConfig.
	ErrInvalidConfig = errors.New("config must be a pointer to a struct embedding EnvConfig")

	// ErrVarNotSet is returned when a required environment variable is not set and has no default.
	ErrVarNotSet = errors.New("env var not set")

	// ErrUnsupportedVarType is returned when trying to parse an environment variable
	// into an unsupported Go type.
	ErrUnsupportedVarType = errors.New("unsupported env var type")
)

// EnvConfig is a base type that must be embedded in configuration structs
// to enable environment variable parsing.
type EnvConfig struct {
	namespace string
}

//nolint:varnamelen
func getEnvConfig(cfg any) (*EnvConfig, error) {
	v := reflect.ValueOf(cfg)

	// Ensure cfg is a pointer to a struct
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return nil, ErrInvalidConfig
	}

	v = v.Elem() // Dereference the pointer to get the struct value
	t := v.Type()

	// Iterate over fields to find the embedded EnvConfig
	for i := range t.NumField() {
		field := t.Field(i)
		//nolint:exhaustruct,forcetypeassert
		if field.Anonymous && field.Type == reflect.TypeOf(EnvConfig{}) {
			if ev := v.Field(i); ev.CanAddr() {
				return ev.Addr().Interface().(*EnvConfig), nil
			}
		}
	}

	return nil, ErrInvalidConfig
}

// Parse loads configuration values from environment variables into the provided struct.
// The struct must embed EnvConfig and use `env` tags to specify variable names.
// The namespace parameter is used as a prefix for all environment variables.
// Supports string, int, and bool fields. Nested structs are supported.
// Returns an error if parsing fails or required variables are missing.
func Parse(ctx context.Context, cfg any, namespace string) error {
	envConfig, err := getEnvConfig(cfg)
	if err != nil {
		return fmt.Errorf("get env config: %w", err)
	}

	envConfig.namespace = namespace

	return parse(namespace, "", cfg)
}

func parse(namespace, prefix string, c interface{}) error {
	t := reflect.TypeOf(c).Elem()
	v := reflect.ValueOf(c).Elem()

	for i := range t.NumField() {
		field := t.Field(i)
		structField := v.Field(i)

		fieldType := field.Type.Kind()

		if fieldType == reflect.Struct {
			envPrefix := field.Tag.Get("envPrefix")

			if err := parse(namespace, prefix+envPrefix, structField.Addr().Interface()); err != nil {
				return err
			}

			continue
		}

		if err := parseField(namespace, prefix, field, structField); err != nil {
			return fmt.Errorf("parse field: %w", err)
		}
	}

	return nil
}

//nolint:funlen,cyclop
func parseField(
	namespace string,
	prefix string,
	field reflect.StructField,
	structField reflect.Value,
) error {
	fieldType := field.Type.Kind()

	envTag := field.Tag.Get("env")
	defaultValue, hasDefault := field.Tag.Lookup("default")

	if envTag == "" {
		return nil // Skip field if no env tag is set
	}

	// Iterate over possible namespaces
	var (
		nsParts   = strings.Split(namespace, "_")
		envName   string
		envValue  string
		envExists bool
	)

	for i := len(nsParts); i > 0; i-- {
		envName = strings.Join(nsParts[:i], "_")

		if envName != "" {
			envName += "_"
		}

		envName = envName + prefix + envTag
		envValue, envExists = os.LookupEnv(envName)

		if envExists {
			break // Use the found value
		}
	}

	if !envExists {
		if !hasDefault {
			return fmt.Errorf("%w: %s", ErrVarNotSet, envTag)
		}

		envValue = defaultValue // Apply default value
	}

	//nolint:exhaustive
	switch fieldType {
	case reflect.String:
		structField.SetString(envValue)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intValue, err := strconv.Atoi(envValue)
		if err != nil {
			return fmt.Errorf("invalid type for %s: %w", envTag, err)
		}

		structField.SetInt(int64(intValue))
	case reflect.Bool:
		boolValue, err := strconv.ParseBool(envValue)
		if err != nil {
			return fmt.Errorf("invalid type for %s: %w", envTag, err)
		}

		structField.SetBool(boolValue)
	default:
		return fmt.Errorf("%w: %s (%v)", ErrUnsupportedVarType, envTag, fieldType)
	}

	return nil
}
