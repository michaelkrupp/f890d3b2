package imagesvc

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/mkrupp/homecase-michael/internal/domain"
	"golang.org/x/image/tiff"
)

const (
	MIMETypeJPEG = "image/jpeg"
	MIMETypePNG  = "image/png"
	MIMETypeTIFF = "image/tiff"
)

//nolint:gochecknoglobals
var (
	imageExtTypes = map[string]string{
		".jpg":  MIMETypeJPEG,
		".jpeg": MIMETypeJPEG,
		".png":  MIMETypePNG,
		".tiff": MIMETypeTIFF,
		".tif":  MIMETypeTIFF,
	}

	imageExtHeaders = map[string][]string{
		MIMETypeJPEG: {"\xFF\xD8"},
		MIMETypePNG:  {"\x89\x50\x4E\x47\x0D\x0A\x1A\x0A"},
		MIMETypeTIFF: {"\x49\x49\x2A\x00", "\x4D\x4D\x00\x2A"},
	}

	imageDecoders = map[string]func(io.Reader) (image.Image, error){
		MIMETypeJPEG: jpeg.Decode,
		MIMETypeTIFF: tiff.Decode,
		MIMETypePNG:  png.Decode,
	}

	imageEncoders = map[string]func(io.Writer, image.Image) error{
		MIMETypeJPEG: func(w io.Writer, i image.Image) error { return jpeg.Encode(w, i, nil) },
		MIMETypeTIFF: func(w io.Writer, i image.Image) error { return tiff.Encode(w, i, nil) },
		MIMETypePNG:  png.Encode,
	}
)

func getDecoderByType(mimeType string) (func(io.Reader) (image.Image, error), error) {
	decoder, ok := imageDecoders[mimeType]
	if !ok {
		return nil, fmt.Errorf("%w: %q", domain.ErrImageTypeNotSupported, mimeType)
	}

	return decoder, nil
}

func getEncoderByType(mimeType string) (func(io.Writer, image.Image) error, error) {
	encoder, ok := imageEncoders[mimeType]
	if !ok {
		return nil, fmt.Errorf("%w: %q", domain.ErrImageTypeNotSupported, mimeType)
	}

	return encoder, nil
}
