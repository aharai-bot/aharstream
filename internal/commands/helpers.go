package commands

import (
	"strconv"
	"strings"
	"time"

	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/store"
	"EverythingSuckz/fsb/internal/utils"

	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/dispatcher"
	"github.com/celestix/gotgproto/ext"
	"github.com/celestix/gotgproto/storage"
	"github.com/gotd/td/telegram/message/entity"
	"github.com/gotd/td/telegram/message/styling"
	"github.com/gotd/td/tg"
)

// StartTime is the process start time (used by /ping and /status).
var StartTime = time.Now()

// Client is the main bot client. It is set by the bot package before handlers load.
var Client *gotgproto.Client

// WorkersInfo returns one line per worker bot. Set from main to avoid an import cycle.
var WorkersInfo func() []string

// ---------- styled text shortcuts ----------

type part = styling.StyledTextOption

func p(s string) part        { return styling.Plain(s) }
func b(s string) part        { return styling.Bold(s) }
func i(s string) part        { return styling.Italic(s) }
func c(s string) part        { return styling.Code(s) }
func nl() part               { return styling.Plain("\n") }
func parts(x ...part) []part { return x }

func build(ps []part) (string, []tg.MessageEntityClass, error) {
	tb := entity.Builder{}
	if err := styling.Perform(&tb, ps...); err != nil {
		return "", nil, err
	}
	text, ents := tb.Complete()
	return text, ents, nil
}

func reply(ctx *ext.Context, u *ext.Update, ps ...part) {
	_, _ = ctx.Reply(u, ext.ReplyTextStyledTextArray(ps), &ext.ReplyOpts{NoWebpage: true})
}

func replyKb(ctx *ext.Context, u *ext.Update, kb tg.ReplyMarkupClass, ps ...part) {
	_, _ = ctx.Reply(u, ext.ReplyTextStyledTextArray(ps), &ext.ReplyOpts{NoWebpage: true, Markup: kb})
}

func btnCB(text, data string) tg.KeyboardButtonClass {
	return &tg.KeyboardButtonCallback{Text: text, Data: []byte(data)}
}

func btnURL(text, url string) tg.KeyboardButtonClass {
	return &tg.KeyboardButtonURL{Text: text, URL: url}
}

func kb(rows ...[]tg.KeyboardButtonClass) *tg.ReplyInlineMarkup {
	m := &tg.ReplyInlineMarkup{}
	for _, r := range rows {
		m.Rows = append(m.Rows, tg.KeyboardButtonRow{Buttons: r})
	}
	return m
}

// ---------- permissions ----------

// IsOwner is true for IDs listed in the ADMINS env variable.
func IsOwner(id int64) bool { return utils.Contains(config.ValueOf.Admins, id) }

// IsAdmin is true for owners and admins added with /addadmin.
func IsAdmin(id int64) bool { return IsOwner(id) || store.Default.IsStoredAdmin(id) }

// ---------- gate: runs at the start of every user-facing handler ----------

// userID returns the private-chat user id of the update, and false when it is
// not a private chat (the bot only works in DMs).
func userID(ctx *ext.Context, u *ext.Update) (int64, bool) {
	chat := u.EffectiveChat()
	if chat == nil {
		return 0, false
	}
	id := chat.GetID()
	peer := ctx.PeerStorage.GetPeerById(id)
	if peer == nil || peer.Type != int(storage.TypeUser) {
		return 0, false
	}
	return id, true
}

// gate registers the user and enforces allow-list, bans and maintenance mode.
// It returns the user id and whether processing may continue.
func gate(ctx *ext.Context, u *ext.Update) (int64, bool) {
	id, ok := userID(ctx, u)
	if !ok {
		return 0, false
	}
	var username, first string
	if eu := u.EffectiveUser(); eu != nil {
		username, first = eu.Username, eu.FirstName
	}
	store.Default.Touch(id, username, first)

	if len(config.ValueOf.AllowedUsers) != 0 && !utils.Contains(config.ValueOf.AllowedUsers, id) && !IsAdmin(id) {
		reply(ctx, u, p("🚫 You are not allowed to use this bot."))
		return id, false
	}
	if store.Default.IsBanned(id) {
		usr, _ := store.Default.GetUser(id)
		msg := "🚫 You have been banned from using this bot."
		if usr.BanReason != "" {
			msg += "\nReason: " + usr.BanReason
		}
		reply(ctx, u, p(msg))
		return id, false
	}
	if store.Default.Maintenance() && !IsAdmin(id) {
		reply(ctx, u, p("🛠 "+config.ValueOf.BotName+" is under maintenance right now. Please try again later."))
		return id, false
	}
	return id, true
}

// adminGate is gate + admin check. Non-admins get no hint that admin commands exist.
func adminGate(ctx *ext.Context, u *ext.Update) (int64, bool) {
	id, ok := gate(ctx, u)
	if !ok {
		return 0, false
	}
	if !IsAdmin(id) {
		reply(ctx, u, p("⛔ This command is for admins only."))
		return id, false
	}
	return id, true
}

func ownerGate(ctx *ext.Context, u *ext.Update) (int64, bool) {
	id, ok := gate(ctx, u)
	if !ok {
		return 0, false
	}
	if !IsOwner(id) {
		reply(ctx, u, p("⛔ This command is for the bot owner only."))
		return id, false
	}
	return id, true
}

// ---------- arg helpers ----------

// argsAfterCommand returns the message text after the command word.
func argsAfterCommand(u *ext.Update) string {
	t := strings.TrimSpace(u.EffectiveMessage.Text)
	if idx := strings.IndexAny(t, " \n"); idx >= 0 {
		return strings.TrimSpace(t[idx+1:])
	}
	return ""
}

func argInt64(u *ext.Update, n int) (int64, bool) {
	a := u.Args()
	if len(a) <= n {
		return 0, false
	}
	v, err := strconv.ParseInt(strings.TrimPrefix(a[n], "@"), 10, 64)
	return v, err == nil
}

// repliedMessage fetches the message the user replied to (if any).
func repliedMessage(ctx *ext.Context, u *ext.Update, chatID int64) *tg.Message {
	hdr, ok := u.EffectiveMessage.ReplyTo.(*tg.MessageReplyHeader)
	if !ok || hdr.ReplyToMsgID == 0 {
		return nil
	}
	msgs, err := ctx.GetMessages(chatID, []tg.InputMessageClass{&tg.InputMessageID{ID: hdr.ReplyToMsgID}})
	if err != nil || len(msgs) == 0 {
		return nil
	}
	m, _ := msgs[0].(*tg.Message)
	return m
}

func endGroups() error { return dispatcher.EndGroups }
