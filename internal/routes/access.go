package routes

import (
	"crypto/subtle"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/auth"
	"EverythingSuckz/fsb/internal/commands"
	"EverythingSuckz/fsb/internal/store"
	"EverythingSuckz/fsb/internal/webhook"

	"github.com/gin-gonic/gin"
)

//go:embed pages/*.html
var pagesFS embed.FS

var pages = template.Must(template.ParseFS(pagesFS, "pages/*.html"))

var (
	seenVisitors  = auth.NewTTLSet()
	unlockLimiter = auth.NewLimiter()
)

func (e *allRoutes) LoadAccess(r *Route) {
	r.Engine.POST("/unlock/:messageID", postUnlock)
}

func unlockCookie(id int) string { return "fsb_u_" + strconv.Itoa(id) }

func isHTTPSHost() bool { return strings.HasPrefix(config.ValueOf.Host, "https://") }

// linkToken returns the access token sent with a request: the "k" query value
// (put into stream URLs by the watch page) or the cookie set after unlocking.
func linkToken(ctx *gin.Context, id int) (tok string, fromQuery bool) {
	if k := ctx.Query("k"); k != "" {
		return k, true
	}
	if c, err := ctx.Cookie(unlockCookie(id)); err == nil {
		return c, false
	}
	return "", false
}

// gateLink applies the password and view-limit rules of a link and records the
// visit for the owner's analytics. status == 0 means the request may continue.
func gateLink(ctx *gin.Context, rec *store.FileRecord) (int, string) {
	hash := ctx.Query("hash")
	tok, fromQuery := linkToken(ctx, rec.MessageID)
	unlocked := auth.CheckUnlock(tok, rec.MessageID, hash)

	if rec.PassHash != "" && !unlocked {
		return http.StatusUnauthorized, "password required"
	}
	if fromQuery && unlocked {
		return 0, "" // the watch page already counted this visit
	}

	ua := ctx.GetHeader("User-Agent")
	device := auth.ClassifyUA(ua)
	visitor := fmt.Sprintf("%d|%s|%s", rec.MessageID, ctx.ClientIP(), ua)
	if seenVisitors.Has(visitor) {
		return 0, ""
	}
	// Link previews (Telegram, WhatsApp...) and HEAD probes must not use up views.
	if device == "Bot" || ctx.Request.Method == http.MethodHead {
		if rec.MaxViews > 0 && rec.Views >= rec.MaxViews {
			return http.StatusGone, "this link has reached its view limit"
		}
		return 0, ""
	}
	country := auth.Country(ctx.Request.Header)
	if country == "" {
		country = "??"
	}
	if !store.Default.AddView(rec.MessageID, country, device) {
		return http.StatusGone, "this link has reached its view limit"
	}
	seenVisitors.Add(visitor, 10*time.Minute)

	if u, ok := store.Default.GetUser(rec.OwnerID); ok && u.WebhookURL != "" {
		webhook.Fire(u.WebhookURL, u.WebhookSecret, "link.viewed", map[string]any{
			"id":      rec.MessageID,
			"name":    rec.Name,
			"device":  device,
			"country": country,
			"views":   rec.Views + 1,
		})
	}
	return 0, ""
}

type unlockData struct {
	BotName string
	Action  string
	Error   string
}

func renderUnlock(ctx *gin.Context, messageID int, hash, errMsg string, status int) {
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.Header("Cache-Control", "no-store")
	ctx.Header("Referrer-Policy", "no-referrer")
	ctx.Status(status)
	d := unlockData{
		BotName: config.ValueOf.BotName,
		Action:  "/unlock/" + strconv.Itoa(messageID) + "?hash=" + url.QueryEscape(hash),
		Error:   errMsg,
	}
	if err := pages.ExecuteTemplate(ctx.Writer, "unlock.html", d); err != nil {
		log.Sugar().Error(err)
	}
}

func postUnlock(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("messageID"))
	if err != nil {
		ctx.String(http.StatusBadRequest, "bad id")
		return
	}
	hash := ctx.Query("hash")
	rec, ok := store.Default.GetFile(id)
	if !ok || rec.PassHash == "" || subtle.ConstantTimeCompare([]byte(hash), []byte(rec.Hash)) != 1 {
		ctx.String(http.StatusNotFound, "link unavailable")
		return
	}
	key := ctx.ClientIP() + "|" + strconv.Itoa(id)
	if !unlockLimiter.Allow(key, 8, 10*time.Minute) {
		renderUnlock(ctx, id, hash, "Too many attempts. Please wait a few minutes.", http.StatusTooManyRequests)
		return
	}
	if !auth.CheckPassword(rec.PassHash, ctx.PostForm("password")) {
		renderUnlock(ctx, id, hash, "Wrong password. Try again.", http.StatusUnauthorized)
		return
	}
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(unlockCookie(id), auth.UnlockToken(id, hash), int(auth.UnlockTTL.Seconds()), "/", "", isHTTPSHost(), true)
	ctx.Redirect(http.StatusSeeOther, "/watch/"+strconv.Itoa(id)+"?hash="+url.QueryEscape(hash))
}

// fileJSON is the owner's view of a link (used by the app API and the public API).
func fileJSON(r *store.FileRecord) gin.H {
	status := "active"
	switch {
	case r.Revoked:
		status = "revoked"
	case r.ExpiresAt != 0 && time.Now().Unix() > r.ExpiresAt:
		status = "expired"
	}
	h := gin.H{
		"id":          r.MessageID,
		"name":        r.Name,
		"size":        r.Size,
		"mime":        r.Mime,
		"created_at":  r.CreatedAt,
		"expires_at":  r.ExpiresAt,
		"status":      status,
		"protected":   r.PassHash != "",
		"max_views":   r.MaxViews,
		"views":       r.Views,
		"downloads":   r.Downloads,
		"bytes":       r.Bytes,
		"folder":      r.Folder,
		"last_viewed": r.LastViewed,
		"countries":   r.Countries,
		"devices":     r.Devices,
	}
	if config.ValueOf.EnablePlayer {
		h["watch_url"] = commands.WatchURL(r)
	}
	if r.PassHash == "" {
		h["stream_url"] = commands.StreamURL(r)
		h["download_url"] = commands.StreamURL(r) + "&d=true"
	}
	return h
}
