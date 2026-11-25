package routes

import (
	_ "embed"
	"html/template"
	"net/url"
	"strconv"
	"strings"

	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/utils"

	"github.com/gin-gonic/gin"
)

func (e *allRoutes) LoadWatch(r *Route) {
	if !config.ValueOf.EnablePlayer {
		return
	}
	r.Engine.GET("/watch/:messageID", getWatchRoute)
}

type watchData struct {
	BotName     string
	Name        string
	Size        string
	Kind        string // video | audio | image | pdf | other
	KindLabel   string
	StreamURL   string
	DownloadURL string
	Downloads   int64
	Expires     string
}

func getWatchRoute(ctx *gin.Context) {
	file, rec, _, messageID, status, msg := resolveFile(ctx)
	if status != 0 {
		ctx.Header("Content-Type", "text/html; charset=utf-8")
		ctx.String(status, errorPage(msg))
		return
	}
	id := strconv.Itoa(messageID)
	base := "/stream/" + id + "?hash=" + url.QueryEscape(ctx.Query("hash"))

	m := strings.ToLower(file.MimeType)
	kind := "other"
	switch {
	case strings.HasPrefix(m, "video/"):
		kind = "video"
	case strings.HasPrefix(m, "audio/"):
		kind = "audio"
	case strings.HasPrefix(m, "image/") && !riskyInline(m):
		kind = "image"
	case strings.Contains(m, "pdf"):
		kind = "pdf"
	}
	size := utils.HumanSize(file.FileSize)
	if file.FileSize == 0 {
		size = "Photo"
	}
	d := watchData{
		BotName:     config.ValueOf.BotName,
		Name:        utils.DisplayName(file.FileName, file.MimeType, file.ID),
		Size:        size,
		Kind:        kind,
		KindLabel:   utils.FileKind(file.MimeType),
		StreamURL:   base,
		DownloadURL: base + "&d=true",
		Expires:     "No expiry",
	}
	if rec != nil {
		d.Downloads = rec.Downloads
		if rec.ExpiresAt != 0 {
			d.Expires = utils.TimeLeft(rec.ExpiresAt)
		}
	}
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Header("Referrer-Policy", "no-referrer")
	if err := watchTmpl.Execute(ctx.Writer, d); err != nil {
		log.Sugar().Error(err)
	}
}

func errorPage(msg string) string {
	return `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Link unavailable</title>
<body style="margin:0;min-height:100vh;display:grid;place-items:center;background:#0b1020;color:#e8ecf8;font-family:system-ui,sans-serif">
<div style="text-align:center;padding:24px"><div style="font-size:54px">🚫</div><h2>Link unavailable</h2><p style="opacity:.7">` +
		template.HTMLEscapeString(msg) + `</p></div></body>`
}

//go:embed watch.html
var watchHTML string

var watchTmpl = template.Must(template.New("watch").Parse(watchHTML))
