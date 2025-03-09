package imagesvc

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"
	"strings"

	"golang.org/x/image/draw"
)

var (
	// ErrUnknownInterpolator is returned when an unsupported interpolation method is specified.
	ErrUnknownInterpolator = errors.New("unknown interpolator")

	// ErrUnsupportedMIMEType is returned when trying to process an unsupported image format.
	ErrUnsupportedMIMEType = errors.New("unsupported MIME type")
)

//nolint:gochecknoglobals
var (
	// interpolMap maps interpolator names to their implementations.
	// Supported values: "nearestneighbor", "catmullrom", "bilinear", "approxbilinear".
	interpolMap = map[string]draw.Interpolator{
		"nearestneighbor": draw.NearestNeighbor,
		"catmullrom":      draw.CatmullRom,
		"bilinear":        draw.BiLinear,
		"approxbilinear":  draw.ApproxBiLinear,
	}
)

func getInterpolatorByName(name string) (draw.Interpolator, error) {
	interpol, ok := interpolMap[strings.ToLower(name)]
	if !ok {
		return nil, ErrUnknownInterpolator
	}

	return interpol, nil
}

// resizeImage resizes an image to the specified width while maintaining aspect ratio.
// It supports JPEG, PNG and TIFF formats.
// The interpolator parameter specifies the scaling algorithm to use.
// Returns ErrUnknownInterpolator if the interpolator is not supported.
// Returns ErrUnsupportedContentType if the image format is not supported.
func resizeImage(data []byte, ctype string, width int, interpolator string) (resized []byte, err error) {
	// Decode image
	original, err := decodeImage(bytes.NewReader(data), ctype)
	if err != nil {
		return []byte{}, fmt.Errorf("decode image: %w", err)
	}

	// Resize image
	ratio := float64(width) / float64(original.Bounds().Dx())
	height := int(float64(original.Bounds().Dy()) * ratio)

	bitmap := image.NewRGBA(image.Rect(0, 0, width, height))

	interpol, err := getInterpolatorByName(interpolator)
	if err != nil {
		return []byte{}, fmt.Errorf("get interpolator: %w", err)
	}

	interpol.Scale(bitmap, bitmap.Bounds(), original, original.Bounds(), draw.Over, nil)

	// Encode image
	resized, err = encodeImage(bitmap, ctype)
	if err != nil {
		return []byte{}, fmt.Errorf("encode image: %w", err)
	}

	return resized, nil
}

// decodeImage decodes a binary image into a Go image.Image object.
// Returns ErrUnsupportedContentType if the content type is not supported.
func decodeImage(reader io.Reader, ctype string) (image image.Image, err error) {
	decoder, err := getDecoderByType(ctype)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedMIMEType, ctype)
	}

	return decoder(reader)
}

// encodeImage encodes a Go image.Image object into binary format.
// Returns ErrUnsupportedContentType if the content type is not supported.
func encodeImage(bitmap image.Image, ctype string) ([]byte, error) {
	var (
		buffer []byte
		writer = bytes.NewBuffer(buffer)
	)

	encoder, err := getEncoderByType(ctype)
	if err != nil {
		return nil, fmt.Errorf("get encoder: %w", err)
	}

	err = encoder(writer, bitmap)

	return writer.Bytes(), err
}
