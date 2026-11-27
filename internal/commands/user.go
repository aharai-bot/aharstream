package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/i18n"
	"EverythingSuckz/fsb/internal/store"
	"EverythingSuckz/fsb/internal/utils"

	"github.com/celestix/gotgproto/dispatcher"
	"github.com/celestix/gotgproto/dispatcher/handlers"
	"github.com/celestix/gotgproto/dispatcher/handlers/filters"
	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
)

func (m *command) LoadUser(d dispatcher.Dispatcher) {
	log := m.log.Named("user")
	defer log.Sugar().Info("Loaded")
	d.AddHandler(handlers.NewCommand("myfiles", myFiles))
	d.AddHandler(handlers.NewCommand("mystats", myStats))
	d.AddHandler(handlers.NewCommand("expiry", expiry))
	d.AddHandler(handlers.NewCommand("revoke", revoke))
	d.AddHandler(handlers.NewCommand("info", info))
	d.AddHandler(handlers.NewCommand("link", relink))
}

func fileStatus(rec store.FileRecord) string {
	switch {
	case rec.Revoked:
		return "🗑 revoked"
	case rec.ExpiresAt != 0 && time.Now().Unix() > rec.ExpiresAt:
		return "⌛ expired"
	default:
		return "🟢 active"
	}
}

func myFilesText(userID int64) []part {
	lang := userLang(userID)
	files := store.Default.FilesOf(userID, 10)
	if len(files) == 0 {
		return parts(b(i18n.T(lang, "files_title")), nl(), nl(), p(i18n.T(lang, "files_empty")))
	}
	ps := parts(b(i18n.T(lang, "files_title")), nl(), nl())
	for idx, f := range files {
		ps = append(ps,
			b(fmt.Sprintf("%d. ", idx+1)), p(utils.ShortName(f.Name, 40)), nl(),
			p("   "+utils.HumanSize(f.Size)+" • ⬇️ "+strconv.FormatInt(f.Downloads, 10)+" • "+fileStatus(f)+" • ID "), c(fmt.Sprint(f.MessageID)), nl(),
		)
	}
	ps = append(ps, nl(), i("/revoke <id> · "+i18n.T(lang, "files_hint")))
	return ps
}

func myFiles(ctx *ext.Context, u *ext.Update) error {
	id, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	reply(ctx, u, myFilesText(id)...)
	return endGroups()
}

func myStats(ctx *ext.Context, u *ext.Update) error {
	id, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	usr, _ := store.Default.GetUser(id)
	var downloads, bytes int64
	var active int
	all := store.Default.FilesOf(id, 0)
	for _, f := range all {
		downloads += f.Downloads
		bytes += f.Bytes
		if !f.Revoked && (f.ExpiresAt == 0 || f.ExpiresAt > time.Now().Unix()) {
			active++
		}
	}
	exp := "server default"
	switch {
	case usr.ExpiryHours > 0:
		exp = fmt.Sprintf("%d hours", usr.ExpiryHours)
	case usr.ExpiryHours < 0:
		exp = "never"
	case config.ValueOf.DefaultExpiryHours > 0:
		exp = fmt.Sprintf("%d hours (server default)", config.ValueOf.DefaultExpiryHours)
	default:
		exp = "never (server default)"
	}
	role := "User"
	if IsOwner(id) {
		role = "Owner 👑"
	} else if IsAdmin(id) {
		role = "Admin 🛡"
	}
	reply(ctx, u,
		b("📊 Your statistics"), nl(), nl(),
		b("Role: "), p(role), nl(),
		b("Member since: "), p(utils.FormatUnix(usr.JoinedAt)), nl(),
		b("Links created: "), c(fmt.Sprint(len(all))), p(" ("), c(fmt.Sprint(active)), p(" active)"), nl(),
		b("Total downloads: "), c(fmt.Sprint(downloads)), nl(),
		b("Data served: "), c(utils.HumanSize(bytes)), nl(),
		b("Link lifetime: "), p(exp),
	)
	return endGroups()
}

// parseHours understands "24", "24h", "7d", "2w".
func parseHours(s string) (int, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, false
	}
	mult := 1
	switch s[len(s)-1] {
	case 'h':
		s = s[:len(s)-1]
	case 'd':
		mult, s = 24, s[:len(s)-1]
	case 'w':
		mult, s = 24*7, s[:len(s)-1]
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, false
	}
	n *= mult
	if n > 24*365 {
		return 0, false
	}
	return n, true
}

func expiry(ctx *ext.Context, u *ext.Update) error {
	id, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	arg := strings.ToLower(argsAfterCommand(u))
	switch arg {
	case "":
		usr, _ := store.Default.GetUser(id)
		cur := "server default"
		if usr.ExpiryHours > 0 {
			cur = fmt.Sprintf("%d hours", usr.ExpiryHours)
		} else if usr.ExpiryHours < 0 {
			cur = "never"
		}
		reply(ctx, u,
			b("⏳ Link lifetime"), nl(),
			p("Current: "), c(cur), nl(), nl(),
			p("Set how long your new links stay alive:"), nl(),
			c("/expiry 24h"), p(" – 24 hours"), nl(),
			c("/expiry 7d"), p(" – 7 days"), nl(),
			c("/expiry off"), p(" – never expire"), nl(),
			c("/expiry default"), p(" – use server default"),
		)
	case "off", "never", "0":
		store.Default.SetExpiry(id, -1)
		reply(ctx, u, p("✅ New links will never expire."))
	case "default", "reset":
		store.Default.SetExpiry(id, 0)
		reply(ctx, u, p("✅ Link lifetime reset to the server default."))
	default:
		h, ok := parseHours(arg)
		if !ok {
			reply(ctx, u, p("⚠️ Invalid value. Examples: /expiry 12h, /expiry 7d, /expiry off (max 365d)."))
			return endGroups()
		}
		store.Default.SetExpiry(id, h)
		reply(ctx, u, p(fmt.Sprintf("✅ New links will expire after %d hour(s). Existing links are not changed.", h)))
	}
	return endGroups()
}

func revoke(ctx *ext.Context, u *ext.Update) error {
	id, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	mid, ok := argInt64(u, 1)
	if !ok {
		reply(ctx, u, p("Usage: /revoke <link id>\nYou can find IDs in /myfiles."))
		return endGroups()
	}
	rec, found := store.Default.GetFile(int(mid))
	if !found {
		reply(ctx, u, p("⚠️ No link with that ID."))
		return endGroups()
	}
	if rec.OwnerID != id && !IsAdmin(id) {
		reply(ctx, u, p("⛔ That link doesn't belong to you."))
		return endGroups()
	}
	store.Default.RevokeFile(int(mid), false)
	reply(ctx, u, p("🗑 Link "+fmt.Sprint(mid)+" has been revoked. It no longer works."))
	return endGroups()
}

func info(ctx *ext.Context, u *ext.Update) error {
	id, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	rm := repliedMessage(ctx, u, id)
	if rm == nil || rm.Media == nil {
		reply(ctx, u, p("ℹ️ Reply to a file with /info to see its details."))
		return endGroups()
	}
	f, err := utils.FileFromMedia(rm.Media)
	if err != nil {
		reply(ctx, u, p("⚠️ "+err.Error()))
		return endGroups()
	}
	size := utils.HumanSize(f.FileSize)
	if f.FileSize == 0 {
		size = "photo"
	}
	reply(ctx, u,
		b("🔎 File details"), nl(), nl(),
		b("Name: "), p(utils.DisplayName(f.FileName, f.MimeType, f.ID)), nl(),
		b("Type: "), p(utils.FileKind(f.MimeType)), nl(),
		b("MIME: "), c(f.MimeType), nl(),
		b("Size: "), p(size), nl(),
		b("File ID: "), c(fmt.Sprint(f.ID)),
	)
	return endGroups()
}

func relink(ctx *ext.Context, u *ext.Update) error {
	id, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	rm := repliedMessage(ctx, u, id)
	if rm == nil || rm.Media == nil {
		reply(ctx, u, p("🔗 Reply to a file with /link to generate a fresh link for it."))
		return endGroups()
	}
	rec, err := createLink(ctx, id, rm.ID, rm.Media)
	if err != nil {
		if le, ok := err.(*linkError); ok {
			reply(ctx, u, p("⚠️ "+le.msg))
		} else {
			reply(ctx, u, p("❌ Error - "+err.Error()))
		}
		return endGroups()
	}
	text, markup := linkMessage(rec, userLang(id))
	_, _ = ctx.Reply(u, ext.ReplyTextStyledTextArray(text), &ext.ReplyOpts{Markup: markup, NoWebpage: true, ReplyToMessageId: rm.ID})
	return endGroups()
}

// ---------- inline button callbacks ----------

func (m *command) LoadCallbacks(d dispatcher.Dispatcher) {
	log := m.log.Named("callbacks")
	defer log.Sugar().Info("Loaded")
	d.AddHandler(handlers.NewCallbackQuery(filters.CallbackQuery.Prefix("ahar:"), onCallback))
}

func answerCB(ctx *ext.Context, cb *tg.UpdateBotCallbackQuery, text string, alert bool) {
	_, _ = ctx.AnswerCallback(&tg.MessagesSetBotCallbackAnswerRequest{QueryID: cb.QueryID, Message: text, Alert: alert})
}

func onCallback(ctx *ext.Context, u *ext.Update) error {
	cb := u.CallbackQuery
	if cb == nil {
		return nil
	}
	uid := cb.UserID
	if store.Default.IsBanned(uid) {
		answerCB(ctx, cb, "You are banned.", true)
		return endGroups()
	}
	data := string(cb.Data)
	answered := false
	lang := userLang(uid)
	back := kb([]tg.KeyboardButtonClass{btnCB(i18n.T(lang, "btn_home"), "ahar:home")})

	switch {
	case data == "ahar:home":
		name := ""
		if eu := u.EffectiveUser(); eu != nil {
			name = eu.FirstName
		}
		editText(ctx, uid, cb.MsgID, homeText(name, lang), homeKeyboard(lang))
	case data == "ahar:help":
		editText(ctx, uid, cb.MsgID, helpText(IsAdmin(uid), IsOwner(uid)), back)
	case data == "ahar:about":
		editText(ctx, uid, cb.MsgID, aboutText(), back)
	case data == "ahar:files":
		editText(ctx, uid, cb.MsgID, myFilesText(uid), back)
	case strings.HasPrefix(data, "ahar:rv:"):
		mid, err := strconv.Atoi(strings.TrimPrefix(data, "ahar:rv:"))
		if err != nil {
			answerCB(ctx, cb, "Bad request", true)
			return endGroups()
		}
		rec, ok := store.Default.GetFile(mid)
		if !ok {
			answerCB(ctx, cb, "Link not found", true)
			return endGroups()
		}
		if rec.OwnerID != uid && !IsAdmin(uid) {
			answerCB(ctx, cb, "That link isn't yours.", true)
			return endGroups()
		}
		store.Default.RevokeFile(mid, false)
		answerCB(ctx, cb, "Link revoked ✅", false)
		answered = true
		editText(ctx, uid, cb.MsgID, parts(b("🗑 Link revoked"), nl(), p(utils.ShortName(rec.Name, 60)+" can no longer be accessed.")), back)
	case data == "ahar:langmenu":
		editText(ctx, uid, cb.MsgID, parts(b(i18n.T(lang, "lang_choose"))), langKeyboard())
	case strings.HasPrefix(data, "ahar:lang:"):
		code := strings.TrimPrefix(data, "ahar:lang:")
		if i18n.Valid(code) {
			store.Default.SetLang(uid, code)
			lang = code
			name := ""
			if eu := u.EffectiveUser(); eu != nil {
				name = eu.FirstName
			}
			editText(ctx, uid, cb.MsgID, homeText(name, lang), homeKeyboard(lang))
			answerCB(ctx, cb, i18n.T(lang, "lang_set", i18n.Name(lang)), false)
			answered = true
		}
	case strings.HasPrefix(data, "ahar:ex:"), strings.HasPrefix(data, "ahar:vw:"), strings.HasPrefix(data, "ahar:pw:"):
		if linkSettingCallback(ctx, cb, uid, lang, data) {
			answered = true
		}
	case strings.HasPrefix(data, "ahar:ch:"):
		if channelDecision(ctx, cb, uid, data) {
			answered = true
		}
	case strings.HasPrefix(data, "ahar:adm"):
		if !IsAdmin(uid) {
			answerCB(ctx, cb, "Admins only.", true)
			return endGroups()
		}
		panelBack := kb([]tg.KeyboardButtonClass{btnCB("⬅️ Admin panel", "ahar:adm")})
		switch data {
		case "ahar:adm":
			editText(ctx, uid, cb.MsgID, adminPanelText(), adminPanelKeyboard())
		case "ahar:adm:status":
			editText(ctx, uid, cb.MsgID, statusParts(), panelBack)
		case "ahar:adm:users":
			editText(ctx, uid, cb.MsgID, usersParts(), panelBack)
		case "ahar:adm:top":
			editText(ctx, uid, cb.MsgID, topFilesParts(), panelBack)
		}
	}
	if !answered {
		answerCB(ctx, cb, "", false)
	}
	return endGroups()
}
