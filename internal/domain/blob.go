package domain

import (
	"bytes"
	"fmt"
	"io"
)

// Blob represents a binary large object with an identifier and content.
type Blob struct {
	ID   BlobID
	Body []byte
}

// NewBlob creates a new Blob with the given ID and content.
func NewBlob(id BlobID, body []byte) *Blob {
	return &Blob{
		ID:   id,
		Body: body,
	}
}

// Size returns the size of the blob's content in bytes.
func (blob *Blob) Size() int64 {
	return int64(len(blob.Body))
}

// Read returns a reader for accessing the blob's content.
func (blob *Blob) Read() io.Reader {
	return bytes.NewReader(blob.Body)
}

// Bytes returns the blob's content as a byte slice.
func (blob *Blob) Bytes() []byte {
	return blob.Body
}

// WriteTo writes the blob's content to the given writer.
// Returns the number of bytes written and any error encountered.
func (blob *Blob) WriteTo(writer io.Writer) (int64, error) {
	n, err := writer.Write(blob.Body)
	if err != nil {
		return 0, fmt.Errorf("write: %w", err)
	}

	return int64(n), nil
}

// ReadFrom reads the blob's content from the given reader.
// Returns the number of bytes read and any error encountered.
func (blob *Blob) ReadFrom(reader io.Reader) (int64, error) {
	body, err := io.ReadAll(reader)
	if err != nil {
		return 0, fmt.Errorf("read all: %w", err)
	}

	blob.Body = body

	return int64(len(body)), nil
}
