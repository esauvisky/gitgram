package webhook

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
)

// DeliveryKey returns the deduplication key for one delivery. Idempotency-Key
// (older GitLab: webhook-id) is the only header that stays the same across
// automatic retries and manual resends of one hook; X-Gitlab-Event-UUID is
// generated per delivery attempt and is not used. The body hash is the last
// resort.
func DeliveryKey(h http.Header, body []byte) string {
	for _, name := range [...]string{"Idempotency-Key", "Webhook-Id"} {
		if v := h.Get(name); v != "" {
			return v
		}
	}
	sum := sha256.Sum256(body)
	return "h:" + hex.EncodeToString(sum[:])
}

// VerifyToken reports whether the X-Gitlab-Token header equals secret, in
// constant time.
func VerifyToken(h http.Header, secret string) bool {
	return subtle.ConstantTimeCompare([]byte(h.Get("X-Gitlab-Token")), []byte(secret)) == 1
}
