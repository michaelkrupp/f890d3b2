package encoding_test

import (
	"testing"

	"github.com/mkrupp/homecase-michael/internal/util/encoding"
)

func TestEncodeCrockfordB32LC(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{
			name:  "empty input",
			input: []byte{},
			want:  "",
		},
		{
			name:  "single byte",
			input: []byte{0xF5},
			want:  "ym",
		},
		{
			name:  "two bytes",
			input: []byte{0xF5, 0x3A},
			want:  "ymx0",
		},
		{
			name:  "three bytes",
			input: []byte{0xF5, 0x3A, 0x58},
			want:  "ymx5g",
		},
		{
			name:  "four bytes",
			input: []byte{0xF5, 0x3A, 0x58, 0x9B},
			want:  "ymx5h6r",
		},
		{
			name:  "five bytes with padding",
			input: []byte{0xF5, 0x3A, 0x58, 0x9B, 0xC4},
			want:  "ymx5h6y4",
		},
		{
			name:  "all zero bytes",
			input: []byte{0, 0, 0, 0},
			want:  "0000000",
		},
		{
			name:  "all ones",
			input: []byte{255, 255, 255, 255},
			want:  "zzzzzzr",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := encoding.EncodeCrockfordB32LC(tt.input)
			if got != tt.want {
				t.Errorf("EncodeCrockfordB32LC() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeCrockfordB32LC(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "already lowercase",
			input: "abc123def",
			want:  "abc123def",
		},
		{
			name:  "uppercase",
			input: "ABC123DEF",
			want:  "abc123def",
		},
		{
			name:  "mixed case",
			input: "aBc123DeF",
			want:  "abc123def",
		},
		{
			name:  "with whitespace",
			input: "  ABC 123 DEF  ",
			want:  "abc123def",
		},
		{
			name:  "O to 0",
			input: "ABCO123ODEF",
			want:  "abc01230def",
		},
		{
			name:  "I and L to 1",
			input: "ABCI123LDEF",
			want:  "abc11231def",
		},
		{
			name:  "all substitutions",
			input: "OIL OIL",
			want:  "011011",
		},
		{
			name:  "real world example",
			input: "Z7IO-L5KM-NXQP",
			want:  "z710-15km-nxqp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := encoding.NormalizeCrockfordB32LC(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeCrockfordB32LC() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEncodeThenNormalize(t *testing.T) {
	t.Parallel()

	// Test that normalizing an encoded value doesn't change it
	input := []byte{0xF5, 0x3A, 0x58, 0x9B, 0xC4}
	encoded := encoding.EncodeCrockfordB32LC(input)
	normalized := encoding.NormalizeCrockfordB32LC(encoded)

	if encoded != normalized {
		t.Errorf("Normalization changed encoded value: got %q, want %q", normalized, encoded)
	}
}
