package routes

import (
	"net/http"
	"time"

	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/bot"
	"EverythingSuckz/fsb/internal/store"
	"EverythingSuckz/fsb/internal/utils"

	"github.com/gin-gonic/gin"
)

var startedAt = time.Now()

func (e *allRoutes) LoadMisc(r *Route) {
	r.Engine.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.Engine.GET("/api/stats", func(ctx *gin.Context) {
		st, files := store.Default.GetStats()
		total, _ := store.Default.UserCount()
		ctx.JSON(http.StatusOK, gin.H{
			"name":            config.ValueOf.BotName,
			"version":         utils.Version,
			"uptime":          utils.TimeFormat(uint64(time.Since(startedAt).Seconds())),
			"workers":         len(bot.Workers.Bots),
			"users":           total,
			"links_stored":    files,
			"links_created":   st.TotalLinks,
			"total_downloads": st.TotalDownloads,
			"bytes_served":    st.TotalBytes,
		})
	})
}
