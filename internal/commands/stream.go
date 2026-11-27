package commands

import (
	"strings"

	"EverythingSuckz/fsb/internal/i18n"

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
	lang := userLang(userID)
	if msg.Media == nil {
		if strings.HasPrefix(strings.TrimSpace(msg.Text), "/") {
			if strings.EqualFold(strings.Fields(msg.Text)[0], "/cancel") {
				if clearPending(userID) {
					reply(ctx, u, p(i18n.T(lang, "cancelled")))
				}
				return endGroups()
			}
			reply(ctx, u, p(i18n.T(lang, "unknown_cmd")))
		} else if handlePendingText(ctx, u, userID, lang) {
			return endGroups()
		} else {
			reply(ctx, u, p(i18n.T(lang, "send_file")))
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
	text, markup := linkMessage(rec, lang)
	_, _ = ctx.Reply(u, ext.ReplyTextStyledTextArray(text), &ext.ReplyOpts{
		Markup:           markup,
		NoWebpage:        true,
		ReplyToMessageId: msg.ID,
	})
	return endGroups()
}
