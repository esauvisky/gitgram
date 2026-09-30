package telegram

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/esauvisky/gitgram/internal/actions"
)

func (c *Client) registerCallbackRoute() {
	c.bot.RegisterHandlerMatchFunc(
		func(u *models.Update) bool { return u.CallbackQuery != nil },
		c.onCallbackQuery,
	)
}

// onCallbackQuery decodes the pressed button, runs OnCallback and always
// answers the query (Telegram keeps the button spinning otherwise).
func (c *Client) onCallbackQuery(ctx context.Context, b *bot.Bot, u *models.Update) {
	cq := u.CallbackQuery
	answer := &bot.AnswerCallbackQueryParams{CallbackQueryID: cq.ID}

	cb, err := actions.Decode(cq.Data)
	if err != nil {
		c.log.Warn("telegram callback: undecodable data", "data", cq.Data, "err", err)
		answer.Text = "This button is no longer valid."
	} else {
		req := actions.Request{Callback: cb, TelegramUserID: cq.From.ID}
		switch cq.Message.Type {
		case models.MaybeInaccessibleMessageTypeMessage:
			req.ChatID = cq.Message.Message.Chat.ID
			req.MessageID = cq.Message.Message.ID
		case models.MaybeInaccessibleMessageTypeInaccessibleMessage:
			req.ChatID = cq.Message.InaccessibleMessage.Chat.ID
			req.MessageID = cq.Message.InaccessibleMessage.MessageID
		}
		res := c.opts.OnCallback(ctx, req)
		answer.Text = res.Toast
		answer.ShowAlert = res.Alert
	}

	if _, err := b.AnswerCallbackQuery(ctx, answer); err != nil {
		c.log.Warn("telegram callback: answer failed", "err", err)
	}
}
