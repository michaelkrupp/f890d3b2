package domain

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

var ErrMediaTooLarge = errors.New("media too large")

// Media represents a media file with its content and metadata.
type Media struct {
	data []byte
	meta MediaMeta
}

// NewMedia creates a new Media instance with the given content and metadata.
// It automatically updates the metadata based on the content.
func NewMedia(data []byte, meta MediaMeta) Media {
	var media Media

	media.data = data
	media.meta = meta

	media.meta.update(data)

	return media
}

// ID returns the media's unique identifier.
func (m Media) ID() MediaID {
	return m.meta.ID
}

// Hash returns the content hash of the media.
func (m Media) Hash() string {
	return m.meta.Hash
}

// Meta returns the media's metadata.
func (m Media) Meta() MediaMeta {
	return m.meta
}

// ContentType returns the media's MIME type.
func (m Media) MIMEType() string {
	return m.meta.MIMEType
}

// Owner returns the username of the media's owner.
func (m Media) Owner() string {
	return m.meta.Owner
}

// Bytes returns the media's content as a byte slice.
func (m Media) Bytes() []byte {
	return m.data
}

// Read returns a reader for accessing the media's content.
func (m Media) Read() io.Reader {
	return bytes.NewReader(m.data)
}

// WriteTo writes the media's content to the given writer.
// Returns the number of bytes written and any error encountered.
func (m Media) WriteTo(writer io.Writer) (int64, error) {
	bytes, err := writer.Write(m.data)
	if err != nil {
		return 0, fmt.Errorf("write: %w", err)
	}

	m.meta.update(m.data)

	return int64(bytes), err
}

// Size returns the size of the media's content in bytes.
func (m Media) Size() int64 {
	return int64(len(m.data))
}

// AsBlob converts the media to a Blob using its content hash as the ID.
func (m Media) AsBlob() *Blob {
	return NewBlob(BlobID(m.meta.Hash), m.data)
}
