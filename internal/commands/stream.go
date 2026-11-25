package commands

import (
	"strings"

	"github.com/celestix/gotgproto/dispatcher"
	"github.com/celestix/gotgproto/dispatcher/handlers"
	"github.com/celestix/gotgproto/ext"
)

// The catch-all media handler lives in group 10 so every command handler
// (group 0) gets a chance first, regardless of load order.
func (m *command) LoadStream(d dispatcher.Dispatcher) {
	log := m.log.Named("stream")
	defer log.Sugar().Info("Loaded")
	d.AddHandlerToGroup(handlers.NewMessage(nil, sendLink), 10)
}

func sendLink(ctx *ext.Context, u *ext.Update) error {
	userID, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	msg := u.EffectiveMessage
	if msg.Media == nil {
		if strings.HasPrefix(strings.TrimSpace(msg.Text), "/") {
			reply(ctx, u, p("🤔 Unknown command. Send /help to see what I can do."))
		} else {
			reply(ctx, u, p("📎 Send me any file, video, audio, photo or document and I'll give you a direct link.\n\nSee /help for everything else."))
		}
		return endGroups()
	}
	rec, err := createLink(ctx, userID, msg.ID, msg.Media)
	if err != nil {
		if le, ok := err.(*linkError); ok {
			reply(ctx, u, p("⚠️ "+le.msg))
		} else {
			reply(ctx, u, p("❌ Error - "+err.Error()))
		}
		return endGroups()
	}
	text, markup := linkMessage(rec)
	_, _ = ctx.Reply(u, ext.ReplyTextStyledTextArray(text), &ext.ReplyOpts{
		Markup:           markup,
		NoWebpage:        true,
		ReplyToMessageId: msg.ID,
	})
	return endGroups()
}
