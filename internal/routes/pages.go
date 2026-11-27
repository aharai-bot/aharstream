package routes

import (
	"crypto/subtle"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/store"
	"EverythingSuckz/fsb/internal/utils"

	"github.com/gin-gonic/gin"
)

func (e *allRoutes) LoadPages(r *Route) {
	r.Engine.GET("/app", getAppPage)
	r.Engine.GET("/docs", getDocsPage)
	r.Engine.GET("/c/:id", getCollectionPage)
}

func renderPage(ctx *gin.Context, status int, name string, data any) {
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.Header("Cache-Control", "no-store")
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Status(status)
	if err := pages.ExecuteTemplate(ctx.Writer, name, data); err != nil {
		log.Sugar().Error(err)
	}
}

// publicBase returns the public base URL (HOST, or the request's own origin).
func publicBase(ctx *gin.Context) string {
	if h := strings.TrimRight(config.ValueOf.Host, "/"); h != "" {
		return h
	}
	scheme := "http"
	if ctx.Request.TLS != nil || ctx.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + ctx.Request.Host
}

type appData struct {
	BotName string
	BotUser string
	Version string
}

func getAppPage(ctx *gin.Context) {
	renderPage(ctx, http.StatusOK, "app.html", appData{
		BotName: config.ValueOf.BotName,
		BotUser: botUsername(),
		Version: utils.Version,
	})
}

type docsData struct {
	BotName string
	Host    string
}

func getDocsPage(ctx *gin.Context) {
	renderPage(ctx, http.StatusOK, "docs.html", docsData{BotName: config.ValueOf.BotName, Host: publicBase(ctx)})
}

type collItem struct {
	Name     string
	Size     string
	Kind     string
	Watch    string
	Download string
	Locked   bool
	Status   string // "" when usable, otherwise a short reason
}

type collData struct {
	BotName string
	Title   string
	Items   []collItem
	Count   int
	Total   string
}

func getCollectionPage(ctx *gin.Context) {
	c, ok := store.Default.GetCollection(ctx.Param("id"))
	if !ok || subtle.ConstantTimeCompare([]byte(ctx.Query("k")), []byte(c.Token)) != 1 {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(http.StatusNotFound, errorPage("This collection does not exist."))
		return
	}
	if c.Revoked || store.Default.IsBanned(c.OwnerID) {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(http.StatusGone, errorPage("This collection is no longer shared."))
		return
	}
	now := time.Now().Unix()
	var total int64
	d := collData{BotName: config.ValueOf.BotName, Title: c.Title}
	for _, id := range c.Files {
		rec, ok := store.Default.GetFile(id)
		if !ok {
			continue
		}
		it := collItem{
			Name:   rec.Name,
			Size:   utils.HumanSize(rec.Size),
			Kind:   utils.FileKind(rec.Mime),
			Locked: rec.PassHash != "",
		}
		if rec.Size == 0 {
			it.Size = "Photo"
		}
		switch {
		case rec.Revoked:
			it.Status = "Removed by the owner"
		case rec.ExpiresAt != 0 && now > rec.ExpiresAt:
			it.Status = "Expired"
		case rec.MaxViews > 0 && rec.Views >= rec.MaxViews:
			it.Status = "View limit reached"
		}
		q := "?hash=" + url.QueryEscape(rec.Hash)
		sid := strconv.Itoa(rec.MessageID)
		if config.ValueOf.EnablePlayer && (utils.Playable(rec.Mime) || it.Locked) {
			it.Watch = "/watch/" + sid + q
		}
		if !it.Locked {
			it.Download = "/stream/" + sid + q + "&d=true"
		}
		total += rec.Size
		d.Items = append(d.Items, it)
	}
	d.Count = len(d.Items)
	d.Total = utils.HumanSize(total)
	renderPage(ctx, http.StatusOK, "collection.html", d)
}
