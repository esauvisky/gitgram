package telegram

import (
	"context"
	"slices"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (c *Client) registerCommandRoute() {
	c.bot.RegisterHandlerMatchFunc(
		func(u *models.Update) bool {
			return u.Message != nil && strings.HasPrefix(u.Message.Text, "/")
		},
		c.onCommand,
	)
}

// onCommand parses "/name[@bot] args" from a configured chat and hands it
// to OnCommand when the name is one the bot offers.
func (c *Client) onCommand(ctx context.Context, _ *bot.Bot, u *models.Update) {
	m := u.Message
	if c.opts.OnCommand == nil || !slices.Contains(c.opts.ChatIDs, m.Chat.ID) || m.From == nil {
		return
	}
	head, args, _ := strings.Cut(m.Text, " ")
	name := strings.TrimPrefix(head, "/")
	if i := strings.IndexByte(name, '@'); i >= 0 {
		name = name[:i]
	}
	for _, cmd := range c.opts.Commands {
		if cmd.Name == name {
			c.opts.OnCommand(ctx, CommandCall{Name: name, Args: strings.TrimSpace(args), ChatID: m.Chat.ID, UserID: m.From.ID, MessageID: m.ID})
			return
		}
	}
}
