package telegram

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/go-telegram/bot/models"
)

// Message is a rendered Telegram message: HTML body plus inline keyboard.
// It mirrors render.Message field for field so the engine can convert with
// a plain struct literal and outbox payloads written as render.Message JSON
// unmarshal into it directly.
type Message struct {
	// HTML is Bot API HTML for classic messages; Rich is rich message HTML.
	// Exactly one is set.
	HTML     string
	Rich     string `json:",omitempty"`
	Keyboard [][]Button
}

// Button is one inline keyboard button; Data is callback_data (an encoded
// actions.Callback). The JSON tags match render.Button so outbox payloads
// round-trip exactly.
type Button struct {
	Text string `json:"text"`
	Data string `json:"data,omitempty"`
}

// Hash is sha256(HTML + canonical keyboard JSON), hex encoded. It is what
// card_messages.rendered_hash stores and the sender compares before editing.
func (m Message) Hash() string {
	h := sha256.New()
	h.Write([]byte(m.HTML))
	h.Write([]byte(m.Rich))
	if len(m.Keyboard) > 0 {
		kb, _ := json.Marshal(m.Keyboard)
		h.Write(kb)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// rich returns the InputRichMessage for a rich message, nil for classic.
func (m Message) rich() *models.InputRichMessage {
	if m.Rich == "" {
		return nil
	}
	return &models.InputRichMessage{HTML: m.Rich, SkipEntityDetection: true}
}

// replyMarkup converts the keyboard to the bot model; nil when empty so the
// field is omitted from the request.
func (m Message) replyMarkup() models.ReplyMarkup {
	if len(m.Keyboard) == 0 {
		return nil
	}
	rows := make([][]models.InlineKeyboardButton, 0, len(m.Keyboard))
	for _, row := range m.Keyboard {
		if len(row) == 0 {
			continue
		}
		r := make([]models.InlineKeyboardButton, 0, len(row))
		for _, b := range row {
			r = append(r, models.InlineKeyboardButton{Text: b.Text, CallbackData: b.Data})
		}
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		return nil
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}
