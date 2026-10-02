package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// callTimeout bounds one Telegram API call plus its follow-up store writes.
// In-flight work runs on its own context so a shutdown lets it finish.
const callTimeout = 30 * time.Second

// Sender is the single outbox worker. Every row goes to each chat in
// chats; chats[0] is the primary, whose card rows the engine owns.
type Sender struct {
	client   *Client
	chats    []int64
	outbox   Outbox
	renderer Renderer
	log      *slog.Logger
	limiter  *limiter
	wake     chan struct{}
}

// NewSender builds the worker for chats (at least one); call Run to start
// it.
func NewSender(client *Client, chats []int64, outbox Outbox, renderer Renderer, logger *slog.Logger) *Sender {
	if logger == nil {
		logger = slog.Default()
	}
	return &Sender{
		client:   client,
		chats:    chats,
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
		err = s.processSend(ctx, run, item)
	default:
		s.log.Error("outbox: unknown op, dropping", "id", item.ID, "op", item.Op)
		err = s.outbox.DeleteOutbox(ctx, item.ID, item.Generation)
	}
	if err != nil {
		s.log.Error("outbox: store error", "id", item.ID, "op", item.Op, "err", err)
	}
}

// processCard renders the current object state once and creates or edits
// the card message in every chat. A chat whose message already shows the
// render is skipped, so a retry after a partial failure only touches the
// chats that still need it. The primary chat's card row gates the rest:
// missing or no longer live, the row is dropped.
func (s *Sender) processCard(ctx, run context.Context, item *OutboxItem) error {
	kind, pid, oid := item.CardKind, item.CardProjectID, item.CardObjectID
	primary, err := s.outbox.GetCard(ctx, s.chats[0], kind, pid, oid)
	if err != nil {
		return err
	}
	if primary == nil || primary.Status != CardLive {
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
	threadID := primary.ThreadID
	if threadID == nil {
		threadID = item.ThreadID
	}
	msg, err := s.renderer.Render(kind, state, threadID)
	if err != nil {
		s.log.Error("outbox: render failed, dropping", "id", item.ID, "kind", kind, "project", pid, "object", oid, "err", err)
		return s.outbox.DeleteOutbox(ctx, item.ID, item.Generation)
	}
	hash := msg.Hash()

	var retry time.Duration
	var lastErr string
	for i, chat := range s.chats {
		card, thread := primary, threadID
		if i > 0 {
			// Forum topics are configured for the primary chat; other chats
			// get their cards in General.
			if card, err = s.outbox.GetCard(ctx, chat, kind, pid, oid); err != nil {
				return err
			}
			thread = card.ThreadID
		}
		if card.Status != CardLive || (card.MessageID != nil && hash == card.RenderedHash) {
			continue
		}
		tgErr, err := s.deliverCard(ctx, run, chat, card, msg, thread, hash, kind, pid, oid)
		if err != nil {
			return err
		}
		if tgErr == nil {
			continue
		}
		if run.Err() != nil {
			return nil
		}
		wait, err := s.fail(ctx, item, chat, tgErr, kind, pid, oid)
		if err != nil {
			return err
		}
		if wait > retry {
			retry, lastErr = wait, tgErr.Error()
		}
	}
	return s.finish(ctx, item, retry, lastErr)
}

// deliverCard posts or edits the card in one chat and records the result.
// tgErr is the Telegram failure, err a store failure.
func (s *Sender) deliverCard(ctx, run context.Context, chat int64, card *Card, msg Message, thread *int64, hash, kind string, pid, oid int64) (tgErr, err error) {
	if card.MessageID == nil {
		sent, usedThread, tgErr := s.send(ctx, run, chat, msg, thread)
		if tgErr != nil {
			return tgErr, nil
		}
		return nil, s.outbox.SetCardMessage(ctx, chat, kind, pid, oid, usedThread, sent.ID, hash)
	}
	if tgErr := s.limiter.wait(run); tgErr != nil {
		return tgErr, nil
	}
	edit := &bot.EditMessageTextParams{
		ChatID:             chat,
		MessageID:          *card.MessageID,
		LinkPreviewOptions: &models.LinkPreviewOptions{IsDisabled: bot.True()},
		Text:               msg.HTML,
		ParseMode:          models.ParseModeHTML,
		ReplyMarkup:        msg.replyMarkup(),
	}
	if _, tgErr := s.client.bot.EditMessageText(ctx, edit); tgErr != nil {
		if o, _ := classify(tgErr); o != outcomeNotModified {
			return tgErr, nil
		}
	}
	return nil, s.outbox.SetCardHash(ctx, chat, kind, pid, oid, hash)
}

// processSend posts the row payload in every chat it has not reached yet.
func (s *Sender) processSend(ctx, run context.Context, item *OutboxItem) error {
	var msg Message
	if err := json.Unmarshal(item.Payload, &msg); err != nil {
		s.log.Error("outbox: bad payload, dropping", "id", item.ID, "op", item.Op, "err", err)
		return s.outbox.DeleteOutbox(ctx, item.ID, item.Generation)
	}
	var retry time.Duration
	var lastErr string
	for i, chat := range s.chats {
		if slices.Contains(item.SentChats, chat) {
			continue
		}
		thread := item.ThreadID
		if i > 0 {
			thread = nil
		}
		if _, _, tgErr := s.send(ctx, run, chat, msg, thread); tgErr != nil {
			if run.Err() != nil {
				return nil
			}
			wait, err := s.fail(ctx, item, chat, tgErr, "", 0, 0)
			if err != nil {
				return err
			}
			if wait > 0 {
				if wait > retry {
					retry, lastErr = wait, tgErr.Error()
				}
				continue
			}
		}
		if len(s.chats) > 1 {
			if err := s.outbox.MarkOutboxSent(ctx, item.ID, chat); err != nil {
				return err
			}
		}
	}
	return s.finish(ctx, item, retry, lastErr)
}

// finish deletes a row every chat is done with, or reschedules it after
// retry when some chat still needs it.
func (s *Sender) finish(ctx context.Context, item *OutboxItem, retry time.Duration, lastErr string) error {
	if retry > 0 {
		return s.outbox.DeferOutbox(ctx, item.ID, time.Now().Add(retry), item.Attempts+1, lastErr)
	}
	return s.outbox.DeleteOutbox(ctx, item.ID, item.Generation)
}

// send posts msg to chat, retrying once without the thread when Telegram
// reports the topic missing, closed or deleted. Each attempt takes a
// rate-limiter slot. It returns the thread actually used.
func (s *Sender) send(ctx, run context.Context, chat int64, msg Message, threadID *int64) (*models.Message, *int64, error) {
	thread := 0
	if threadID != nil {
		thread = int(*threadID)
	}
	post := func(thread int) (*models.Message, error) {
		return s.client.bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chat, MessageThreadID: thread, Text: msg.HTML, ParseMode: models.ParseModeHTML,
			LinkPreviewOptions: &models.LinkPreviewOptions{IsDisabled: bot.True()},
			ReplyMarkup:        msg.replyMarkup(),
		})
	}
	if err := s.limiter.wait(run); err != nil {
		return nil, threadID, err
	}
	sent, err := post(thread)
	if err == nil {
		return sent, threadID, nil
	}
	if o, _ := classify(err); o == outcomeThreadNotFound && threadID != nil {
		s.log.Warn("telegram: thread unavailable, sending to General", "chat", chat, "thread", *threadID, "err", err)
		if err := s.limiter.wait(run); err != nil {
			return nil, threadID, err
		}
		sent, err = post(0)
		if err == nil {
			return sent, nil, nil
		}
	}
	return nil, threadID, err
}

// fail applies the error taxonomy to a failed send or edit in one chat. It
// returns how long to wait before retrying the row, zero when the chat is
// done with (delivered as far as it ever will be). kind/pid/oid name the
// card for outcomeMessageGone/outcomeUneditable; empty kind means the row
// is not a card.
func (s *Sender) fail(ctx context.Context, item *OutboxItem, chat int64, err error, kind string, pid, oid int64) (time.Duration, error) {
	o, wait := classify(err)
	switch o {
	case outcomeRateLimited:
		s.log.Warn("telegram: rate limited", "id", item.ID, "chat", chat, "retry_after", wait)
		return wait, nil
	case outcomeMessageGone, outcomeUneditable:
		status := CardDeleted
		if o == outcomeUneditable {
			status = CardUneditable
		}
		s.log.Warn("telegram: card message unavailable", "id", item.ID, "chat", chat, "kind", kind, "project", pid, "object", oid, "status", status, "err", err)
		if kind != "" {
			return 0, s.outbox.SetCardStatus(ctx, chat, kind, pid, oid, status)
		}
		return 0, nil
	case outcomePermanent:
		var mig *bot.MigrateError
		if errors.As(err, &mig) {
			s.log.Error("telegram: chat migrated to a supergroup, update GITGRAM_CHAT_ID", "id", item.ID, "chat", chat, "migrate_to_chat_id", mig.MigrateToChatID)
		}
		s.log.Error("telegram: permanent failure, giving up on this chat", "id", item.ID, "chat", chat, "op", item.Op, "err", err)
		return 0, nil
	case outcomeNotModified:
		return 0, nil
	case outcomeThreadNotFound:
		// The retry without a thread already failed; fall through to backoff.
	}
	wait = backoff(item.Attempts)
	s.log.Warn("telegram: transient failure", "id", item.ID, "chat", chat, "op", item.Op, "attempt", item.Attempts+1, "retry_in", wait, "err", err)
	return wait, nil
}
