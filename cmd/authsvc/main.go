package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/mkrupp/homecase-michael/internal/infra/config"
	"github.com/mkrupp/homecase-michael/internal/infra/logging"
	"github.com/mkrupp/homecase-michael/internal/infra/transport/http"
	"github.com/mkrupp/homecase-michael/internal/repo/user"
	"github.com/mkrupp/homecase-michael/internal/svc/authsvc"
)

const (
	appName = "demo"
	svcName = "authsvc"
)

type Config struct {
	config.EnvConfig

	Log  logging.LoggerConfig            `envPrefix:"LOG_"`
	Auth authsvc.AuthConfig              `envPrefix:"AUTH_"`
	HTTP authsvc.HTTPTransportConfig     `envPrefix:"HTTP_"`
	User user.SQLiteUserRepositoryConfig `envPrefix:"USER_"`
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
		log := logging.GetLogger("cmd.authsvc")

		if err != nil {
			log.ErrorContext(ctx, "error", "err", err)
			panic(err)
		}

		log.InfoContext(ctx, "shutdown")
	}()

	authSvc, err := authsvc.NewAuthService(
		user.SQLiteUserRepositoryFactory(cfg.User),
		cfg.Auth,
	)
	if err != nil {
		return fmt.Errorf("new auth service: %w", err)
	}

	httpTransport := authsvc.NewHTTPTransport(authSvc, cfg.HTTP)

	if err := http.ListenAndServe(ctx, httpTransport, cfg.HTTP.HTTPTransportConfig); err != nil {
		return fmt.Errorf("listen and serve: %w", err)
	}

	return nil
}
