package commands

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/auth"
	"EverythingSuckz/fsb/internal/i18n"
	"EverythingSuckz/fsb/internal/store"
	"EverythingSuckz/fsb/internal/utils"
	"EverythingSuckz/fsb/internal/webhook"

	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
)

// linkError is shown to the user verbatim.
type linkError struct{ msg string }

func (e *linkError) Error() string { return e.msg }

func isSupportedMedia(m tg.MessageMediaClass) bool {
	switch m.(type) {
	case *tg.MessageMediaDocument, *tg.MessageMediaPhoto:
		return true
	}
	return false
}

// watchURL / streamURL build public links for a record.
func streamURL(rec *store.FileRecord) string {
	return fmt.Sprintf("%s/stream/%d?hash=%s", config.ValueOf.Host, rec.MessageID, rec.Hash)
}

func watchURL(rec *store.FileRecord) string {
	return fmt.Sprintf("%s/watch/%d?hash=%s", config.ValueOf.Host, rec.MessageID, rec.Hash)
}

// StreamURL / WatchURL build public links for a record (used by the web API too).
func StreamURL(rec *store.FileRecord) string { return streamURL(rec) }
func WatchURL(rec *store.FileRecord) string  { return watchURL(rec) }

// LinkOptions are the per-link settings a user can choose.
type LinkOptions struct {
	ExpiryHours *int   // nil = user / server default, 0 = never expires
	Password    string // "" = public
	MaxViews    int    // 0 = unlimited
	Folder      string
}

// LinkPayload is the JSON shape used by webhooks and the public API.
func LinkPayload(rec *store.FileRecord) map[string]any {
	m := map[string]any{
		"id":         rec.MessageID,
		"name":       rec.Name,
		"size":       rec.Size,
		"mime":       rec.Mime,
		"created_at": rec.CreatedAt,
		"expires_at": rec.ExpiresAt,
		"protected":  rec.PassHash != "",
		"max_views":  rec.MaxViews,
		"views":      rec.Views,
		"stream_url": streamURL(rec),
	}
	if config.ValueOf.EnablePlayer {
		m["watch_url"] = watchURL(rec)
	}
	return m
}

// userLang returns the user's bot language code.
func userLang(id int64) string {
	if u, ok := store.Default.GetUser(id); ok && i18n.Valid(u.Lang) {
		return u.Lang
	}
	return "en"
}

// createLink forwards the user's message into the log channel and stores a link record.
func createLink(ctx *ext.Context, userID int64, srcMsgID int, media tg.MessageMediaClass) (*store.FileRecord, error) {
	return createLinkFrom(ctx, userID, userID, srcMsgID, media, LinkOptions{})
}

// createLinkFrom is createLink for any source chat (private chat or channel).
func createLinkFrom(ctx *ext.Context, fromChatID, userID int64, srcMsgID int, media tg.MessageMediaClass, opts LinkOptions) (*store.FileRecord, error) {
	admin := IsAdmin(userID)

	if media == nil || !isSupportedMedia(media) {
		return nil, &linkError{"Sorry, this message type is unsupported. Send a file, video, audio, photo or document."}
	}
	if opts.Password != "" && !config.ValueOf.EnablePlayer {
		return nil, &linkError{i18n.T(userLang(userID), "pw_need_web")}
	}
	pre, err := utils.FileFromMedia(media)
	if err != nil {
		return nil, &linkError{"Could not read this file: " + err.Error()}
	}
	if !admin && config.ValueOf.MaxFileSizeMB > 0 && pre.FileSize > config.ValueOf.MaxFileSizeMB*1024*1024 {
		return nil, &linkError{fmt.Sprintf("File is too large (%s). The limit is %d MB.", utils.HumanSize(pre.FileSize), config.ValueOf.MaxFileSizeMB)}
	}
	if !admin && config.ValueOf.MaxLinksPerHour > 0 {
		n := store.Default.CountUserLinksSince(userID, time.Now().Add(-time.Hour).Unix())
		if n >= config.ValueOf.MaxLinksPerHour {
			return nil, &linkError{fmt.Sprintf("Slow down! You can create up to %d links per hour.", config.ValueOf.MaxLinksPerHour)}
		}
	}

	update, err := utils.ForwardMessages(ctx, fromChatID, config.ValueOf.LogChannelID, srcMsgID)
	if err != nil {
		utils.Logger.Sugar().Error(err)
		return nil, err
	}
	if len(update.Updates) < 2 {
		return nil, errors.New("unexpected update structure from Telegram")
	}
	msgIDUpdate, ok := update.Updates[0].(*tg.UpdateMessageID)
	if !ok {
		return nil, errors.New("unexpected update type")
	}
	newMsg, ok := update.Updates[1].(*tg.UpdateNewChannelMessage)
	if !ok {
		return nil, errors.New("unexpected channel message update")
	}
	msg, ok := newMsg.Message.(*tg.Message)
	if !ok {
		return nil, errors.New("unexpected message type")
	}
	file, err := utils.FileFromMedia(msg.Media)
	if err != nil {
		return nil, err
	}
	hash := utils.GetShortHash(utils.PackFile(file.FileName, file.FileSize, file.MimeType, file.ID))

	hours := config.ValueOf.DefaultExpiryHours
	if usr, ok := store.Default.GetUser(userID); ok && usr.ExpiryHours != 0 {
		hours = usr.ExpiryHours
		if hours < 0 { // -1 means the user explicitly chose "never"
			hours = 0
		}
	}
	if opts.ExpiryHours != nil {
		hours = *opts.ExpiryHours
	}
	var expires int64
	if hours > 0 {
		expires = time.Now().Add(time.Duration(hours) * time.Hour).Unix()
	}

	rec := &store.FileRecord{
		MessageID: msgIDUpdate.ID,
		OwnerID:   userID,
		Name:      utils.DisplayName(file.FileName, file.MimeType, file.ID),
		Size:      file.FileSize,
		Mime:      file.MimeType,
		Hash:      hash,
		CreatedAt: time.Now().Unix(),
		ExpiresAt: expires,
		MaxViews:  opts.MaxViews,
		Folder:    opts.Folder,
	}
	if opts.Password != "" {
		ph, err := auth.HashPassword(opts.Password)
		if err != nil {
			return nil, err
		}
		rec.PassHash = ph
	}
	store.Default.AddFile(rec)
	if u, ok := store.Default.GetUser(userID); ok && u.WebhookURL != "" {
		webhook.Fire(u.WebhookURL, u.WebhookSecret, "link.created", LinkPayload(rec))
	}
	return rec, nil
}

// linkMessage builds the text and keyboard that accompany a fresh link.
func linkMessage(rec *store.FileRecord, lang string) ([]part, *tg.ReplyInlineMarkup) {
	link := streamURL(rec)
	size := utils.HumanSize(rec.Size)
	if rec.Size == 0 {
		size = "photo"
	}
	protected := rec.PassHash != ""
	text := parts(
		b(i18n.T(lang, "link_ready")), nl(), nl(),
		b("📄 "), p(utils.ShortName(rec.Name, 60)), nl(),
		b("📊 "), p(utils.FileKind(rec.Mime)+" • "+size), nl(),
		b("⏳ "), p(utils.TimeLeft(rec.ExpiresAt)), nl(),
	)
	if protected {
		text = append(text, b("🔒 "), p(i18n.T(lang, "locked")), nl())
	}
	if rec.MaxViews > 0 {
		text = append(text, p(i18n.T(lang, "view_limit", rec.Views, rec.MaxViews)), nl())
	}
	text = append(text, b("🆔 "), c(fmt.Sprint(rec.MessageID)), nl(), nl())
	if !protected {
		text = append(text, c(link))
	}

	var rows [][]tg.KeyboardButtonClass
	if !strings.Contains(link, "http://localhost") { // Telegram rejects localhost URL buttons
		if !protected {
			row := []tg.KeyboardButtonClass{btnURL(i18n.T(lang, "btn_download"), link+"&d=true")}
			if strings.Contains(rec.Mime, "video") || strings.Contains(rec.Mime, "audio") || strings.Contains(rec.Mime, "pdf") {
				row = append(row, btnURL(i18n.T(lang, "btn_stream"), link))
			}
			rows = append(rows, row)
		}
		if config.ValueOf.EnablePlayer && utils.Playable(rec.Mime) || protected {
			row := []tg.KeyboardButtonClass{btnURL(i18n.T(lang, "btn_player"), watchURL(rec))}
			if isHTTPS() {
				row = append(row, &tg.KeyboardButtonWebView{Text: i18n.T(lang, "btn_app"), URL: watchURL(rec)})
			}
			rows = append(rows, row)
		}
	}
	id := rec.MessageID
	rows = append(rows,
		[]tg.KeyboardButtonClass{
			btnCB("⏳ 1h", fmt.Sprintf("ahar:ex:%d:1", id)),
			btnCB("⏳ 1d", fmt.Sprintf("ahar:ex:%d:24", id)),
			btnCB("⏳ 7d", fmt.Sprintf("ahar:ex:%d:168", id)),
			btnCB("♾", fmt.Sprintf("ahar:ex:%d:-1", id)),
		},
		[]tg.KeyboardButtonClass{
			btnCB(i18n.T(lang, "btn_password"), fmt.Sprintf("ahar:pw:%d", id)),
			btnCB("👁 ×1", fmt.Sprintf("ahar:vw:%d:1", id)),
			btnCB("👁 ×10", fmt.Sprintf("ahar:vw:%d:10", id)),
			btnCB("👁 ∞", fmt.Sprintf("ahar:vw:%d:0", id)),
		},
		[]tg.KeyboardButtonClass{
			btnCB(i18n.T(lang, "btn_revoke"), fmt.Sprintf("ahar:rv:%d", id)),
			btnCB(i18n.T(lang, "btn_files"), "ahar:files"),
		},
	)
	return text, kb(rows...)
}

// isHTTPS reports whether HOST is a public https URL (required for Mini Apps).
func isHTTPS() bool { return strings.HasPrefix(config.ValueOf.Host, "https://") }
