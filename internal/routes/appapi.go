package routes

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/auth"
	"EverythingSuckz/fsb/internal/commands"
	"EverythingSuckz/fsb/internal/i18n"
	"EverythingSuckz/fsb/internal/store"
	"EverythingSuckz/fsb/internal/utils"
	"EverythingSuckz/fsb/internal/webhook"

	"github.com/gin-gonic/gin"
)

func (e *allRoutes) LoadAppAPI(r *Route) {
	g := r.Engine
	g.GET("/api/me", withUser(apiMe))
	g.GET("/api/me/files", withUser(apiFiles))
	g.PATCH("/api/me/files/:id", withUser(apiPatchFile))
	g.DELETE("/api/me/files/:id", withUser(apiDeleteFile))
	g.GET("/api/me/collections", withUser(apiCollections))
	g.POST("/api/me/collections", withUser(apiCreateCollection))
	g.PATCH("/api/me/collections/:cid", withUser(apiPatchCollection))
	g.DELETE("/api/me/collections/:cid", withUser(apiDeleteCollection))
	g.GET("/api/me/keys", withUser(apiKeys))
	g.POST("/api/me/keys", withUser(apiCreateKey))
	g.DELETE("/api/me/keys/:kid", withUser(apiDeleteKey))
	g.PUT("/api/me/webhook", withUser(apiSetWebhook))
	g.POST("/api/me/webhook/test", withUser(apiTestWebhook))
	g.PUT("/api/me/lang", withUser(apiSetLang))
}

func bad(ctx *gin.Context, status int, msg string) {
	ctx.JSON(status, gin.H{"error": msg})
}

func collectionJSON(c *store.Collection) gin.H {
	return gin.H{
		"id":         c.ID,
		"title":      c.Title,
		"files":      c.Files,
		"created_at": c.CreatedAt,
		"revoked":    c.Revoked,
		"url":        strings.TrimRight(config.ValueOf.Host, "/") + "/c/" + c.ID + "?k=" + c.Token,
	}
}

func keyJSON(k store.APIKey) gin.H {
	return gin.H{"id": k.ID, "name": k.Name, "prefix": k.Prefix, "created_at": k.CreatedAt, "last_used": k.LastUsed}
}

func apiMe(ctx *gin.Context, uid int64) {
	usr, _ := store.Default.GetUser(uid)
	keys := []gin.H{}
	for _, k := range store.Default.APIKeysOf(uid) {
		keys = append(keys, keyJSON(k))
	}
	langs := []gin.H{}
	for _, l := range i18n.Langs {
		langs = append(langs, gin.H{"code": l.Code, "name": l.Name})
	}
	lang := usr.Lang
	if !i18n.Valid(lang) {
		lang = "en"
	}
	ctx.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":         uid,
			"first_name": usr.FirstName,
			"username":   usr.Username,
			"lang":       lang,
			"is_admin":   commands.IsAdmin(uid),
		},
		"summary":   store.Default.SummaryOf(uid),
		"bot":       botUsername(),
		"bot_name":  config.ValueOf.BotName,
		"player":    config.ValueOf.EnablePlayer,
		"languages": langs,
		"keys":      keys,
		"webhook":   gin.H{"url": usr.WebhookURL, "secret": usr.WebhookSecret},
		"host":      strings.TrimRight(config.ValueOf.Host, "/"),
	})
}

func apiFiles(ctx *gin.Context, uid int64) {
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "500"))
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	recs := store.Default.QueryFiles(uid, ctx.Query("q"), ctx.Query("folder"), limit)
	out := make([]gin.H, 0, len(recs))
	for i := range recs {
		out = append(out, fileJSON(&recs[i]))
	}
	ctx.JSON(http.StatusOK, gin.H{"files": out, "folders": store.Default.SummaryOf(uid).Folders})
}

func ownedFile(ctx *gin.Context, uid int64) (int, bool) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		bad(ctx, http.StatusBadRequest, "bad id")
		return 0, false
	}
	rec, ok := store.Default.GetFile(id)
	if !ok || (rec.OwnerID != uid && !commands.IsAdmin(uid)) {
		bad(ctx, http.StatusNotFound, "link not found")
		return 0, false
	}
	return id, true
}

type patchFile struct {
	Name          *string `json:"name"`
	Folder        *string `json:"folder"`
	ExpiryHours   *int    `json:"expiry_hours"` // -1 = never, >0 = hours from now
	Password      *string `json:"password"`     // non-empty sets a password
	ClearPassword bool    `json:"clear_password"`
	MaxViews      *int    `json:"max_views"` // 0 = unlimited
	Revoked       *bool   `json:"revoked"`
}

func apiPatchFile(ctx *gin.Context, uid int64) {
	id, ok := ownedFile(ctx, uid)
	if !ok {
		return
	}
	patchFileCommon(ctx, id)
}

// patchFileCommon applies a JSON patch to a link the caller is allowed to change.
func patchFileCommon(ctx *gin.Context, id int) {
	var in patchFile
	if err := ctx.ShouldBindJSON(&in); err != nil {
		bad(ctx, http.StatusBadRequest, "invalid JSON")
		return
	}
	var passHash string
	if in.Password != nil && *in.Password != "" {
		if !config.ValueOf.EnablePlayer {
			bad(ctx, http.StatusBadRequest, "passwords need the web player, which is disabled on this server")
			return
		}
		if len([]rune(*in.Password)) < 4 || len(*in.Password) > 100 {
			bad(ctx, http.StatusBadRequest, "password must be 4-100 characters")
			return
		}
		h, err := auth.HashPassword(*in.Password)
		if err != nil {
			bad(ctx, http.StatusInternalServerError, "could not hash the password")
			return
		}
		passHash = h
	}
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" || len([]rune(n)) > 200 {
			bad(ctx, http.StatusBadRequest, "name must be 1-200 characters")
			return
		}
		in.Name = &n
	}
	if in.Folder != nil {
		f := strings.TrimSpace(*in.Folder)
		if len([]rune(f)) > 40 {
			bad(ctx, http.StatusBadRequest, "folder name is too long")
			return
		}
		in.Folder = &f
	}
	if in.MaxViews != nil && (*in.MaxViews < 0 || *in.MaxViews > 1000000) {
		bad(ctx, http.StatusBadRequest, "max_views must be between 0 and 1000000")
		return
	}
	if in.ExpiryHours != nil && (*in.ExpiryHours == 0 || *in.ExpiryHours < -1 || *in.ExpiryHours > 24*365) {
		bad(ctx, http.StatusBadRequest, "expiry_hours must be -1 (never) or 1-8760")
		return
	}
	store.Default.UpdateFile(id, func(f *store.FileRecord) {
		if in.Name != nil {
			f.Name = *in.Name
		}
		if in.Folder != nil {
			f.Folder = *in.Folder
		}
		if in.ExpiryHours != nil {
			if *in.ExpiryHours < 0 {
				f.ExpiresAt = 0
			} else {
				f.ExpiresAt = time.Now().Add(time.Duration(*in.ExpiryHours) * time.Hour).Unix()
			}
		}
		if passHash != "" {
			f.PassHash = passHash
		} else if in.ClearPassword {
			f.PassHash = ""
		}
		if in.MaxViews != nil {
			f.MaxViews = *in.MaxViews
			if *in.MaxViews > 0 {
				f.Views = 0
			}
		}
		if in.Revoked != nil {
			f.Revoked = *in.Revoked
		}
	})
	rec, _ := store.Default.GetFile(id)
	ctx.JSON(http.StatusOK, fileJSON(&rec))
}

func apiDeleteFile(ctx *gin.Context, uid int64) {
	id, ok := ownedFile(ctx, uid)
	if !ok {
		return
	}
	store.Default.DeleteFile(id)
	ctx.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- collections ----------

func apiCollections(ctx *gin.Context, uid int64) {
	out := []gin.H{}
	for _, c := range store.Default.CollectionsOf(uid) {
		cc := c
		out = append(out, collectionJSON(&cc))
	}
	ctx.JSON(http.StatusOK, gin.H{"collections": out})
}

type collectionBody struct {
	Title   *string `json:"title"`
	Files   *[]int  `json:"files"`
	Revoked *bool   `json:"revoked"`
}

func cleanTitle(s string) (string, bool) {
	s = strings.TrimSpace(s)
	return s, s != "" && len([]rune(s)) <= 80
}

func apiCreateCollection(ctx *gin.Context, uid int64) {
	var in collectionBody
	if err := ctx.ShouldBindJSON(&in); err != nil || in.Title == nil {
		bad(ctx, http.StatusBadRequest, "title is required")
		return
	}
	title, ok := cleanTitle(*in.Title)
	if !ok {
		bad(ctx, http.StatusBadRequest, "title must be 1-80 characters")
		return
	}
	if len(store.Default.CollectionsOf(uid)) >= 50 {
		bad(ctx, http.StatusBadRequest, "you can have up to 50 collections")
		return
	}
	var files []int
	if in.Files != nil {
		files = *in.Files
	}
	c := store.Default.CreateCollection(uid, title, files)
	ctx.JSON(http.StatusOK, collectionJSON(c))
}

func apiPatchCollection(ctx *gin.Context, uid int64) {
	var in collectionBody
	if err := ctx.ShouldBindJSON(&in); err != nil {
		bad(ctx, http.StatusBadRequest, "invalid JSON")
		return
	}
	title := ""
	if in.Title != nil {
		t, ok := cleanTitle(*in.Title)
		if !ok {
			bad(ctx, http.StatusBadRequest, "title must be 1-80 characters")
			return
		}
		title = t
	}
	id := ctx.Param("cid")
	ok := store.Default.UpdateCollection(uid, id, func(c *store.Collection, files map[int]*store.FileRecord) {
		if in.Title != nil {
			c.Title = title
		}
		if in.Files != nil {
			var keep []int
			seen := map[int]bool{}
			for _, fid := range *in.Files {
				if f, ok := files[fid]; ok && f.OwnerID == uid && !seen[fid] {
					keep = append(keep, fid)
					seen[fid] = true
				}
			}
			c.Files = keep
		}
		if in.Revoked != nil {
			c.Revoked = *in.Revoked
		}
	})
	if !ok {
		bad(ctx, http.StatusNotFound, "collection not found")
		return
	}
	c, _ := store.Default.GetCollection(id)
	ctx.JSON(http.StatusOK, collectionJSON(&c))
}

func apiDeleteCollection(ctx *gin.Context, uid int64) {
	if !store.Default.DeleteCollection(uid, ctx.Param("cid")) {
		bad(ctx, http.StatusNotFound, "collection not found")
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- API keys ----------

func apiKeys(ctx *gin.Context, uid int64) {
	out := []gin.H{}
	for _, k := range store.Default.APIKeysOf(uid) {
		out = append(out, keyJSON(k))
	}
	ctx.JSON(http.StatusOK, gin.H{"keys": out})
}

func apiCreateKey(ctx *gin.Context, uid int64) {
	var in struct {
		Name string `json:"name"`
	}
	_ = ctx.ShouldBindJSON(&in)
	name := utils.ShortName(strings.TrimSpace(in.Name), 30)
	if in.Name == "" {
		name = "key"
	}
	if len(store.Default.APIKeysOf(uid)) >= 5 {
		bad(ctx, http.StatusBadRequest, "you can have up to 5 API keys")
		return
	}
	plain, k := store.Default.CreateAPIKey(uid, name)
	out := keyJSON(k)
	out["key"] = plain // shown once
	ctx.JSON(http.StatusOK, out)
}

func apiDeleteKey(ctx *gin.Context, uid int64) {
	if !store.Default.DeleteAPIKey(uid, ctx.Param("kid")) {
		bad(ctx, http.StatusNotFound, "key not found")
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------- webhook & language ----------

func apiSetWebhook(ctx *gin.Context, uid int64) {
	var in struct {
		URL string `json:"url"`
	}
	if err := ctx.ShouldBindJSON(&in); err != nil {
		bad(ctx, http.StatusBadRequest, "invalid JSON")
		return
	}
	in.URL = strings.TrimSpace(in.URL)
	if in.URL == "" {
		store.Default.SetWebhook(uid, "", "")
		ctx.JSON(http.StatusOK, gin.H{"url": "", "secret": ""})
		return
	}
	if err := webhook.ValidateURL(in.URL); err != nil {
		bad(ctx, http.StatusBadRequest, err.Error())
		return
	}
	usr, _ := store.Default.GetUser(uid)
	secret := usr.WebhookSecret
	if secret == "" {
		secret = "whsec_" + store.RandomToken(16)
	}
	store.Default.SetWebhook(uid, in.URL, secret)
	ctx.JSON(http.StatusOK, gin.H{"url": in.URL, "secret": secret})
}

var webhookTestLimiter = auth.NewLimiter()

func apiTestWebhook(ctx *gin.Context, uid int64) {
	usr, _ := store.Default.GetUser(uid)
	if usr.WebhookURL == "" {
		bad(ctx, http.StatusBadRequest, "set a webhook URL first")
		return
	}
	if !webhookTestLimiter.Allow(strconv.FormatInt(uid, 10), 5, time.Minute) {
		bad(ctx, http.StatusTooManyRequests, "slow down")
		return
	}
	if err := webhook.Send(usr.WebhookURL, usr.WebhookSecret, "ping", gin.H{"message": "Hello from " + config.ValueOf.BotName}); err != nil {
		bad(ctx, http.StatusBadGateway, "delivery failed: "+err.Error())
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"ok": true})
}

func apiSetLang(ctx *gin.Context, uid int64) {
	var in struct {
		Lang string `json:"lang"`
	}
	if err := ctx.ShouldBindJSON(&in); err != nil || !i18n.Valid(in.Lang) {
		bad(ctx, http.StatusBadRequest, "unsupported language")
		return
	}
	store.Default.SetLang(uid, in.Lang)
	ctx.JSON(http.StatusOK, gin.H{"lang": in.Lang})
}
