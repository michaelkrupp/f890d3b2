package domain

import "errors"

// ErrNoMediaID is returned when a media ID is required but not provided.
var ErrNoMediaID = errors.New("no media ID")

// MediaID is an alias for BlobID used to identify media objects.
// This allows for type-safe handling of media identifiers while
// maintaining compatibility with the blob storage system.
type MediaID = BlobID
