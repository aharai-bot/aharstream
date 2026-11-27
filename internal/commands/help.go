package commands

import (
	"fmt"
	"runtime"
	"time"

	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/utils"

	"github.com/celestix/gotgproto/dispatcher"
	"github.com/celestix/gotgproto/dispatcher/handlers"
	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
)

func (m *command) LoadHelp(d dispatcher.Dispatcher) {
	log := m.log.Named("help")
	defer log.Sugar().Info("Loaded")
	d.AddHandler(handlers.NewCommand("help", help))
	d.AddHandler(handlers.NewCommand("about", about))
	d.AddHandler(handlers.NewCommand("ping", ping))
}

func helpText(admin, owner bool) []part {
	ps := parts(
		b("📖 "+config.ValueOf.BotName+" — Help"), nl(), nl(),
		b("How it works"), nl(),
		p("Send any file and get a direct link. Tap Download, Stream or Open in Web Player."), nl(), nl(),
		b("👤 User commands"), nl(),
		c("/start"), p(" – welcome screen"), nl(),
		c("/myfiles"), p(" – your 10 latest links"), nl(),
		c("/mystats"), p(" – your usage statistics"), nl(),
		c("/link"), p(" – reply to a file to (re)generate a link"), nl(),
		c("/info"), p(" – reply to a file to see its details"), nl(),
		c("/expiry"), p(" <24h | 7d | off> – default link lifetime"), nl(),
		c("/revoke"), p(" <id> – disable a link"), nl(),
		c("/ping"), p(" – latency & uptime"), nl(),
		c("/about"), p(" – about this bot"), nl(),
		nl(), b("✨ Links & app"), nl(),
		c("/app"), p(" – open My Files (Mini App) · "), c("/login"), p(" – sign in from a browser"), nl(),
		c("/password"), p(" <id> <pass|off> – protect a link"), nl(),
		c("/views"), p(" <id> <n|off> – limit opens (1 = one-time link)"), nl(),
		c("/linkexpiry"), p(" <id> <24h|7d|off> – expiry of one link"), nl(),
		c("/storage"), p(" – your storage summary · "), c("/collections"), p(" – shareable collections"), nl(),
		c("/language"), p(" – change the bot language"), nl(),
		nl(), b("🧩 Developers"), nl(),
		c("/apikey"), p(" [new <name> | revoke <id>] – API keys"), nl(),
		c("/webhook"), p(" <https url|off|test> – event webhook"), nl(),
		i("Inline mode: type @"+"yourbot in any chat. Channels: post /watchbuttons in your channel."), nl(),
	)
	if admin {
		ps = append(ps, nl(),
			b("🛡 Admin commands"), nl(),
			c("/admin"), p(" – admin panel"), nl(),
			c("/status"), p(" – system & usage status"), nl(),
			c("/users"), p(" – user counts & recent users"), nl(),
			c("/user"), p(" <id> – details about a user"), nl(),
			c("/ban"), p(" <id> [reason] / "), c("/unban"), p(" <id>"), nl(),
			c("/banned"), p(" – list banned users"), nl(),
			c("/broadcast"), p(" <text> – message all users (or reply to a text)"), nl(),
			c("/maintenance"), p(" <on|off> – pause the bot for normal users"), nl(),
			c("/topfiles"), p(" – most downloaded files"), nl(),
			c("/revoke"), p(" <id> / "), c("/restore"), p(" <id> – disable or re-enable any link"), nl(),
			c("/workers"), p(" – list streaming bots"), nl(),
			c("/purge"), p(" – clean expired link records"), nl(),
			c("/admins"), p(" – list admins"), nl(),
		)
		if owner {
			ps = append(ps, nl(), i("👑 Owner only: /addadmin <id>, /rmadmin <id>"), nl())
		}
	}
	return ps
}

func aboutText() []part {
	return parts(
		b("ℹ️ "+config.ValueOf.BotName), nl(), nl(),
		p("Turn Telegram files into fast direct links with seeking support, a web player, expiring links and detailed stats."), nl(), nl(),
		b("Version: "), c(utils.Version), nl(),
		b("Runtime: "), c(runtime.Version()), nl(),
		b("Uptime: "), c(utils.TimeFormat(uint64(time.Since(StartTime).Seconds()))), nl(), nl(),
		i("Remodified by Ahar Developers. Built on TG-FileStreamBot by EverythingSuckz (AGPL-3.0)."),
	)
}

func help(ctx *ext.Context, u *ext.Update) error {
	id, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	replyKb(ctx, u, kb([]tg.KeyboardButtonClass{btnCB("🏠 Home", "ahar:home")}), helpText(IsAdmin(id), IsOwner(id))...)
	return endGroups()
}

func about(ctx *ext.Context, u *ext.Update) error {
	if _, ok := gate(ctx, u); !ok {
		return endGroups()
	}
	replyKb(ctx, u, kb([]tg.KeyboardButtonClass{btnCB("🏠 Home", "ahar:home")}), aboutText()...)
	return endGroups()
}

func ping(ctx *ext.Context, u *ext.Update) error {
	id, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	start := time.Now()
	sent, err := ctx.Reply(u, ext.ReplyTextString("🏓 Pong!"), nil)
	if err != nil || sent == nil {
		return endGroups()
	}
	latency := time.Since(start)
	editText(ctx, id, sent.ID, parts(
		b("🏓 Pong!"), nl(),
		b("Latency: "), c(fmt.Sprintf("%d ms", latency.Milliseconds())), nl(),
		b("Uptime: "), c(utils.TimeFormat(uint64(time.Since(StartTime).Seconds()))),
	), nil)
	return endGroups()
}

// editText edits a bot message in a private chat with styled text.
func editText(ctx *ext.Context, chatID int64, msgID int, ps []part, markup tg.ReplyMarkupClass) {
	text, ents, err := build(ps)
	if err != nil {
		return
	}
	req := &tg.MessagesEditMessageRequest{ID: msgID, Message: text, Entities: ents, NoWebpage: true}
	if markup != nil {
		req.ReplyMarkup = markup
	}
	_, _ = ctx.EditMessage(chatID, req)
}
