package domain

import "errors"

var (
	ErrImageTypeNotSupported = errors.New("image type not supported")
	ErrImageTypeMismatch     = errors.New("image ext does not match content type")
	ErrImageTooLarge         = errors.New("image too large")
)
