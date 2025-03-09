package uuid

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrUnknownVersion = errors.New("unknown UUID version")
	ErrInvalidFormat  = errors.New("invalid UUID format")
)

const (
	UUIDSize = 16
)

// UUID represents a 128-bit universally unique identifier as specified in RFC 4122.
type UUID struct {
	bytes [UUIDSize]byte
}

// UUIDVersion represents the version number of a UUID.
// Different versions have different algorithms for generating the UUID.
type UUIDVersion int

const (
	// UUIDv7 represents UUID version 7, which is a time-ordered UUID that
	// includes a timestamp and random data.
	UUIDv7 UUIDVersion = 7
)

// New generates a new UUID of the specified version.
// Returns an error if an unsupported version is specified.
func New(version UUIDVersion) (UUID, error) {
	var uuid UUID

	switch version {
	case UUIDv7:
		generateUUIDv7(&uuid)
	default:
		return UUID{}, fmt.Errorf("%w: %d", ErrUnknownVersion, version)
	}

	return uuid, nil
}

// Parse decodes a UUID from its string representation.
// The string can contain hyphens which will be removed before parsing.
// Returns an error if the string is not a valid UUID format (32 hex chars with optional hyphens).
func Parse(uuidStr string) (UUID, error) {
	var uuid UUID

	// Remove dashes
	uuidStr = strings.ReplaceAll(uuidStr, "-", "")
	if len(uuidStr) != UUIDSize*2 {
		return UUID{}, ErrInvalidFormat
	}

	// Decode hex string into bytes
	bytes, err := hex.DecodeString(uuidStr)
	if err != nil {
		return UUID{}, fmt.Errorf("failed to parse UUID: %w", err)
	}

	copy(uuid.bytes[:], bytes)

	return uuid, nil
}

func generateUUIDv7(uuid *UUID) {
	// Get current Unix timestamp in milliseconds (48 bits)
	now := time.Now().UnixMilli()

	// Fill the first 6 bytes with the timestamp (big-endian)
	uuid.bytes[0] = byte(now >> 40)
	uuid.bytes[1] = byte(now >> 32)
	uuid.bytes[2] = byte(now >> 24)
	uuid.bytes[3] = byte(now >> 16)
	uuid.bytes[4] = byte(now >> 8)
	uuid.bytes[5] = byte(now)

	// Generate 10 bytes of cryptographic randomness
	_, err := rand.Read(uuid.bytes[6:])
	if err != nil {
		panic("failed to generate UUIDv7: " + err.Error())
	}

	// Set the version (7) in the correct position
	uuid.bytes[6] = (uuid.bytes[6] & 0x0F) | 0x70 // Set high nibble to '7'

	// Set the variant (RFC 4122) - bits 6 and 7 of byte 8 should be "10"
	uuid.bytes[8] = (uuid.bytes[8] & 0x3F) | 0x80 // Set high bits to '10'
}

// String returns the canonical string representation of the UUID,
// formatted according to RFC 4122 with hyphens (8-4-4-4-12 format).
func (u UUID) String() string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(u.bytes[0:4]),
		binary.BigEndian.Uint16(u.bytes[4:6]),
		binary.BigEndian.Uint16(u.bytes[6:8]),
		binary.BigEndian.Uint16(u.bytes[8:10]),
		u.bytes[10:16])
}

// Bytes returns the raw bytes of the UUID.
func (u UUID) Bytes() []byte {
	return u.bytes[:]
}
