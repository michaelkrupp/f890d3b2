package domain

// BlobID is a string-based identifier for blob objects.
// It is typically a Crockford Base32 encoded hash of the blob's content.
type BlobID string

// String returns the string representation of the BlobID.
func (id BlobID) String() string {
	return string(id)
}
