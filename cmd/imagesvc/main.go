package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/mkrupp/homecase-michael/internal/infra/config"
	"github.com/mkrupp/homecase-michael/internal/infra/logging"
	"github.com/mkrupp/homecase-michael/internal/infra/transport/http"
	"github.com/mkrupp/homecase-michael/internal/repo/blob"
	"github.com/mkrupp/homecase-michael/internal/svc/authsvc/authclient"
	"github.com/mkrupp/homecase-michael/internal/svc/imagesvc"
	"github.com/mkrupp/homecase-michael/internal/svc/mediasvc"
)

const (
	appName = "demo"
	svcName = "imagesvc"
)

type Config struct {
	config.EnvConfig

	Log        logging.LoggerConfig                `envPrefix:"LOG_"`
	Media      mediasvc.MediaConfig                `envPrefix:"MEDIA_"`
	Image      imagesvc.ImageConfig                `envPrefix:"IMAGE_"`
	ImageHTTP  imagesvc.HTTPTransportConfig        `envPrefix:"IMAGE_HTTP_"`
	AuthClient authclient.HTTPClientConfig         `envPrefix:"AUTH_CLIENT_"`
	Blob       blob.FileSystemBlobRepositoryConfig `envPrefix:"BLOB_"`
}

func main() {
	var (
		cfg Config
		ctx = context.Background()

		configPrefix = strings.ToUpper(strings.Join([]string{appName, svcName}, "_"))
		loggerName   = strings.ToLower(strings.Join([]string{appName, svcName}, "."))
	)

	if err := config.Parse(ctx, &cfg, configPrefix); err != nil {
		panic(err)
	}

	logging.Configure(ctx, cfg.Log, loggerName)

	if err := run(ctx, cfg); err != nil {
		panic(err)
	}
}

func run(ctx context.Context, cfg Config) (err error) {
	defer func() {
		log := logging.GetLogger("cmd.imagesvc")

		if err != nil {
			log.ErrorContext(ctx, "error", "err", err)
			panic(err)
		}

		log.InfoContext(ctx, "shutdown")
	}()

	mediaSvc, err := mediasvc.NewBlobMediaService(
		ctx,
		blob.FileSystemBlobRepositoryFactory(cfg.Blob),
		cfg.Media,
	)
	if err != nil {
		return fmt.Errorf("new media service: %w", err)
	}

	authClient := authclient.NewHTTPClient(cfg.AuthClient, nil)

	imageSvc, err := imagesvc.NewBlobImageService(
		ctx,
		blob.FileSystemBlobRepositoryFactory(cfg.Blob),
		mediaSvc,
		authClient,
		cfg.Image,
	)
	if err != nil {
		return fmt.Errorf("new image service: %w", err)
	}

	httpTransport := imagesvc.NewHTTPTransport(imageSvc, authClient, cfg.ImageHTTP)

	if err := http.ListenAndServe(ctx, httpTransport, cfg.ImageHTTP.HTTPTransportConfig); err != nil {
		return fmt.Errorf("listen and serve: %w", err)
	}

	return nil
}
