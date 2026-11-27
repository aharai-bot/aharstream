package routes

import (
	"net/http"
	"time"

	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/auth"
	"EverythingSuckz/fsb/internal/bot"
	"EverythingSuckz/fsb/internal/commands"
	"EverythingSuckz/fsb/internal/store"

	"github.com/gin-gonic/gin"
)

const sessionCookie = "fsb_s"

var authLimiter = auth.NewLimiter()

func (e *allRoutes) LoadSession(r *Route) {
	r.Engine.GET("/auth/login", authLoginLink)
	r.Engine.GET("/auth/telegram", authWidget)
	r.Engine.POST("/auth/webapp", authWebApp)
	r.Engine.POST("/auth/logout", func(ctx *gin.Context) {
		ctx.SetSameSite(http.SameSiteLaxMode)
		ctx.SetCookie(sessionCookie, "", -1, "/", "", isHTTPSHost(), true)
		ctx.JSON(http.StatusOK, gin.H{"ok": true})
	})
}

func setSession(ctx *gin.Context, uid int64) {
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(sessionCookie, auth.SessionToken(uid), int(auth.SessionTTL.Seconds()), "/", "", isHTTPSHost(), true)
}

func sessionUser(ctx *gin.Context) (int64, bool) {
	c, err := ctx.Cookie(sessionCookie)
	if err != nil {
		return 0, false
	}
	return auth.ParseSession(c)
}

func authAllowed(ctx *gin.Context) bool {
	if !authLimiter.Allow(ctx.ClientIP(), 30, 10*time.Minute) {
		ctx.String(http.StatusTooManyRequests, "too many attempts, try again later")
		return false
	}
	return true
}

func authFail(ctx *gin.Context, status int, msg string) {
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(status, errorPage(msg))
}

// GET /auth/login?t=...  (one-time link from the bot's /login command)
func authLoginLink(ctx *gin.Context) {
	if !authAllowed(ctx) {
		return
	}
	uid, ok := auth.ConsumeLoginToken(ctx.Query("t"))
	if !ok {
		authFail(ctx, http.StatusUnauthorized, "This sign-in link is invalid or has expired. Send /login to the bot for a new one.")
		return
	}
	if err := commands.UserAllowed(uid); err != nil {
		authFail(ctx, http.StatusForbidden, err.Error())
		return
	}
	setSession(ctx, uid)
	ctx.Redirect(http.StatusSeeOther, "/app")
}

// GET /auth/telegram?id=..&hash=..  ("Log in with Telegram" widget callback)
func authWidget(ctx *gin.Context) {
	if !authAllowed(ctx) {
		return
	}
	u, err := auth.ValidateLoginWidget(ctx.Request.URL.Query(), config.ValueOf.BotToken, 10*time.Minute)
	if err != nil {
		authFail(ctx, http.StatusUnauthorized, "Telegram sign-in failed: "+err.Error())
		return
	}
	if err := commands.UserAllowed(u.ID); err != nil {
		authFail(ctx, http.StatusForbidden, err.Error())
		return
	}
	store.Default.Touch(u.ID, u.Username, u.FirstName)
	setSession(ctx, u.ID)
	ctx.Redirect(http.StatusSeeOther, "/app")
}

// POST /auth/webapp  {"initData": "..."}  (inside the Telegram Mini App)
func authWebApp(ctx *gin.Context) {
	if ctx.GetHeader("X-Requested-With") != "fetch" {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "bad request"})
		return
	}
	if !authLimiter.Allow(ctx.ClientIP(), 30, 10*time.Minute) {
		ctx.JSON(http.StatusTooManyRequests, gin.H{"error": "too many attempts"})
		return
	}
	var body struct {
		InitData string `json:"initData"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil || body.InitData == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "missing initData"})
		return
	}
	u, err := auth.ValidateWebApp(body.InitData, config.ValueOf.BotToken, 24*time.Hour)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	if err := commands.UserAllowed(u.ID); err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	store.Default.Touch(u.ID, u.Username, u.FirstName)
	setSession(ctx, u.ID)
	ctx.JSON(http.StatusOK, gin.H{"ok": true})
}

// botUsername returns the main bot's @username (without @) when known.
func botUsername() string {
	if bot.Workers != nil && len(bot.Workers.Bots) > 0 && bot.Workers.Bots[0].Self != nil {
		return bot.Workers.Bots[0].Self.Username
	}
	return ""
}

// withUser protects the app API: it needs a valid session and, for anything
// that changes data, the X-Requested-With header (a simple CSRF guard).
func withUser(h func(*gin.Context, int64)) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		uid, ok := sessionUser(ctx)
		if !ok {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": "not signed in"})
			return
		}
		if err := commands.UserAllowed(uid); err != nil {
			ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		if ctx.Request.Method != http.MethodGet && ctx.GetHeader("X-Requested-With") != "fetch" {
			ctx.JSON(http.StatusForbidden, gin.H{"error": "bad request"})
			return
		}
		ctx.Header("Cache-Control", "no-store")
		h(ctx, uid)
	}
}
