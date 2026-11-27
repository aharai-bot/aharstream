package routes

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"EverythingSuckz/fsb/internal/auth"
	"EverythingSuckz/fsb/internal/commands"
	"EverythingSuckz/fsb/internal/store"

	"github.com/gin-gonic/gin"
)

var apiKeyLimiter = auth.NewLimiter()

func (e *allRoutes) LoadAPIv1(r *Route) {
	g := r.Engine.Group("/api/v1", apiKeyAuth())
	g.GET("/me", v1Me)
	g.GET("/files", v1Files)
	g.GET("/files/:id", v1File)
	g.DELETE("/files/:id", v1Revoke)
	g.PATCH("/files/:id", v1Patch)
}

// apiKeyAuth accepts "Authorization: Bearer <key>" or "X-API-Key: <key>".
func apiKeyAuth() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		key := strings.TrimSpace(ctx.GetHeader("X-API-Key"))
		if key == "" {
			if h := ctx.GetHeader("Authorization"); strings.HasPrefix(strings.ToLower(h), "bearer ") {
				key = strings.TrimSpace(h[7:])
			}
		}
		if key == "" {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing API key"})
			return
		}
		k, ok := store.Default.LookupAPIKey(key)
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			return
		}
		if !apiKeyLimiter.Allow(k.ID, 120, time.Minute) {
			ctx.Header("Retry-After", "60")
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit: 120 requests per minute"})
			return
		}
		if err := commands.UserAllowed(k.OwnerID); err != nil {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		ctx.Set("owner", k.OwnerID)
		ctx.Header("Cache-Control", "no-store")
		ctx.Next()
	}
}

func owner(ctx *gin.Context) int64 { return ctx.GetInt64("owner") }

func v1Me(ctx *gin.Context) {
	uid := owner(ctx)
	ctx.JSON(http.StatusOK, gin.H{"id": uid, "summary": store.Default.SummaryOf(uid)})
}

func v1Files(ctx *gin.Context) {
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "100"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	recs := store.Default.QueryFiles(owner(ctx), ctx.Query("q"), ctx.Query("folder"), limit)
	out := make([]gin.H, 0, len(recs))
	for i := range recs {
		out = append(out, fileJSON(&recs[i]))
	}
	ctx.JSON(http.StatusOK, gin.H{"files": out})
}

func v1Owned(ctx *gin.Context) (store.FileRecord, bool) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		bad(ctx, http.StatusBadRequest, "bad id")
		return store.FileRecord{}, false
	}
	rec, ok := store.Default.GetFile(id)
	if !ok || rec.OwnerID != owner(ctx) {
		bad(ctx, http.StatusNotFound, "link not found")
		return rec, false
	}
	return rec, true
}

func v1File(ctx *gin.Context) {
	if rec, ok := v1Owned(ctx); ok {
		ctx.JSON(http.StatusOK, fileJSON(&rec))
	}
}

// DELETE revokes the link (it stops working; stats stay available).
func v1Revoke(ctx *gin.Context) {
	rec, ok := v1Owned(ctx)
	if !ok {
		return
	}
	store.Default.RevokeFile(rec.MessageID, false)
	ctx.JSON(http.StatusOK, gin.H{"ok": true})
}

// PATCH changes a link's settings (same fields as the My Files page).
func v1Patch(ctx *gin.Context) {
	if rec, ok := v1Owned(ctx); ok {
		patchFileCommon(ctx, rec.MessageID)
	}
}
