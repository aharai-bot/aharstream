package routes

import (
	_ "embed"
	"html/template"
	"net/http"
	"strings"
	"time"

	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/bot"
	"EverythingSuckz/fsb/internal/utils"

	"github.com/gin-gonic/gin"
)

//go:embed home.html
var homeHTML string

var homeTmpl = template.Must(template.New("home").Parse(homeHTML))

type homeData struct {
	BotName   string
	Version   string
	BotUser   string
	Workers   int
	Player    bool
	StartedMs int64
	Uptime    string
}

// WantsHTML reports whether the request comes from a browser (so "/" can show
// the landing page while API clients and monitors still get the JSON status).
func WantsHTML(ctx *gin.Context) bool {
	if ctx.Query("format") == "json" {
		return false
	}
	return strings.Contains(ctx.GetHeader("Accept"), "text/html")
}

// ServeHome renders the landing page.
func ServeHome(ctx *gin.Context, version string, started time.Time) {
	d := homeData{
		BotName:   config.ValueOf.BotName,
		Version:   version,
		Workers:   len(bot.Workers.Bots),
		Player:    config.ValueOf.EnablePlayer,
		StartedMs: started.UnixMilli(),
		Uptime:    utils.TimeFormat(uint64(time.Since(started).Seconds())),
	}
	if len(bot.Workers.Bots) > 0 && bot.Workers.Bots[0].Self != nil {
		d.BotUser = bot.Workers.Bots[0].Self.Username
	}
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Header("Cache-Control", "no-store")
	ctx.Status(http.StatusOK)
	if err := homeTmpl.Execute(ctx.Writer, d); err != nil {
		log.Sugar().Error(err)
	}
}
