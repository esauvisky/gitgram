package webhook

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/esauvisky/gitgram/internal/event"
)

// NewHandler returns the HTTP handler for the GitLab webhook endpoint. It
// rejects bodies over maxBody bytes (413), verifies X-Gitlab-Token against
// secret (401), parses the payload and calls handle with the delivery key
// and the event. Ignored kinds and malformed payloads answer 200 so GitLab
// never counts them as failures (it disables hooks after consecutive
// non-2xx responses); only an error from handle yields 500. A nil logger
// uses slog.Default().
func NewHandler(secret string, maxBody int64, handle func(ctx context.Context, key string, ev event.Event) error, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !VerifyToken(r.Header, secret) {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
		if err != nil {
			var tooBig *http.MaxBytesError
			if errors.As(err, &tooBig) {
				http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
				return
			}
			logger.Warn("webhook: read body", "err", err)
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}
		received := time.Now()
		key := DeliveryKey(r.Header, body)
		header := r.Header.Get("X-Gitlab-Event")

		ev, err := Parse(header, body, received)
		if err != nil {
			if errors.Is(err, ErrIgnored) {
				logger.Debug("webhook: ignored", "event", header, "key", key, "reason", err)
				plain(w, "ignored")
				return
			}
			logger.Warn("webhook: unparsable payload", "event", header, "key", key, "err", err)
			plain(w, "unparsed")
			return
		}
		// GitLab drops the connection after 10 s and never retries; the
		// write path must not be aborted by the socket closing, so handle
		// runs detached from the request's cancellation.
		if err := handle(context.WithoutCancel(r.Context()), key, ev); err != nil {
			logger.Error("webhook: handle failed", "kind", ev.EventKind(), "project", ev.Proj().Path, "key", key, "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		plain(w, "ok")
	})
}

func plain(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, msg+"\n")
}
