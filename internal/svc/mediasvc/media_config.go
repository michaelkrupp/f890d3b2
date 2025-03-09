package mediasvc

// MediaConfig holds configuration parameters for the media service.
type MediaConfig struct {
	// MaxSize is the maximum allowed file size for uploaded images in bytes.
	// Default is 20MB.
	MaxSize int64 `env:"MAX_SIZE" default:"20971520"`
}
