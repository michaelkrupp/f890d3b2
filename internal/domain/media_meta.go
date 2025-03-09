package domain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/mkrupp/homecase-michael/internal/util/encoding"
)

// MediaMeta contains metadata about a media file.
//
//nolint:recvcheck
type MediaMeta struct {
	Filename string  `json:"filename"` // Original filename
	ID       MediaID `json:"id"`       // Unique identifier
	Hash     string  `json:"hash"`     // Content hash (Crockford Base32)
	Size     int64   `json:"size"`     // Size in bytes
	Owner    string  `json:"owner"`    // Username of owner
	MIMEType string  `json:"mimeType"` // MIME type
}

// NewMediaMetaFromBlob creates MediaMeta from a JSON-encoded blob.
// Returns an error if the blob contains invalid JSON.
func NewMediaMetaFromBlob(blob *Blob) (MediaMeta, error) {
	var imgMeta MediaMeta
	if err := json.Unmarshal(blob.Bytes(), &imgMeta); err != nil {
		return MediaMeta{}, fmt.Errorf("unmarshal metadata: %w", err)
	}

	return imgMeta, nil
}

// update recalculates metadata fields based on the provided content.
// This includes content type detection and hash calculation.
func (imgMeta *MediaMeta) update(data []byte) {
	// Update media metadata
	hasher := sha256.New()
	hasher.Write(data)
	imgMeta.Hash = encoding.EncodeCrockfordB32LC(hasher.Sum(nil))
	imgMeta.Size = int64(len(data))

	// Update media ID
	hasher.Reset()
	hasher.Write([]byte(imgMeta.Hash))
	hasher.Write([]byte(imgMeta.Filename))
	hasher.Write([]byte(imgMeta.MIMEType))
	hasher.Write([]byte(imgMeta.Owner))
	imgMeta.ID = MediaID(encoding.EncodeCrockfordB32LC(hasher.Sum(nil)))
}

// AsBlob converts the metadata to a JSON-encoded blob using the ID as the blob ID.
// Returns an error if JSON marshaling fails.
func (imgMeta MediaMeta) AsBlob() (*Blob, error) {
	data, err := json.Marshal(imgMeta)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	return NewBlob(imgMeta.ID, data), nil
}
