package encoding

import (
	"bytes"
	"strings"
)

const crockfordBase32Alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ" // Crockford's Base32 alphabet

// EncodeCrockfordB32LC encodes a byte slice using Crockford's Base32 alphabet and returns
// the result in lowercase. This encoding is similar to standard Base32 but uses a modified
// alphabet that eliminates easily confused characters.
//
//nolint:gosec
func EncodeCrockfordB32LC(input []byte) string {
	var (
		result bytes.Buffer
		bits   = 0
		accum  = 0
	)

	for _, b := range input {
		accum = accum<<8 | int(b)
		bits += 8

		for bits >= 5 {
			bits -= 5
			result.WriteByte(crockfordBase32Alphabet[(accum>>(bits))&0x1F])
		}
	}

	if bits > 0 {
		result.WriteByte(crockfordBase32Alphabet[(accum<<uint(5-bits))&0x1F])
	}

	return strings.ToLower(result.String())
}

// NormalizeCrockfordB32LC normalizes a Crockford Base32 string by:
// - Removing all whitespace
// - Converting to lowercase
// - Replacing 'O' with '0'
// - Replacing 'I' and 'L' with '1'
// This helps handle common human transcription errors and variations in input.
func NormalizeCrockfordB32LC(input string) string {
	var result bytes.Buffer

	input = strings.ReplaceAll(input, " ", "")
	input = strings.ToUpper(input)

	for _, char := range input {
		switch char {
		case 'O':
			result.WriteRune('0')
		case 'I':
			fallthrough
		case 'L':
			result.WriteRune('1')
		default:
			result.WriteRune(char)
		}
	}

	return strings.ToLower(result.String())
}
