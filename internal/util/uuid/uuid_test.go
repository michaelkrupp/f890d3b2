package uuid_test

import (
	"regexp"
	"testing"
	"time"

	"github.com/mkrupp/homecase-michael/internal/util/uuid"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version uuid.UUIDVersion
		wantErr error
	}{
		{
			name:    "generates valid v7 UUID",
			version: uuid.UUIDv7,
			wantErr: nil,
		},
		{
			name:    "fails for unsupported version",
			version: 999,
			wantErr: uuid.ErrUnknownVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uuid_, err := uuid.New(tt.version)

			if (err != nil) != (tt.wantErr != nil) {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				// Verify UUID format
				if !isValidUUID(t, uuid_.String()) {
					t.Errorf("New() generated invalid UUID format: %s", uuid_)
				}

				// For v7, verify timestamp is recent
				if tt.version == uuid.UUIDv7 {
					timestamp := extractV7Timestamp(uuid_)
					now := time.Now().UnixMilli()
					if now-timestamp > 1000 { // Allow 1 second difference
						t.Errorf("UUIDv7 timestamp too old: got %d, want close to %d", timestamp, now)
					}
				}
			}
		})
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	validUUID := "123e4567-e89b-7abc-9def-123456789abc"
	tests := []struct {
		name    string
		uuidStr string
		wantErr error
	}{
		{
			name:    "parses valid UUID with hyphens",
			uuidStr: validUUID,
			wantErr: nil,
		},
		{
			name:    "parses valid UUID without hyphens",
			uuidStr: "123e4567e89b7abc9def123456789abc",
			wantErr: nil,
		},
		{
			name:    "fails for invalid length",
			uuidStr: "123e4567",
			wantErr: uuid.ErrInvalidFormat,
		},
		{
			name:    "fails for invalid characters",
			uuidStr: "123e4567-e89b-7abc-9def-12345678xxxx",
			wantErr: uuid.ErrInvalidFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uuid, err := uuid.Parse(tt.uuidStr)
			if (err != nil) != (tt.wantErr != nil) {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if !isValidUUID(t, uuid.String()) {
					t.Errorf("Parse() resulted in invalid UUID format: %s", uuid)
				}
			}
		})
	}
}

func TestString(t *testing.T) {
	t.Parallel()

	// Generate a UUID and verify its string format
	uuid_, err := uuid.New(uuid.UUIDv7)
	if err != nil {
		t.Fatalf("Failed to generate UUID: %v", err)
	}

	uuidStr := uuid_.String()
	if !isValidUUID(t, uuidStr) {
		t.Errorf("String() returned invalid UUID format: %s", uuidStr)
	}

	// Parse the string back into a UUID and verify it matches
	parsed, err := uuid.Parse(uuidStr)
	if err != nil {
		t.Errorf("Failed to parse generated UUID string: %v", err)
	}

	if parsed.String() != uuidStr {
		t.Errorf("String() roundtrip failed: got %s, want %s", parsed.String(), uuidStr)
	}
}

func TestConcurrentGeneration(t *testing.T) {
	t.Parallel()

	const numGoroutines = 100
	uuids := make(chan uuid.UUID, numGoroutines)
	seen := make(map[string]bool)

	// Generate UUIDs concurrently
	for range numGoroutines {
		go func() {
			uuid_, err := uuid.New(uuid.UUIDv7)
			if err != nil {
				t.Errorf("Failed to generate UUID: %v", err)
				uuids <- uuid.UUID{} // Send empty UUID on error
				return
			}
			uuids <- uuid_
		}()
	}

	// Collect and verify UUIDs
	for range numGoroutines {
		uuid := <-uuids
		uuidStr := uuid.String()

		if !isValidUUID(t, uuidStr) {
			t.Errorf("Generated invalid UUID format: %s", uuidStr)
		}

		if seen[uuidStr] {
			t.Errorf("Duplicate UUID generated: %s", uuidStr)
		}
		seen[uuidStr] = true
	}
}

// Helper functions

func isValidUUID(t *testing.T, uuidStr string) bool {
	t.Helper()
	pattern := `^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`
	matched, err := regexp.MatchString(pattern, uuidStr)
	if err != nil {
		t.Fatalf("Failed to match UUID pattern: %v", err)
	}
	return matched
}

func extractV7Timestamp(uuid_ uuid.UUID) int64 {
	// Extract 48-bit timestamp from UUIDv7
	return int64(uuid_.Bytes()[0])<<40 |
		int64(uuid_.Bytes()[1])<<32 |
		int64(uuid_.Bytes()[2])<<24 |
		int64(uuid_.Bytes()[3])<<16 |
		int64(uuid_.Bytes()[4])<<8 |
		int64(uuid_.Bytes()[5])
}
