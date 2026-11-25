package commands

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/store"
	"EverythingSuckz/fsb/internal/utils"

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

// createLink forwards the user's message into the log channel and stores a link record.
func createLink(ctx *ext.Context, userID int64, srcMsgID int, media tg.MessageMediaClass) (*store.FileRecord, error) {
	admin := IsAdmin(userID)

	if media == nil || !isSupportedMedia(media) {
		return nil, &linkError{"Sorry, this message type is unsupported. Send a file, video, audio, photo or document."}
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

	update, err := utils.ForwardMessages(ctx, userID, config.ValueOf.LogChannelID, srcMsgID)
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
	}
	store.Default.AddFile(rec)
	return rec, nil
}

// linkMessage builds the text and keyboard that accompany a fresh link.
func linkMessage(rec *store.FileRecord) ([]part, *tg.ReplyInlineMarkup) {
	link := streamURL(rec)
	size := utils.HumanSize(rec.Size)
	if rec.Size == 0 {
		size = "photo"
	}
	text := parts(
		b("✅ Your link is ready!"), nl(), nl(),
		b("📄 "), p(utils.ShortName(rec.Name, 60)), nl(),
		b("📊 "), p(utils.FileKind(rec.Mime)+" • "+size), nl(),
		b("⏳ "), p(utils.TimeLeft(rec.ExpiresAt)), nl(),
		b("🆔 "), c(fmt.Sprint(rec.MessageID)), nl(), nl(),
		c(link),
	)

	var rows [][]tg.KeyboardButtonClass
	if !strings.Contains(link, "http://localhost") { // Telegram rejects localhost URL buttons
		row := []tg.KeyboardButtonClass{btnURL("⬇️ Download", link+"&d=true")}
		if strings.Contains(rec.Mime, "video") || strings.Contains(rec.Mime, "audio") || strings.Contains(rec.Mime, "pdf") {
			row = append(row, btnURL("▶️ Stream", link))
		}
		rows = append(rows, row)
		if config.ValueOf.EnablePlayer && utils.Playable(rec.Mime) {
			rows = append(rows, []tg.KeyboardButtonClass{btnURL("🎞 Open in Web Player", watchURL(rec))})
		}
	}
	rows = append(rows, []tg.KeyboardButtonClass{
		btnCB("🗑 Revoke link", fmt.Sprintf("ahar:rv:%d", rec.MessageID)),
		btnCB("📂 My files", "ahar:files"),
	})
	return text, kb(rows...)
}
