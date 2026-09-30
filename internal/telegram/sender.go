package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// callTimeout bounds one Telegram API call plus its follow-up store writes.
// In-flight work runs on its own context so a shutdown lets it finish.
const callTimeout = 30 * time.Second

// Sender is the single outbox worker for the one chat.
type Sender struct {
	client   *Client
	outbox   Outbox
	renderer Renderer
	log      *slog.Logger
	limiter  *limiter
	wake     chan struct{}
}

// NewSender builds the worker; call Run to start it.
func NewSender(client *Client, outbox Outbox, renderer Renderer, logger *slog.Logger) *Sender {
	if logger == nil {
		logger = slog.Default()
	}
	return &Sender{
		client:   client,
		outbox:   outbox,
		renderer: renderer,
		log:      logger,
		limiter:  newLimiter(),
		wake:     make(chan struct{}, 1),
	}
}

// Notify wakes the loop after outbox rows were inserted or rescheduled.
// Never blocks.
func (s *Sender) Notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Run drains the outbox until ctx is cancelled. It returns after the
// in-flight request (if any) has completed.
func (s *Sender) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		item, err := s.outbox.OutboxHead(ctx)
		if err != nil {
			s.log.Error("outbox head", "err", err)
			if !s.sleep(ctx, backoffMin) {
				return
			}
			continue
		}
		if item == nil {
			select {
			case <-ctx.Done():
				return
			case <-s.wake:
			}
			continue
		}
		if wait := time.Until(item.NotBefore); wait > 0 {
			if !s.sleep(ctx, wait) {
				return
			}
			continue // re-read head: a Notify may have changed the queue
		}
		s.process(ctx, item)
	}
}

// sleep waits for d or a Notify. False means ctx is done.
func (s *Sender) sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
	case <-s.wake:
	}
	return true
}

// process handles one row. Store and Telegram calls run on a detached
// context so shutdown never cuts a call or its bookkeeping in half; run is
// the Run context, consulted only while waiting for a rate-limiter slot so a
// shutdown during the wait leaves the row queued for the next start.
func (s *Sender) process(run context.Context, item *OutboxItem) {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()

	var err error
	switch item.Op {
	case OpCard:
		err = s.processCard(ctx, run, item)
	case OpSend:
		err = s.processSend(ctx, run, item, nil)
	case OpReply:
		err = s.processReply(ctx, run, item)
	default:
		s.log.Error("outbox: unknown op, dropping", "id", item.ID, "op", item.Op)
		err = s.outbox.DeleteOutbox(ctx, item.ID, item.Generation)
	}
	if err != nil {
		s.log.Error("outbox: store error", "id", item.ID, "op", item.Op, "err", err)
	}
}

// processCard renders the current object state and creates or edits the
// card message.
func (s *Sender) processCard(ctx, run context.Context, item *OutboxItem) error {
	kind, pid, oid := item.CardKind, item.CardProjectID, item.CardObjectID
	card, err := s.outbox.GetCard(ctx, kind, pid, oid)
	if err != nil {
		return err
	}
	if card == nil || card.Status != CardLive {
		s.log.Debug("outbox: card missing or not live, dropping", "id", item.ID, "kind", kind, "project", pid, "object", oid)
		return s.outbox.DeleteOutbox(ctx, item.ID, item.Generation)
	}
	state, err := s.outbox.GetObject(ctx, kind, pid, oid)
	if err != nil {
		return err
	}
	if state == nil {
		s.log.Warn("outbox: object state missing, dropping", "id", item.ID, "kind", kind, "project", pid, "object", oid)
		return s.outbox.DeleteOutbox(ctx, item.ID, item.Generation)
	}
	threadID := card.ThreadID
	if threadID == nil {
		threadID = item.ThreadID
	}
	msg, err := s.renderer.Render(kind, state, threadID)
	if err != nil {
		s.log.Error("outbox: render failed, dropping", "id", item.ID, "kind", kind, "project", pid, "object", oid, "err", err)
		return s.outbox.DeleteOutbox(ctx, item.ID, item.Generation)
	}
	hash := msg.Hash()
	if card.MessageID != nil && hash == card.RenderedHash {
		return s.outbox.DeleteOutbox(ctx, item.ID, item.Generation)
	}

	if card.MessageID == nil {
		sent, usedThread, err := s.send(ctx, run, msg, threadID, nil)
		if err != nil {
			if run.Err() != nil {
				return nil
			}
			return s.fail(ctx, item, err, kind, pid, oid)
		}
		if err := s.outbox.SetCardMessage(ctx, kind, pid, oid, usedThread, sent.ID, hash); err != nil {
			return err
		}
		return s.outbox.DeleteOutbox(ctx, item.ID, item.Generation)
	}

	if err := s.limiter.wait(run); err != nil {
		return nil
	}
	_, err = s.client.bot.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:             s.client.opts.ChatID,
		MessageID:          *card.MessageID,
		Text:               msg.HTML,
		ParseMode:          models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{IsDisabled: bot.True()},
		ReplyMarkup:        msg.replyMarkup(),
	})
	if err != nil {
		if o, _ := classify(err); o != outcomeNotModified {
			return s.fail(ctx, item, err, kind, pid, oid)
		}
	}
	if err := s.outbox.SetCardHash(ctx, kind, pid, oid, hash); err != nil {
		return err
	}
	return s.outbox.DeleteOutbox(ctx, item.ID, item.Generation)
}

// processSend posts a standalone message from the row payload.
func (s *Sender) processSend(ctx, run context.Context, item *OutboxItem, reply *models.ReplyParameters) error {
	var msg Message
	if err := json.Unmarshal(item.Payload, &msg); err != nil {
		s.log.Error("outbox: bad payload, dropping", "id", item.ID, "op", item.Op, "err", err)
		return s.outbox.DeleteOutbox(ctx, item.ID, item.Generation)
	}
	if _, _, err := s.send(ctx, run, msg, item.ThreadID, reply); err != nil {
		if run.Err() != nil {
			return nil
		}
		return s.fail(ctx, item, err, "", 0, 0)
	}
	return s.outbox.DeleteOutbox(ctx, item.ID, item.Generation)
}

// processReply posts the payload as a reply to the anchor card's message,
// in the topic that message lives in; when the anchor has no message the
// payload goes out standalone in the row's own thread.
func (s *Sender) processReply(ctx, run context.Context, item *OutboxItem) error {
	card, err := s.outbox.GetCard(ctx, item.CardKind, item.CardProjectID, item.CardObjectID)
	if err != nil {
		return err
	}
	var reply *models.ReplyParameters
	if card != nil && card.MessageID != nil {
		reply = &models.ReplyParameters{MessageID: *card.MessageID, AllowSendingWithoutReply: true}
		item.ThreadID = card.ThreadID
	}
	return s.processSend(ctx, run, item, reply)
}

// send posts msg, retrying once without the thread when Telegram reports
// the topic missing, closed or deleted. Each attempt takes a rate-limiter
// slot. It returns the thread actually used.
func (s *Sender) send(ctx, run context.Context, msg Message, threadID *int64, reply *models.ReplyParameters) (*models.Message, *int64, error) {
	params := &bot.SendMessageParams{
		ChatID:             s.client.opts.ChatID,
		Text:               msg.HTML,
		ParseMode:          models.ParseModeHTML,
		LinkPreviewOptions: &models.LinkPreviewOptions{IsDisabled: bot.True()},
		ReplyParameters:    reply,
		ReplyMarkup:        msg.replyMarkup(),
	}
	if threadID != nil {
		params.MessageThreadID = int(*threadID)
	}
	if err := s.limiter.wait(run); err != nil {
		return nil, threadID, err
	}
	sent, err := s.client.bot.SendMessage(ctx, params)
	if err == nil {
		return sent, threadID, nil
	}
	if o, _ := classify(err); o == outcomeThreadNotFound && threadID != nil {
		s.log.Warn("telegram: thread unavailable, sending to General", "thread", *threadID, "err", err)
		params.MessageThreadID = 0
		if err := s.limiter.wait(run); err != nil {
			return nil, threadID, err
		}
		sent, err = s.client.bot.SendMessage(ctx, params)
		if err == nil {
			return sent, nil, nil
		}
	}
	return nil, threadID, err
}

// fail applies the error taxonomy to a failed send/edit. kind/pid/oid name
// the card for outcomeMessageGone/outcomeUneditable; empty kind means the
// row is not a card.
func (s *Sender) fail(ctx context.Context, item *OutboxItem, err error, kind string, pid, oid int64) error {
	o, wait := classify(err)
	switch o {
	case outcomeRateLimited:
		s.log.Warn("telegram: rate limited", "id", item.ID, "retry_after", wait)
		return s.outbox.DeferOutbox(ctx, item.ID, time.Now().Add(wait), item.Attempts+1, err.Error())
	case outcomeMessageGone, outcomeUneditable:
		status := CardDeleted
		if o == outcomeUneditable {
			status = CardUneditable
		}
		s.log.Warn("telegram: card message unavailable", "id", item.ID, "kind", kind, "project", pid, "object", oid, "status", status, "err", err)
		if kind != "" {
			if err := s.outbox.SetCardStatus(ctx, kind, pid, oid, status); err != nil {
				return err
			}
		}
		return s.outbox.DeleteOutbox(ctx, item.ID, item.Generation)
	case outcomePermanent:
		var mig *bot.MigrateError
		if errors.As(err, &mig) {
			s.log.Error("telegram: chat migrated to a supergroup, update telegram.chat_id", "id", item.ID, "migrate_to_chat_id", mig.MigrateToChatID)
		}
		s.log.Error("telegram: permanent failure, dropping", "id", item.ID, "op", item.Op, "err", err)
		return s.outbox.DeleteOutbox(ctx, item.ID, item.Generation)
	case outcomeNotModified:
		return s.outbox.DeleteOutbox(ctx, item.ID, item.Generation)
	case outcomeThreadNotFound:
		// The retry without a thread already failed; fall through to backoff.
	}
	wait = backoff(item.Attempts)
	s.log.Warn("telegram: transient failure", "id", item.ID, "op", item.Op, "attempt", item.Attempts+1, "retry_in", wait, "err", err)
	return s.outbox.DeferOutbox(ctx, item.ID, time.Now().Add(wait), item.Attempts+1, err.Error())
}
