package config_test

import (
	"context"
	"errors"
	"os"
	"testing"

	. "github.com/mkrupp/homecase-michael/internal/infra/config"
)

type testConfig struct {
	EnvConfig

	StringValue string `env:"STRING_VALUE" default:"default"`
	IntValue    int    `env:"INT_VALUE" default:"42"`
	BoolValue   bool   `env:"BOOL_VALUE" default:"true"`
	NoEnvTag    string
	Nested      testNestedConfig
}

type testNestedConfig struct {
	NestedString string `env:"NESTED_STRING" default:"nested-default"`
}

func setupEnv(t *testing.T, envVars map[string]string) {
	t.Helper()

	// Store original env values
	original := make(map[string]string)

	for k := range envVars {
		if v, exists := os.LookupEnv(k); exists {
			original[k] = v
		}
	}

	// Set test values
	for k, v := range envVars {
		t.Setenv(k, v)
	}
}

//nolint:paralleltest
func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		prefix  string
		envVars map[string]string
		want    testConfig
		wantErr bool
	}{
		{
			name:    "uses default values when env vars not set",
			prefix:  "",
			envVars: map[string]string{},
			want: testConfig{
				StringValue: "default",
				IntValue:    42,
				BoolValue:   true,
				Nested: testNestedConfig{
					NestedString: "nested-default",
				},
			},
		},
		{
			name:   "reads environment variables",
			prefix: "",
			envVars: map[string]string{
				"STRING_VALUE":  "env-value",
				"INT_VALUE":     "123",
				"BOOL_VALUE":    "false",
				"NESTED_STRING": "env-nested",
			},
			want: testConfig{
				StringValue: "env-value",
				IntValue:    123,
				BoolValue:   false,
				Nested: testNestedConfig{
					NestedString: "env-nested",
				},
			},
		},
		{
			name:   "handles prefix correctly",
			prefix: "APP",
			envVars: map[string]string{
				"APP_STRING_VALUE": "prefixed-value",
			},
			want: testConfig{
				StringValue: "prefixed-value",
				IntValue:    42,
				BoolValue:   true,
				Nested: testNestedConfig{
					NestedString: "nested-default",
				},
			},
		},
		{
			name:   "fails on invalid int value",
			prefix: "",
			envVars: map[string]string{
				"INT_VALUE": "not-a-number",
			},
			wantErr: true,
		},
		{
			name:   "fails on invalid bool value",
			prefix: "",
			envVars: map[string]string{
				"BOOL_VALUE": "not-a-bool",
			},
			wantErr: true,
		},
		{
			name:    "ignores fields without env tag",
			prefix:  "",
			envVars: map[string]string{},
			want: testConfig{
				StringValue: "default",
				IntValue:    42,
				BoolValue:   true,
				NoEnvTag:    "",
				Nested: testNestedConfig{
					NestedString: "nested-default",
				},
			},
		},
		{
			name:   "handles multi-level prefixes",
			prefix: "APP_SERVICE",
			envVars: map[string]string{
				"APP_SERVICE_STRING_VALUE": "multi-level-prefix",
			},
			want: testConfig{
				StringValue: "multi-level-prefix",
				IntValue:    42,
				BoolValue:   true,
				Nested: testNestedConfig{
					NestedString: "nested-default",
				},
			},
		},
		{
			name:   "prefers more specific prefix",
			prefix: "APP_SERVICE",
			envVars: map[string]string{
				"APP_STRING_VALUE":         "less-specific",
				"APP_SERVICE_STRING_VALUE": "more-specific",
			},
			want: testConfig{
				StringValue: "more-specific",
				IntValue:    42,
				BoolValue:   true,
				Nested: testNestedConfig{
					NestedString: "nested-default",
				},
			},
		},
		{
			name:   "handles empty string values",
			prefix: "",
			envVars: map[string]string{
				"STRING_VALUE": "",
			},
			want: testConfig{
				StringValue: "",
				IntValue:    42,
				BoolValue:   true,
				Nested: testNestedConfig{
					NestedString: "nested-default",
				},
			},
		},
		{
			name:   "handles zero int values",
			prefix: "",
			envVars: map[string]string{
				"INT_VALUE": "0",
			},
			want: testConfig{
				StringValue: "default",
				IntValue:    0,
				BoolValue:   true,
				Nested: testNestedConfig{
					NestedString: "nested-default",
				},
			},
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupEnv(t, tt.envVars)

			cfg := &testConfig{}
			err := Parse(ctx, cfg, tt.prefix)

			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if cfg.StringValue != tt.want.StringValue {
					t.Errorf("StringValue = %v, want %v", cfg.StringValue, tt.want.StringValue)
				}
				if cfg.IntValue != tt.want.IntValue {
					t.Errorf("IntValue = %v, want %v", cfg.IntValue, tt.want.IntValue)
				}
				if cfg.BoolValue != tt.want.BoolValue {
					t.Errorf("BoolValue = %v, want %v", cfg.BoolValue, tt.want.BoolValue)
				}
				if cfg.NoEnvTag != tt.want.NoEnvTag {
					t.Errorf("NoEnvTag = %v, want %v", cfg.NoEnvTag, tt.want.NoEnvTag)
				}
				if cfg.Nested.NestedString != tt.want.Nested.NestedString {
					t.Errorf("NestedString = %v, want %v", cfg.Nested.NestedString, tt.want.Nested.NestedString)
				}
			}
		})
	}
}

func TestParseInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     interface{}
		wantErr error
	}{
		{
			name:    "non-pointer config",
			cfg:     testConfig{},
			wantErr: ErrInvalidConfig,
		},
		{
			name:    "non-struct pointer",
			cfg:     new(string),
			wantErr: ErrInvalidConfig,
		},
		{
			name: "missing EnvConfig embedding",
			cfg: &struct {
				Value string `env:"VALUE"`
			}{},
			wantErr: ErrInvalidConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Parse(context.Background(), tt.cfg, "")
			if err == nil {
				t.Error("expected error, got nil")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}
