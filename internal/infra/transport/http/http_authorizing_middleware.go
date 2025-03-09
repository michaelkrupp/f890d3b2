package http

import (
	"net/http"

	context_ "github.com/mkrupp/homecase-michael/internal/infra/context"
	"github.com/mkrupp/homecase-michael/internal/infra/logging"
	"github.com/mkrupp/homecase-michael/internal/svc/authsvc/authclient"
)

// AuthorizingMiddleware creates middleware that validates authentication tokens.
// It requires an AuthClient for token validation.
// Requests without a valid token in the Authorization header are rejected.
// On successful validation, the username is added to the request context.
func AuthorizingMiddleware(
	next http.Handler,
	authClient authclient.AuthClient,
	log logging.Logger,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			log.ErrorContext(r.Context(), "no token provided")
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)

			return
		}

		username, ok, err := authClient.Validate(r.Context(), token)
		if err != nil {
			log.ErrorContext(r.Context(), "validate token failed", "error", err)
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)

			return
		} else if !ok {
			log.ErrorContext(r.Context(), "invalid token")
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)

			return
		}

		next.ServeHTTP(w, r.WithContext(context_.WithUsername(r.Context(), username)))
	})
}
