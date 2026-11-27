package commands

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/auth"
	"EverythingSuckz/fsb/internal/i18n"
	"EverythingSuckz/fsb/internal/store"
	"EverythingSuckz/fsb/internal/utils"
	"EverythingSuckz/fsb/internal/webhook"

	"github.com/celestix/gotgproto/dispatcher"
	"github.com/celestix/gotgproto/dispatcher/handlers"
	"github.com/celestix/gotgproto/dispatcher/handlers/filters"
	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
)

// LoadFeatures registers the Telegram-side features: language, sign-in, link
// settings, collections, API keys, webhooks, inline mode and channel mode.
func (m *command) LoadFeatures(d dispatcher.Dispatcher) {
	log := m.log.Named("features")
	defer log.Sugar().Info("Loaded")
	d.AddHandler(handlers.NewCommand("language", cmdLanguage))
	d.AddHandler(handlers.NewCommand("login", cmdLogin))
	d.AddHandler(handlers.NewCommand("app", cmdApp))
	d.AddHandler(handlers.NewCommand("password", cmdPassword))
	d.AddHandler(handlers.NewCommand("views", cmdViews))
	d.AddHandler(handlers.NewCommand("linkexpiry", cmdLinkExpiry))
	d.AddHandler(handlers.NewCommand("storage", cmdStorage))
	d.AddHandler(handlers.NewCommand("collections", cmdCollections))
	d.AddHandler(handlers.NewCommand("apikey", cmdAPIKey))
	d.AddHandler(handlers.NewCommand("webhook", cmdWebhook))
	d.AddHandler(handlers.NewCommand("cancel", cmdCancel))
	d.AddHandler(handlers.NewInlineQuery(filters.InlineQuery.All, onInline))
	// Group 5: after commands (group 0) but before the catch-all media handler (group 10).
	d.AddHandlerToGroup(handlers.NewMessage(nil, onChannelPost), 5)
}

// ---------- shared helpers ----------

// UserAllowed applies the allow-list, ban list and maintenance mode. It is used
// by the web app and API so they follow the same rules as the bot.
func UserAllowed(id int64) error {
	if len(config.ValueOf.AllowedUsers) != 0 && !utils.Contains(config.ValueOf.AllowedUsers, id) && !IsAdmin(id) {
		return errors.New("you are not allowed to use this service")
	}
	if store.Default.IsBanned(id) {
		return errors.New("you have been banned")
	}
	if store.Default.Maintenance() && !IsAdmin(id) {
		return errors.New("the service is under maintenance")
	}
	return nil
}

// CreateLinkFromMessage makes a link for a message the user sent to the bot
// (used by POST /api/v1/links).
func CreateLinkFromMessage(userID int64, msgID int, opts LinkOptions) (*store.FileRecord, error) {
	if Client == nil {
		return nil, errors.New("the bot is not ready yet")
	}
	if err := UserAllowed(userID); err != nil {
		return nil, &linkError{err.Error()}
	}
	ctx := Client.CreateContext()
	msgs, err := ctx.GetMessages(userID, []tg.InputMessageClass{&tg.InputMessageID{ID: msgID}})
	if err != nil || len(msgs) == 0 {
		return nil, &linkError{"message not found in your chat with the bot"}
	}
	m, ok := msgs[0].(*tg.Message)
	if !ok || m.Media == nil {
		return nil, &linkError{"that message has no file"}
	}
	return createLinkFrom(ctx, userID, userID, m.ID, m.Media, opts)
}

// sendStyled sends a styled message to a chat (works from callback handlers too).
func sendStyled(ctx *ext.Context, chatID int64, ps []part, markup tg.ReplyMarkupClass) {
	text, ents, err := build(ps)
	if err != nil {
		return
	}
	req := &tg.MessagesSendMessageRequest{Message: text, Entities: ents, NoWebpage: true}
	if markup != nil {
		req.ReplyMarkup = markup
	}
	_, _ = ctx.SendMessage(chatID, req)
}

func langKeyboard() *tg.ReplyInlineMarkup {
	var rows [][]tg.KeyboardButtonClass
	var row []tg.KeyboardButtonClass
	for _, l := range i18n.Langs {
		row = append(row, btnCB(l.Name, "ahar:lang:"+l.Code))
		if len(row) == 2 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	return kb(rows...)
}

// ownedLink loads a link and checks that the user may change it.
func ownedLink(uid int64, id int, lang string) (store.FileRecord, string) {
	rec, ok := store.Default.GetFile(id)
	if !ok {
		return rec, i18n.T(lang, "not_found")
	}
	if rec.OwnerID != uid && !IsAdmin(uid) {
		return rec, i18n.T(lang, "not_yours")
	}
	return rec, ""
}

func baseURL() string { return strings.TrimRight(config.ValueOf.Host, "/") }

func isLocalhost() bool { return strings.Contains(config.ValueOf.Host, "localhost") }

// ---------- pending "send the password next" state ----------

type pendingAction struct {
	linkID  int
	linkMsg int // id of the bot message that shows the link, so it can be refreshed
	exp     time.Time
}

var (
	pendMu sync.Mutex
	pend   = map[int64]pendingAction{}
)

func setPending(uid int64, linkID, linkMsg int) {
	pendMu.Lock()
	defer pendMu.Unlock()
	pend[uid] = pendingAction{linkID: linkID, linkMsg: linkMsg, exp: time.Now().Add(5 * time.Minute)}
}

func clearPending(uid int64) bool {
	pendMu.Lock()
	defer pendMu.Unlock()
	_, ok := pend[uid]
	delete(pend, uid)
	return ok
}

func takePending(uid int64) (pendingAction, bool) {
	pendMu.Lock()
	defer pendMu.Unlock()
	pa, ok := pend[uid]
	delete(pend, uid)
	if !ok || time.Now().After(pa.exp) {
		return pendingAction{}, false
	}
	return pa, true
}

// handlePendingText consumes the next text message as a link password.
func handlePendingText(ctx *ext.Context, u *ext.Update, uid int64, lang string) bool {
	pa, ok := takePending(uid)
	if !ok {
		return false
	}
	pw := strings.TrimSpace(u.EffectiveMessage.Text)
	_ = ctx.DeleteMessages(uid, []int{u.EffectiveMessage.ID}) // keep the password out of the chat
	res, rec := applyPassword(uid, pa.linkID, pw, lang)
	reply(ctx, u, p(res))
	if rec != nil && pa.linkMsg != 0 {
		text, markup := linkMessage(rec, lang)
		editText(ctx, uid, pa.linkMsg, text, markup)
	}
	return true
}

// applyPassword sets (or with "off" clears) a link password.
func applyPassword(uid int64, id int, pw, lang string) (string, *store.FileRecord) {
	if _, bad := ownedLink(uid, id, lang); bad != "" {
		return bad, nil
	}
	if strings.EqualFold(pw, "off") {
		store.Default.UpdateFile(id, func(f *store.FileRecord) { f.PassHash = "" })
		rec, _ := store.Default.GetFile(id)
		return i18n.T(lang, "pw_cleared"), &rec
	}
	if !config.ValueOf.EnablePlayer {
		return i18n.T(lang, "pw_need_web"), nil
	}
	if len([]rune(pw)) < 4 {
		return i18n.T(lang, "pw_short"), nil
	}
	if len(pw) > 100 {
		return "⚠️ Password is too long.", nil
	}
	h, err := auth.HashPassword(pw)
	if err != nil {
		return "❌ " + err.Error(), nil
	}
	store.Default.UpdateFile(id, func(f *store.FileRecord) { f.PassHash = h })
	rec, _ := store.Default.GetFile(id)
	return i18n.T(lang, "pw_set"), &rec
}

// ---------- per-link settings (buttons under a fresh link) ----------

// linkSettingCallback handles ahar:ex / ahar:vw / ahar:pw. It returns true when
// it already answered the callback.
func linkSettingCallback(ctx *ext.Context, cb *tg.UpdateBotCallbackQuery, uid int64, lang, data string) bool {
	seg := strings.Split(data, ":") // ahar, kind, id[, value]
	if len(seg) < 3 {
		answerCB(ctx, cb, "Bad request", true)
		return true
	}
	id, err := strconv.Atoi(seg[2])
	if err != nil {
		answerCB(ctx, cb, "Bad request", true)
		return true
	}
	if _, bad := ownedLink(uid, id, lang); bad != "" {
		answerCB(ctx, cb, bad, true)
		return true
	}
	toast := ""
	switch seg[1] {
	case "ex":
		if len(seg) < 4 {
			return false
		}
		h, err := strconv.Atoi(seg[3])
		if err != nil {
			return false
		}
		var exp int64
		if h > 0 {
			exp = time.Now().Add(time.Duration(h) * time.Hour).Unix()
		}
		store.Default.UpdateFile(id, func(f *store.FileRecord) { f.ExpiresAt = exp })
		label := i18n.T(lang, "never")
		if exp != 0 {
			label = utils.TimeLeft(exp)
		}
		toast = i18n.T(lang, "exp_set", label)
	case "vw":
		if len(seg) < 4 {
			return false
		}
		n, err := strconv.Atoi(seg[3])
		if err != nil || n < 0 {
			return false
		}
		store.Default.UpdateFile(id, func(f *store.FileRecord) {
			f.MaxViews = n
			if n > 0 {
				f.Views = 0 // a new limit starts from zero
			}
		})
		label := i18n.T(lang, "unlimited")
		if n > 0 {
			label = strconv.Itoa(n)
		}
		toast = i18n.T(lang, "views_set", label)
	case "pw":
		if !config.ValueOf.EnablePlayer {
			answerCB(ctx, cb, i18n.T(lang, "pw_need_web"), true)
			return true
		}
		setPending(uid, id, cb.MsgID)
		answerCB(ctx, cb, "🔒", false)
		sendStyled(ctx, uid, parts2(p(i18n.T(lang, "pw_prompt"))), nil)
		return true
	default:
		return false
	}
	rec, _ := store.Default.GetFile(id)
	text, markup := linkMessage(&rec, lang)
	editText(ctx, uid, cb.MsgID, text, markup)
	answerCB(ctx, cb, toast, false)
	return true
}

// parts2 is a readable alias for building a []part from options.
func parts2(x ...part) []part { return x }

// ---------- commands ----------

func cmdLanguage(ctx *ext.Context, u *ext.Update) error {
	id, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	replyKb(ctx, u, langKeyboard(), b(i18n.T(userLang(id), "lang_choose")))
	return endGroups()
}

func cmdCancel(ctx *ext.Context, u *ext.Update) error {
	id, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	if clearPending(id) {
		reply(ctx, u, p(i18n.T(userLang(id), "cancelled")))
	}
	return endGroups()
}

func cmdLogin(ctx *ext.Context, u *ext.Update) error {
	id, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	lang := userLang(id)
	link := baseURL() + "/auth/login?t=" + auth.NewLoginToken(id)
	if isLocalhost() || config.ValueOf.Host == "" {
		reply(ctx, u, p(i18n.T(lang, "login_msg")), nl(), nl(), c(link))
		return endGroups()
	}
	replyKb(ctx, u, kb([]tg.KeyboardButtonClass{btnURL(i18n.T(lang, "login_btn"), link)}), p(i18n.T(lang, "login_msg")))
	return endGroups()
}

func cmdApp(ctx *ext.Context, u *ext.Update) error {
	id, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	lang := userLang(id)
	if isHTTPS() {
		replyKb(ctx, u, kb([]tg.KeyboardButtonClass{&tg.KeyboardButtonWebView{Text: i18n.T(lang, "btn_openapp"), URL: baseURL() + "/app"}}), p(i18n.T(lang, "app_msg")))
		return endGroups()
	}
	reply(ctx, u, p(i18n.T(lang, "app_msg")), nl(), nl(), c(baseURL()+"/app"), nl(), i("HTTPS is needed for the in-Telegram app. Use /login to sign in from a browser."))
	return endGroups()
}

func cmdPassword(ctx *ext.Context, u *ext.Update) error {
	uid, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	lang := userLang(uid)
	args := u.Args()
	_ = ctx.DeleteMessages(uid, []int{u.EffectiveMessage.ID}) // never leave passwords in the chat
	if len(args) < 3 {
		sendStyled(ctx, uid, parts2(p("Usage: "), c("/password <link id> <password>"), p(" or "), c("/password <link id> off")), nil)
		return endGroups()
	}
	id, err := strconv.Atoi(args[1])
	if err != nil {
		sendStyled(ctx, uid, parts2(p(i18n.T(lang, "not_found"))), nil)
		return endGroups()
	}
	res, _ := applyPassword(uid, id, strings.Join(args[2:], " "), lang)
	sendStyled(ctx, uid, parts2(p(res)), nil)
	return endGroups()
}

func cmdViews(ctx *ext.Context, u *ext.Update) error {
	uid, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	lang := userLang(uid)
	args := u.Args()
	if len(args) < 3 {
		reply(ctx, u, p("Usage: "), c("/views <link id> <number|off>"), p("\n1 = one-time link, off = unlimited."))
		return endGroups()
	}
	id, err := strconv.Atoi(args[1])
	if err != nil {
		reply(ctx, u, p(i18n.T(lang, "not_found")))
		return endGroups()
	}
	if _, bad := ownedLink(uid, id, lang); bad != "" {
		reply(ctx, u, p(bad))
		return endGroups()
	}
	n := 0
	if !strings.EqualFold(args[2], "off") {
		n, err = strconv.Atoi(args[2])
		if err != nil || n < 1 || n > 1000000 {
			reply(ctx, u, p("⚠️ Use a number from 1 to 1000000, or off."))
			return endGroups()
		}
	}
	store.Default.UpdateFile(id, func(f *store.FileRecord) {
		f.MaxViews = n
		if n > 0 {
			f.Views = 0
		}
	})
	label := i18n.T(lang, "unlimited")
	if n > 0 {
		label = strconv.Itoa(n)
	}
	reply(ctx, u, p(i18n.T(lang, "views_set", label)))
	return endGroups()
}

func cmdLinkExpiry(ctx *ext.Context, u *ext.Update) error {
	uid, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	lang := userLang(uid)
	args := u.Args()
	if len(args) < 3 {
		reply(ctx, u, p("Usage: "), c("/linkexpiry <link id> <24h|7d|off>"))
		return endGroups()
	}
	id, err := strconv.Atoi(args[1])
	if err != nil {
		reply(ctx, u, p(i18n.T(lang, "not_found")))
		return endGroups()
	}
	if _, bad := ownedLink(uid, id, lang); bad != "" {
		reply(ctx, u, p(bad))
		return endGroups()
	}
	var exp int64
	label := i18n.T(lang, "never")
	if a := strings.ToLower(args[2]); a != "off" && a != "never" {
		h, ok := parseHours(a)
		if !ok {
			reply(ctx, u, p("⚠️ Invalid value. Examples: 12h, 7d, off."))
			return endGroups()
		}
		exp = time.Now().Add(time.Duration(h) * time.Hour).Unix()
		label = utils.TimeLeft(exp)
	}
	store.Default.UpdateFile(id, func(f *store.FileRecord) { f.ExpiresAt = exp })
	reply(ctx, u, p(i18n.T(lang, "exp_set", label)))
	return endGroups()
}

func cmdStorage(ctx *ext.Context, u *ext.Update) error {
	uid, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	lang := userLang(uid)
	s := store.Default.SummaryOf(uid)
	reply(ctx, u,
		b(i18n.T(lang, "storage_title")), nl(), nl(),
		b(i18n.T(lang, "st_links")+": "), c(fmt.Sprint(s.Files)), p(fmt.Sprintf("  (%s %d · ⌛ %d · 🗑 %d · 🔒 %d)", i18n.T(lang, "st_active"), s.Active, s.Expired, s.Revoked, s.Protected)), nl(),
		b(i18n.T(lang, "st_size")+": "), p(utils.HumanSize(s.Bytes)+" ("+utils.HumanSize(s.ActiveBytes)+" "+i18n.T(lang, "st_active")+")"), nl(),
		b(i18n.T(lang, "st_downloads")+": "), c(fmt.Sprint(s.Downloads)), nl(),
		b(i18n.T(lang, "st_views")+": "), c(fmt.Sprint(s.Views)), nl(),
		b(i18n.T(lang, "st_served")+": "), p(utils.HumanSize(s.BytesServed)),
	)
	return endGroups()
}

func collectionURL(c *store.Collection) string {
	return fmt.Sprintf("%s/c/%s?k=%s", baseURL(), c.ID, c.Token)
}

func cmdCollections(ctx *ext.Context, u *ext.Update) error {
	uid, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	cols := store.Default.CollectionsOf(uid)
	if len(cols) == 0 {
		reply(ctx, u, b("🗂 Collections"), nl(), nl(), p("You have no collections yet. Create one in the app (/app): select files and tap “Add to collection”."))
		return endGroups()
	}
	ps := parts2(b("🗂 Your collections"), nl(), nl())
	for i := range cols {
		col := cols[i]
		state := ""
		if col.Revoked {
			state = " 🗑"
		}
		ps = append(ps, b(utils.ShortName(col.Title, 40)+state), p(fmt.Sprintf(" · %d files", len(col.Files))), nl(), c(collectionURL(&col)), nl(), nl())
	}
	reply(ctx, u, ps...)
	return endGroups()
}

const maxAPIKeys = 5

func cmdAPIKey(ctx *ext.Context, u *ext.Update) error {
	uid, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	args := u.Args()
	sub := ""
	if len(args) > 1 {
		sub = strings.ToLower(args[1])
	}
	switch sub {
	case "new", "create":
		keys := store.Default.APIKeysOf(uid)
		if len(keys) >= maxAPIKeys {
			reply(ctx, u, p(fmt.Sprintf("⚠️ You can have up to %d API keys. Remove one with /apikey revoke <id>.", maxAPIKeys)))
			return endGroups()
		}
		name := "key"
		if len(args) > 2 {
			name = utils.ShortName(strings.Join(args[2:], " "), 30)
		}
		plain, k := store.Default.CreateAPIKey(uid, name)
		reply(ctx, u,
			b("🔑 New API key"), nl(), nl(),
			c(plain), nl(), nl(),
			p("Name: "+k.Name+" · ID: "), c(k.ID), nl(),
			i("Copy it now - it is shown only once. Docs: "+baseURL()+"/docs"),
		)
	case "revoke", "delete", "rm":
		if len(args) < 3 {
			reply(ctx, u, p("Usage: "), c("/apikey revoke <id>"))
			return endGroups()
		}
		if store.Default.DeleteAPIKey(uid, args[2]) {
			reply(ctx, u, p("🗑 API key revoked."))
		} else {
			reply(ctx, u, p("⚠️ No key with that ID."))
		}
	default:
		keys := store.Default.APIKeysOf(uid)
		ps := parts2(b("🔑 API keys"), nl(), nl())
		if len(keys) == 0 {
			ps = append(ps, p("You have no keys yet."), nl())
		}
		for _, k := range keys {
			last := "never used"
			if k.LastUsed != 0 {
				last = "used " + utils.FormatUnix(k.LastUsed)
			}
			ps = append(ps, c(k.ID), p(" · "+k.Name+" · "+k.Prefix+"… · "+last), nl())
		}
		ps = append(ps, nl(), c("/apikey new <name>"), p(" · "), c("/apikey revoke <id>"), nl(), i("API docs: "+baseURL()+"/docs"))
		reply(ctx, u, ps...)
	}
	return endGroups()
}

func cmdWebhook(ctx *ext.Context, u *ext.Update) error {
	uid, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	args := u.Args()
	usr, _ := store.Default.GetUser(uid)
	if len(args) < 2 {
		if usr.WebhookURL == "" {
			reply(ctx, u, b("🪝 Webhook"), nl(), nl(), p("Not set. Receive "), c("link.created"), p(" and "), c("link.viewed"), p(" events at your HTTPS endpoint:"), nl(), c("/webhook https://example.com/hook"), nl(), c("/webhook off"), p(" · "), c("/webhook test"))
		} else {
			reply(ctx, u, b("🪝 Webhook"), nl(), nl(), p("URL: "), c(usr.WebhookURL), nl(), p("Signing secret: "), c(usr.WebhookSecret), nl(), nl(), i("Each request has an X-Fsb-Signature header: sha256= HMAC of the body. Docs: "+baseURL()+"/docs"))
		}
		return endGroups()
	}
	switch strings.ToLower(args[1]) {
	case "off", "remove":
		store.Default.SetWebhook(uid, "", "")
		reply(ctx, u, p("🪝 Webhook removed."))
	case "test":
		if usr.WebhookURL == "" {
			reply(ctx, u, p("⚠️ Set a webhook first."))
			return endGroups()
		}
		if err := webhook.Send(usr.WebhookURL, usr.WebhookSecret, "ping", map[string]any{"message": "Hello from " + config.ValueOf.BotName}); err != nil {
			reply(ctx, u, p("❌ Delivery failed: "+err.Error()))
		} else {
			reply(ctx, u, p("✅ Test event delivered."))
		}
	default:
		raw := strings.TrimSpace(args[1])
		if err := webhook.ValidateURL(raw); err != nil {
			reply(ctx, u, p("⚠️ "+err.Error()))
			return endGroups()
		}
		secret := usr.WebhookSecret
		if secret == "" {
			secret = "whsec_" + store.RandomToken(16)
		}
		store.Default.SetWebhook(uid, raw, secret)
		reply(ctx, u, b("✅ Webhook saved"), nl(), nl(), p("Signing secret: "), c(secret), nl(), i("Send /webhook test to try it."))
	}
	return endGroups()
}

// ---------- inline mode ----------

func onInline(ctx *ext.Context, u *ext.Update) error {
	iq := u.InlineQuery
	if iq == nil {
		return nil
	}
	uid := iq.UserID
	lang := userLang(uid)
	var results []tg.InputBotInlineResultClass

	if UserAllowed(uid) == nil {
		now := time.Now().Unix()
		for _, r := range store.Default.QueryFiles(uid, iq.Query, "", 40) {
			if r.Revoked || (r.ExpiresAt != 0 && now > r.ExpiresAt) {
				continue
			}
			rec := r
			size := utils.HumanSize(rec.Size)
			if rec.Size == 0 {
				size = "photo"
			}
			main := streamURL(&rec)
			if rec.PassHash != "" {
				main = watchURL(&rec)
			}
			text := utils.ShortName(rec.Name, 80) + "\n" + utils.FileKind(rec.Mime) + " • " + size + "\n\n" + main
			var rows [][]tg.KeyboardButtonClass
			if !isLocalhost() {
				var row []tg.KeyboardButtonClass
				if config.ValueOf.EnablePlayer && utils.Playable(rec.Mime) || rec.PassHash != "" {
					row = append(row, btnURL(i18n.T(lang, "btn_player"), watchURL(&rec)))
				}
				if rec.PassHash == "" {
					row = append(row, btnURL(i18n.T(lang, "btn_download"), streamURL(&rec)+"&d=true"))
				}
				if len(row) > 0 {
					rows = append(rows, row)
				}
			}
			msg := &tg.InputBotInlineMessageText{Message: text, NoWebpage: true}
			if len(rows) > 0 {
				msg.ReplyMarkup = kb(rows...)
			}
			desc := utils.FileKind(rec.Mime) + " • " + size
			if rec.PassHash != "" {
				desc += " • 🔒"
			}
			results = append(results, &tg.InputBotInlineResult{
				ID:          strconv.Itoa(rec.MessageID),
				Type:        "article",
				Title:       utils.ShortName(rec.Name, 60),
				Description: desc,
				SendMessage: msg,
			})
			if len(results) >= 25 {
				break
			}
		}
	}
	if len(results) == 0 {
		results = append(results, &tg.InputBotInlineResult{
			ID:          "none",
			Type:        "article",
			Title:       i18n.T(lang, "files_title"),
			Description: i18n.T(lang, "files_empty"),
			SendMessage: &tg.InputBotInlineMessageText{Message: i18n.T(lang, "files_empty")},
		})
	}
	_, _ = ctx.SetInlineBotResult(&tg.MessagesSetInlineBotResultsRequest{
		QueryID:   iq.QueryID,
		Results:   results,
		CacheTime: 5,
		Private:   true,
	})
	return endGroups()
}

// ---------- channel mode ----------

var chanLimiter = auth.NewLimiter()

// onChannelPost adds Watch / Download buttons under media posted in approved
// channels, and handles the "/watchbuttons" request post.
func onChannelPost(ctx *ext.Context, u *ext.Update) error {
	if _, ok := u.UpdateClass.(*tg.UpdateNewChannelMessage); !ok {
		return nil // edits and everything else continue down the chain
	}
	chat := u.EffectiveChat()
	if chat == nil || !chat.IsAChannel() {
		return nil
	}
	msg := u.EffectiveMessage
	if msg == nil || msg.Message == nil {
		return nil
	}
	chID := chat.GetID()
	if chID == config.ValueOf.LogChannelID {
		return endGroups() // our own forwarded copies
	}

	if f := strings.Fields(msg.Text); len(f) > 0 && strings.HasPrefix(strings.ToLower(f[0]), "/watchbuttons") {
		handleWatchButtonsRequest(ctx, u, chID, f)
		return endGroups()
	}

	ch, ok := store.Default.GetChannel(chID)
	if !ok || msg.Media == nil || msg.ReplyMarkup != nil {
		return endGroups()
	}
	rec, err := createLinkFrom(ctx, chID, ch.OwnerID, msg.ID, msg.Media, LinkOptions{})
	if err != nil {
		return endGroups()
	}
	if isLocalhost() {
		return endGroups()
	}
	var row []tg.KeyboardButtonClass
	if config.ValueOf.EnablePlayer && utils.Playable(rec.Mime) {
		row = append(row, btnURL("▶️ Watch", watchURL(rec)))
	}
	row = append(row, btnURL("⬇️ Download", streamURL(rec)+"&d=true"))
	_, _ = ctx.EditMessage(chID, &tg.MessagesEditMessageRequest{ID: msg.ID, ReplyMarkup: kb(row)})
	return endGroups()
}

func handleWatchButtonsRequest(ctx *ext.Context, u *ext.Update, chID int64, f []string) {
	_ = ctx.DeleteMessages(chID, []int{u.EffectiveMessage.ID}) // keep the channel clean
	if len(f) > 1 && strings.EqualFold(f[1], "off") {
		store.Default.SetChannel(chID, 0, false)
		return
	}
	if _, ok := store.Default.GetChannel(chID); ok {
		return
	}
	if !chanLimiter.Allow(strconv.FormatInt(chID, 10), 1, 10*time.Minute) {
		return
	}
	title := fmt.Sprint(chID)
	if u.Entities != nil {
		if c, ok := u.Entities.Channels[chID]; ok {
			title = c.Title + " (" + title + ")"
		}
	}
	markup := kb([]tg.KeyboardButtonClass{
		btnCB("✅ Approve", fmt.Sprintf("ahar:ch:ok:%d", chID)),
		btnCB("❌ Deny", fmt.Sprintf("ahar:ch:no:%d", chID)),
	})
	for _, owner := range config.ValueOf.Admins {
		sendStyled(ctx, owner, parts2(b("📣 Channel request"), nl(), nl(), p("The channel "+title+" asked to get Watch / Download buttons under its media posts."), nl(), p("Approve it?")), markup)
	}
}

// channelDecision handles the owner's Approve / Deny buttons.
func channelDecision(ctx *ext.Context, cb *tg.UpdateBotCallbackQuery, uid int64, data string) bool {
	if !IsOwner(uid) {
		answerCB(ctx, cb, "Owner only.", true)
		return true
	}
	seg := strings.Split(data, ":") // ahar, ch, ok|no, id
	if len(seg) != 4 {
		return false
	}
	chID, err := strconv.ParseInt(seg[3], 10, 64)
	if err != nil {
		return false
	}
	if seg[2] == "ok" {
		store.Default.SetChannel(chID, uid, true)
		editText(ctx, uid, cb.MsgID, parts2(b("✅ Channel approved"), nl(), p(fmt.Sprintf("New media posts in %d now get Watch / Download buttons. Post /watchbuttons off in the channel to stop.", chID))), nil)
		answerCB(ctx, cb, "Approved", false)
	} else {
		editText(ctx, uid, cb.MsgID, parts2(b("❌ Channel denied")), nil)
		answerCB(ctx, cb, "Denied", false)
	}
	return true
}
