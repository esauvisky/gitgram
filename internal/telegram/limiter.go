package telegram

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

// Telegram's per-group budget: 1 message/s with a little burst, and 20 per
// minute measured over a sliding window. Edits share it.
const (
	windowSize = 20
	windowSpan = time.Minute
)

// limiter combines a token bucket (1/s, burst 3) with a sliding window of
// the last windowSize send timestamps.
type limiter struct {
	bucket *rate.Limiter
	sent   []time.Time
}

func newLimiter() *limiter {
	return &limiter{
		bucket: rate.NewLimiter(rate.Every(time.Second), 3),
		sent:   make([]time.Time, 0, windowSize),
	}
}

// wait blocks until one more request is allowed, then records it.
func (l *limiter) wait(ctx context.Context) error {
	if err := l.bucket.Wait(ctx); err != nil {
		return err
	}
	now := time.Now()
	cutoff := now.Add(-windowSpan)
	i := 0
	for i < len(l.sent) && !l.sent[i].After(cutoff) {
		i++
	}
	l.sent = l.sent[i:]
	if len(l.sent) >= windowSize {
		until := l.sent[0].Add(windowSpan)
		t := time.NewTimer(until.Sub(now))
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case now = <-t.C:
		}
		l.sent = l.sent[1:]
	}
	l.sent = append(l.sent, now)
	return nil
}
