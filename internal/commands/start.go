package commands

import (
	"EverythingSuckz/fsb/config"

	"github.com/celestix/gotgproto/dispatcher"
	"github.com/celestix/gotgproto/dispatcher/handlers"
	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
)

func (m *command) LoadStart(d dispatcher.Dispatcher) {
	log := m.log.Named("start")
	defer log.Sugar().Info("Loaded")
	d.AddHandler(handlers.NewCommand("start", start))
}

func homeKeyboard() *tg.ReplyInlineMarkup {
	rows := [][]tg.KeyboardButtonClass{
		{btnCB("📖 Help", "ahar:help"), btnCB("📂 My Files", "ahar:files")},
		{btnCB("ℹ️ About", "ahar:about")},
	}
	if config.ValueOf.SupportURL != "" {
		rows[1] = append(rows[1], btnURL("💬 Support", config.ValueOf.SupportURL))
	}
	return kb(rows...)
}

func homeText(name string) []part {
	if name == "" {
		name = "there"
	}
	return parts(
		b("👋 Hey "+name+"!"), nl(), nl(),
		p("Welcome to "), b(config.ValueOf.BotName), p(" ⚡"), nl(), nl(),
		p("Send me any file, video, audio, photo or document and I'll instantly give you a "),
		b("direct download & streaming link"), p("."), nl(), nl(),
		b("✨ What you get"), nl(),
		p("• 🚀 Fast multi-bot streaming with seeking"), nl(),
		p("• 🎞 A web player for videos, audio, images and PDFs"), nl(),
		p("• ⏳ Expiring links and 🗑 one-tap revoke"), nl(),
		p("• 📊 Download counters for every link"), nl(), nl(),
		i("Just send a file to begin — or tap Help below."),
	)
}

func start(ctx *ext.Context, u *ext.Update) error {
	if _, ok := gate(ctx, u); !ok {
		return endGroups()
	}
	name := ""
	if eu := u.EffectiveUser(); eu != nil {
		name = eu.FirstName
	}
	replyKb(ctx, u, homeKeyboard(), homeText(name)...)
	return endGroups()
}
