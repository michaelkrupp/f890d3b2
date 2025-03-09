package imagesvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"

	"github.com/mkrupp/homecase-michael/internal/domain"
	context_ "github.com/mkrupp/homecase-michael/internal/infra/context"
	"github.com/mkrupp/homecase-michael/internal/infra/logging"
	http_ "github.com/mkrupp/homecase-michael/internal/infra/transport/http"
	"github.com/mkrupp/homecase-michael/internal/svc/authsvc/authclient"
	"github.com/mkrupp/homecase-michael/internal/util/encoding"
)

var ErrPanic = errors.New("panic")

// HTTPTransportConfig contains configuration parameters for the HTTP transport layer.
type HTTPTransportConfig struct {
	http_.HTTPTransportConfig

	// MultipartFileName is the form field name for file uploads.
	// Default is "upload".
	MultipartFileName string `env:"MULTIPART_FILE_NAME" default:"upload"`

	// URLFileIDParam is the URL parameter name for image IDs.
	// Default is "media_id".
	URLFileIDParam string `env:"URL_FILE_ID_PARAM" default:"media_id"`

	// URLFileDownloadParam is the URL parameter for triggering downloads.
	// Default is "download".
	URLFileDownloadParam string `env:"URL_FILE_DOWNLOAD_PARAM" default:"download"`

	// URLWidthParam is the URL parameter for specifying image resize width.
	// Default is "width".
	URLWidthParam string `env:"URL_WIDTH_PARAM" default:"width"`

	// ContentDispositionDownload controls whether files are served with download headers.
	// Default is false.
	ContentDispositionDownload bool `env:"CONTENT_DISPOSITION_DOWNLOAD" default:"false"`

	// MultipartFormMaxMemory is the maximum allowed memory for multipart form uploads.
	// Default is 10MB.
	MultipartFormMaxMemory int64 `env:"MULTIPART_FORM_MAX_SIZE" default:"10485760"`
}

var ErrNoMultipartFiles = errors.New("no multipart files")

// HTTPTransport handles HTTP requests for the image service.
// It provides endpoints for uploading, downloading and deleting images.
type HTTPTransport struct {
	imageSvc   ImageService
	authClient authclient.AuthClient
	log        logging.Logger
	cfg        HTTPTransportConfig
}

var _ http_.HTTPTransport = (*HTTPTransport)(nil)

// NewHTTPTransport creates a new HTTPTransport instance with the given configuration.
// It requires an ImageService for handling business logic and an AuthClient for authentication.
func NewHTTPTransport(
	imageSvc ImageService,
	authClient authclient.AuthClient,
	cfg HTTPTransportConfig,
) *HTTPTransport {
	return &HTTPTransport{
		imageSvc:   imageSvc,
		authClient: authClient,
		log:        logging.GetLogger("svc.imagesvc.http_transport"),
		cfg:        cfg,
	}
}

// ServeHTTP implements http.Handler and sets up routes for the image service endpoints:
// - POST /media: Upload image
// - DELETE /media/{image-id}: Delete image by ID
// - GET /media/{image-id}: Download image by ID
// Routes are protected by authentication middleware.
func (ht *HTTPTransport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /media", ht.HandleUpload)
	mux.HandleFunc(fmt.Sprintf("DELETE /media/{%s}", ht.cfg.URLFileIDParam), ht.HandleDelete)
	mux.HandleFunc(fmt.Sprintf("GET /media/{%s}", ht.cfg.URLFileIDParam), ht.HandleDownload)

	handler := http.Handler(mux)
	handler = http_.AuthorizingMiddleware(handler, ht.authClient, ht.log)

	handler.ServeHTTP(w, r)
}

// HandleUpload processes image upload requests.
// Expects a multipart form with a file field matching MultipartFileName config.
func (ht *HTTPTransport) HandleUpload(w http.ResponseWriter, r *http.Request) {
	_ = ht.handleUpload(w, r)
}

//nolint:funlen
func (ht *HTTPTransport) handleUpload(w http.ResponseWriter, r *http.Request) (err error) {
	log := ht.log.With(logging.Group("http", "method", r.Method, "url", r.URL.String()))

	ctx, cancel := context.WithCancel(r.Context()) // Create cancellable context
	defer cancel()

	defer func() {
		if err != nil {
			log.ErrorContext(ctx, "media upload failed", "error", err)
		} else {
			log.DebugContext(ctx, "media uploaded")
		}
	}()

	mediaCh, errCh := ht.processMultipartForm(ctx, r)

	var (
		mediaResp    []domain.MediaIDResponse
		errGroup     sync.WaitGroup
		errMutex     sync.Mutex
		uploadErrors []error
	)

	// Goroutine to listen for errors and cancel on first error
	errGroup.Add(1)

	go func() {
		defer errGroup.Done()

		for err := range errCh {
			log.ErrorContext(ctx, "media upload error", "error", err)

			errMutex.Lock()
			uploadErrors = append(uploadErrors, err)
			errMutex.Unlock()

			cancel() // Stop media processing
		}
	}()

	// Goroutine to process media files
	errGroup.Add(1)

	go func() {
		defer errGroup.Done()

		for media := range mediaCh {
			log.DebugContext(ctx, "media uploaded", logging.Group("media",
				"id", media.ID().String(),
				"filename", media.Meta().Filename,
				"size", media.Size(),
				"owner", media.Owner(),
			))

			mediaResp = append(mediaResp, domain.MediaIDResponse{
				ID:       media.ID().String(),
				Filename: media.Meta().Filename,
			})
		}
	}()

	// Wait for both goroutines to finish
	errGroup.Wait()

	// If errors occurred, return HTTP 400
	if len(uploadErrors) > 0 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return fmt.Errorf("process multipart form: %w", errors.Join(uploadErrors...))
	}

	// Send response with media IDs
	sort.Slice(mediaResp, func(i, j int) bool {
		return mediaResp[i].ID < mediaResp[j].ID
	})

	if err := json.NewEncoder(w).Encode(mediaResp); err != nil {
		return fmt.Errorf("encode response: %w", err)
	}

	return nil
}

func (ht *HTTPTransport) processMultipartForm(
	ctx context.Context,
	r *http.Request,
) (<-chan domain.Media, <-chan error) {
	if err := r.ParseMultipartForm(ht.cfg.MultipartFormMaxMemory); err != nil {
		errCh := make(chan error, 1)
		errCh <- fmt.Errorf("parse multipart form: %w", err)
		close(errCh)

		return nil, errCh
	}

	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		errCh := make(chan error, 1)
		errCh <- ErrNoMultipartFiles
		close(errCh)

		return nil, errCh
	}

	return ht.processMultipartFormFiles(ctx, r.MultipartForm.File)
}

func (ht *HTTPTransport) processMultipartFormFiles(
	ctx context.Context,
	fileHeaders map[string][]*multipart.FileHeader,
) (<-chan domain.Media, <-chan error) {
	mediaCh := make(chan domain.Media)
	errCh := make(chan error, len(fileHeaders)) // Buffered to avoid blocking

	var (
		wg   sync.WaitGroup
		once sync.Once // Ensures `errCh` is closed only once on early exit
	)

	// Goroutine to manage file processing
	go func() {
		defer close(mediaCh)
		defer close(errCh)

		defer func() {
			if r := recover(); r != nil {
				errCh <- fmt.Errorf("%w: %v", ErrPanic, r)
			}
		}()

		for _, f := range fileHeaders {
			for _, fileHeader := range f {
				// Check if context is already cancelled before processing
				select {
				case <-ctx.Done():
					once.Do(func() { errCh <- ctx.Err() })

					return // Stop processing immediately
				default:
				}

				wg.Add(1)

				go processFile(ctx, ht.imageSvc, fileHeader, mediaCh, errCh, &wg)
			}
		}

		wg.Wait() // Wait for all file processing to complete
	}()

	return mediaCh, errCh
}

//nolint:funlen
func processFile(
	ctx context.Context,
	imageSvc ImageService,
	fileHeader *multipart.FileHeader,
	mediaCh chan<- domain.Media,
	errCh chan<- error,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	defer func() {
		if r := recover(); r != nil {
			errCh <- fmt.Errorf("%w: %v", ErrPanic, r)
		}
	}()

	if ctx.Err() != nil {
		return
	}

	// Check upload constraints before reading the image to buffer
	_, _, err := imageSvc.CheckUploadConstraints(
		fileHeader.Filename,
		fileHeader.Size,
		nil,
	)
	if err != nil {
		errCh <- fmt.Errorf("upload not allowed: %s: %w", fileHeader.Filename, err)
	}

	// Read file content
	file, err := fileHeader.Open()
	if err != nil {
		errCh <- fmt.Errorf("open %s: %w", fileHeader.Filename, err)

		return
	}
	defer file.Close()

	buffer, err := io.ReadAll(file)
	if err != nil {
		errCh <- fmt.Errorf("read %s: %w", fileHeader.Filename, err)

		return
	}

	// Re-Check upload constraints now that the image has been read
	mimeType, _, err := imageSvc.CheckUploadConstraints(
		fileHeader.Filename,
		fileHeader.Size,
		buffer,
	)
	if err != nil {
		errCh <- fmt.Errorf("upload not allowed: %s: %w", fileHeader.Filename, err)
	}

	// Store file via image service
	owner, _ := context_.UsernameFromContext(ctx)
	media := domain.NewMedia(buffer, domain.MediaMeta{ //nolint:exhaustruct
		Filename: fileHeader.Filename,
		Owner:    owner,
		MIMEType: mimeType,
	})

	if err := imageSvc.Store(ctx, media); err != nil {
		errCh <- fmt.Errorf("store %s: %w", fileHeader.Filename, err)

		return
	}

	select {
	case mediaCh <- media:
	case <-ctx.Done():
	}
}

// HandleDelete processes image deletion requests.
// Expects the image ID as a URL parameter matching URLFileIDParam config.
func (ht *HTTPTransport) HandleDelete(w http.ResponseWriter, r *http.Request) {
	_ = ht.handleDelete(w, r)
}

func (ht *HTTPTransport) handleDelete(w http.ResponseWriter, r *http.Request) (err error) {
	log := ht.log.With(logging.Group("http", "method", r.Method, "url", r.URL.String()))

	defer func(ctx context.Context) {
		if err != nil {
			log.ErrorContext(ctx, "media delete failed", "error", err)
		} else {
			log.DebugContext(ctx, "media deleted")
		}
	}(r.Context())

	mediaID := r.PathValue(ht.cfg.URLFileIDParam)
	if mediaID == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return domain.ErrNoMediaID
	}

	mediaID = encoding.NormalizeCrockfordB32LC(mediaID)
	log = log.With(logging.Group("media", "id", mediaID))

	if err := ht.imageSvc.Delete(r.Context(), domain.MediaID(mediaID)); err != nil {
		switch {
		case errors.Is(err, domain.ErrUnauthorized):
			fallthrough
		case errors.Is(err, os.ErrNotExist):
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		default:
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		return fmt.Errorf("delete: %w", err)
	}

	return nil
}

// HandleDownload processes image download requests.
// Expects the image ID as a URL parameter and an optional width parameter for resizing.
func (ht *HTTPTransport) HandleDownload(w http.ResponseWriter, r *http.Request) {
	_ = ht.handleDownload(w, r)
}

//nolint:funlen,cyclop
func (ht *HTTPTransport) handleDownload(w http.ResponseWriter, r *http.Request) (err error) {
	log := ht.log.With(logging.Group("http", "method", r.Method, "url", r.URL.String()))

	defer func(ctx context.Context) {
		if err != nil {
			log.ErrorContext(ctx, "media download failed", "error", err)
		} else {
			log.DebugContext(ctx, "media downloaded")
		}
	}(r.Context())

	fileID := r.PathValue(ht.cfg.URLFileIDParam)
	if fileID == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

		return domain.ErrNoMediaID
	}

	fileID = encoding.NormalizeCrockfordB32LC(fileID)
	log = log.With(logging.Group("media", "id", fileID))

	var width int

	if widthStr := r.URL.Query().Get(ht.cfg.URLWidthParam); widthStr != "" {
		width_, err := strconv.ParseInt(widthStr, 10, 64)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

			return fmt.Errorf("parse width: %w", err)
		}

		width = int(width_)
	}

	media, err := ht.imageSvc.Fetch(r.Context(), domain.MediaID(fileID), width)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUnauthorized):
			fallthrough
		case errors.Is(err, os.ErrNotExist):
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		default:
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		return fmt.Errorf("fetch: %w", err)
	}

	if ht.cfg.ContentDispositionDownload {
		w.Header().Set("Content-Disposition", "attachment; filename="+media.Meta().Filename)
	}

	w.Header().Set("Content-Type", media.MIMEType())
	w.Header().Set("Content-Length", strconv.FormatInt(media.Size(), 10))

	if _, err := media.WriteTo(w); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

		return fmt.Errorf("write to: %w", err)
	}

	return nil
}
