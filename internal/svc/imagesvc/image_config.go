package imagesvc

// ImageConfig holds configuration parameters for the image service.
type ImageConfig struct {
	// Interpolator specifies the image scaling algorithm to use.
	// Valid values are: "nearestneighbor", "catmullrom", "bilinear", "approxbilinear"
	Interpolator string `env:"INTERPOLATOR" default:"catmullrom"`
}
