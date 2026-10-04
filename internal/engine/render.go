package engine

import (
	"encoding/json"
	"fmt"

	"github.com/esauvisky/gitgram/internal/actions"
	"github.com/esauvisky/gitgram/internal/cards"
	"github.com/esauvisky/gitgram/internal/event"
	"github.com/esauvisky/gitgram/internal/render"
	"github.com/esauvisky/gitgram/internal/telegram"
)

var _ telegram.Renderer = (*Engine)(nil)

// Render implements telegram.Renderer: it decodes the stored state by card
// kind and renders it with the owning project's options.
func (e *Engine) Render(kind string, stateJSON []byte, _ *int64) (telegram.Message, error) {
	var msg render.Message
	switch cards.Kind(kind) {
	case cards.KindPipeline:
		var s cards.PipelineState
		if err := json.Unmarshal(stateJSON, &s); err != nil {
			return telegram.Message{}, fmt.Errorf("decode %s state: %w", kind, err)
		}
		msg = render.Pipeline(&s, e.options(s.Project))
	case cards.KindMR:
		var s cards.MRState
		if err := json.Unmarshal(stateJSON, &s); err != nil {
			return telegram.Message{}, fmt.Errorf("decode %s state: %w", kind, err)
		}
		msg = render.MergeRequest(&s, e.options(s.Project))
	case cards.KindPush:
		var s cards.PushState
		if err := json.Unmarshal(stateJSON, &s); err != nil {
			return telegram.Message{}, fmt.Errorf("decode %s state: %w", kind, err)
		}
		msg = render.Push(&s, e.options(s.Project))
	default:
		return telegram.Message{}, fmt.Errorf("render: unknown card kind %q", kind)
	}
	return toTelegram(msg), nil
}

// options builds the render options for a project: the buttons the bot
// can offer on its host.
func (e *Engine) options(p event.Project) render.Options {
	var caps actions.Capabilities = actions.None{}
	switch {
	case p.IsGitHub():
		caps = githubCaps{e}
	case e.writer != nil:
		caps = e
	}
	return render.Options{
		Caps: caps,
	}
}

// toTelegram converts a render.Message into the sender's mirror type.
func toTelegram(m render.Message) telegram.Message {
	out := telegram.Message{HTML: m.HTML}
	if m.Keyboard == nil {
		return out
	}
	out.Keyboard = make([][]telegram.Button, len(m.Keyboard))
	for i, row := range m.Keyboard {
		out.Keyboard[i] = make([]telegram.Button, len(row))
		for j, b := range row {
			out.Keyboard[i][j] = telegram.Button{Text: b.Text, Data: b.Data}
		}
	}
	return out
}
