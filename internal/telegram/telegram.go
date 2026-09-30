// Package telegram wraps github.com/go-telegram/bot for Gitgram: the Client
// owns the bot connection (webhook or long polling) and the callback-query
// route; the Sender drains the outbox into one chat with rate limiting,
// render-at-send hashing and the Telegram error taxonomy. All bot/models
// types stay inside this package.
package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/esauvisky/gitgram/internal/actions"
)

// Transport modes accepted by Options.Mode.
const (
	ModeWebhook = "webhook"
	ModePolling = "polling"
)

// Options configures New.
type Options struct {
	// ChatIDs are the groups the bot serves: commands are accepted from
	// any of them.
	ChatIDs []int64
	// Mode is ModeWebhook or ModePolling.
	Mode string
	// PublicURL is the externally reachable base URL (no trailing slash);
	// webhook mode registers PublicURL + WebhookPath + "/" + WebhookSecret.
	PublicURL string
	// WebhookPath is the local path the Handler is mounted under.
	WebhookPath string
	// WebhookSecret is both the URL suffix and the secret_token Telegram
	// echoes in X-Telegram-Bot-Api-Secret-Token.
	WebhookSecret string
	// Logger receives bot library errors and route logs. Nil uses slog.Default().
	Logger *slog.Logger
	// OnCallback handles a decoded inline-button press. Nil answers every
	// button with actions.Unsupported.
	OnCallback func(context.Context, actions.Request) actions.Result
	// Commands lists the slash commands the bot answers, registered with
	// Telegram at Start so they autocomplete; OnCommand receives them.
	Commands  []Command
	OnCommand func(context.Context, CommandCall)
}

// Command is one slash command the bot offers.
type Command struct {
	Name        string
	Description string
}

// CommandCall is a slash command someone sent in the chat.
type CommandCall struct {
	Name      string
	Args      string
	ChatID    int64
	UserID    int64
	MessageID int
}

// Client is the bot connection.
type Client struct {
	bot  *bot.Bot
	opts Options
	log  *slog.Logger
	me   int64
}

// New builds the client without touching the network (getMe is skipped;
// call Me to verify the token). The callback-query route is registered here.
func New(token string, opts Options) (*Client, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Mode != ModeWebhook && opts.Mode != ModePolling {
		return nil, fmt.Errorf("telegram: unknown mode %q", opts.Mode)
	}
	if opts.OnCallback == nil {
		opts.OnCallback = func(ctx context.Context, req actions.Request) actions.Result {
			res, _ := actions.Unsupported{}.Dispatch(ctx, req)
			return res
		}
	}
	c := &Client{opts: opts, log: opts.Logger}

	botOpts := []bot.Option{
		bot.WithSkipGetMe(),
		bot.WithAllowedUpdates(bot.AllowedUpdates{"message", "callback_query"}),
		bot.WithErrorsHandler(func(err error) { c.log.Error("telegram bot", "err", err) }),
		bot.WithDebugHandler(func(format string, args ...any) { c.log.Debug(fmt.Sprintf(format, args...)) }),
		// Unmatched updates (plain group messages) are ignored instead of
		// hitting the library's log.Printf default.
		bot.WithDefaultHandler(func(context.Context, *bot.Bot, *models.Update) {}),
	}
	if opts.Mode == ModeWebhook && opts.WebhookSecret != "" {
		botOpts = append(botOpts, bot.WithWebhookSecretToken(opts.WebhookSecret))
	}
	b, err := bot.New(token, botOpts...)
	if err != nil {
		return nil, fmt.Errorf("telegram: %w", err)
	}
	c.bot = b
	c.registerCallbackRoute()
	c.registerCommandRoute()
	return c, nil
}

// Me calls getMe and returns the bot's username.
func (c *Client) Me(ctx context.Context) (string, error) {
	me, err := c.bot.GetMe(ctx)
	if err != nil {
		return "", fmt.Errorf("telegram: getMe: %w", err)
	}
	c.me = me.ID
	return me.Username, nil
}

// Start configures the transport and starts consuming updates in the
// background until ctx is cancelled. Webhook mode calls setWebhook with the
// secret token and allowed_updates [message, callback_query]; polling mode
// deletes any webhook and long-polls. Setup errors are returned synchronously.
func (c *Client) Start(ctx context.Context) error {
	if len(c.opts.Commands) > 0 {
		cmds := make([]models.BotCommand, len(c.opts.Commands))
		for i, cmd := range c.opts.Commands {
			cmds[i] = models.BotCommand{Command: cmd.Name, Description: cmd.Description}
		}
		if _, err := c.bot.SetMyCommands(ctx, &bot.SetMyCommandsParams{Commands: cmds}); err != nil {
			return fmt.Errorf("telegram: setMyCommands: %w", err)
		}
	}
	switch c.opts.Mode {
	case ModeWebhook:
		url := strings.TrimRight(c.opts.PublicURL, "/") + c.opts.WebhookPath + "/" + c.opts.WebhookSecret
		if _, err := c.bot.SetWebhook(ctx, &bot.SetWebhookParams{
			URL:            url,
			AllowedUpdates: []string{"message", "callback_query"},
			SecretToken:    c.opts.WebhookSecret,
		}); err != nil {
			return fmt.Errorf("telegram: setWebhook: %w", err)
		}
		c.log.Info("telegram webhook set", "url", strings.Replace(url, c.opts.WebhookSecret, "***", 1))
		go c.bot.StartWebhook(ctx)
	case ModePolling:
		if _, err := c.bot.DeleteWebhook(ctx, &bot.DeleteWebhookParams{}); err != nil {
			return fmt.Errorf("telegram: deleteWebhook: %w", err)
		}
		c.log.Info("telegram polling started")
		go c.bot.Start(ctx)
	}
	return nil
}

// SetOnCommand installs the command handler after construction, for
// callers whose handler needs things built after New.
func (c *Client) SetOnCommand(fn func(context.Context, CommandCall)) { c.opts.OnCommand = fn }

// Reply posts a plain-text reply to a message in chatID, outside the
// outbox: for command acknowledgements only.
func (c *Client) Reply(ctx context.Context, chatID int64, messageID int, text string) error {
	_, err := c.bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID, Text: text,
		ReplyParameters: &models.ReplyParameters{MessageID: messageID, AllowSendingWithoutReply: true},
	})
	if err != nil {
		return fmt.Errorf("telegram: reply: %w", err)
	}
	return nil
}

// Handler is the webhook endpoint for Telegram updates. Mount it at
// WebhookPath + "/" + WebhookSecret; the secret_token header is verified by
// the bot library.
func (c *Client) Handler() http.Handler {
	return c.bot.WebhookHandler()
}
