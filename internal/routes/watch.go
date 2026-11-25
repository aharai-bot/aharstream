package routes

import (
	"html/template"
	"net/http"
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

var watchTmpl = template.Must(template.New("watch").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Name}} · {{.BotName}}</title>
<style>
:root{--bg:#0b1020;--card:#141b34;--text:#e8ecf8;--muted:#8d97b8;--accent:#6c8cff;--accent2:#a76cff}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;background:radial-gradient(1200px 600px at 20% -10%,#1b2550 0,transparent 60%),radial-gradient(900px 500px at 100% 0,#2b1b50 0,transparent 55%),var(--bg);color:var(--text);font-family:system-ui,-apple-system,Segoe UI,Roboto,sans-serif}
.wrap{max-width:980px;margin:0 auto;padding:24px 16px 48px}
header{display:flex;align-items:center;gap:10px;margin-bottom:18px;color:var(--muted);font-weight:600;letter-spacing:.3px}
header .dot{width:10px;height:10px;border-radius:50%;background:linear-gradient(135deg,var(--accent),var(--accent2))}
.card{background:var(--card);border:1px solid rgba(255,255,255,.06);border-radius:18px;overflow:hidden;box-shadow:0 20px 60px rgba(0,0,0,.4)}
.stage{background:#000;display:flex;justify-content:center;align-items:center;min-height:240px}
video,img{max-width:100%;max-height:70vh;display:block}
video{width:100%}
audio{width:100%;margin:28px}
iframe{width:100%;height:75vh;border:0;background:#fff}
.noprev{padding:48px 20px;text-align:center;color:var(--muted)}
.noprev .big{font-size:56px}
.meta{padding:18px 20px}
h1{font-size:1.1rem;margin:0 0 10px;word-break:break-word}
.chips{display:flex;flex-wrap:wrap;gap:8px;margin-bottom:16px}
.chip{background:rgba(255,255,255,.06);border-radius:999px;padding:5px 12px;font-size:.82rem;color:var(--muted)}
.actions{display:flex;flex-wrap:wrap;gap:10px}
.btn{appearance:none;border:0;cursor:pointer;text-decoration:none;font:inherit;font-weight:600;padding:11px 18px;border-radius:12px;color:#fff;background:linear-gradient(135deg,var(--accent),var(--accent2));transition:transform .12s,opacity .12s}
.btn:hover{transform:translateY(-1px)}
.btn.alt{background:rgba(255,255,255,.08)}
.speed{display:none;gap:6px;align-items:center;color:var(--muted);font-size:.85rem}
.speed select{background:rgba(255,255,255,.08);color:var(--text);border:0;border-radius:8px;padding:8px}
footer{text-align:center;color:var(--muted);font-size:.8rem;margin-top:22px}
</style>
</head>
<body>
<div class="wrap">
<header><span class="dot"></span>{{.BotName}}</header>
<div class="card">
<div class="stage">
{{if eq .Kind "video"}}<video id="media" controls autoplay playsinline preload="metadata" src="{{.StreamURL}}"></video>
{{else if eq .Kind "audio"}}<audio id="media" controls autoplay preload="metadata" src="{{.StreamURL}}"></audio>
{{else if eq .Kind "image"}}<img src="{{.StreamURL}}" alt="{{.Name}}">
{{else if eq .Kind "pdf"}}<iframe src="{{.StreamURL}}"></iframe>
{{else}}<div class="noprev"><div class="big">📦</div><p>No preview available for this file type.<br>You can still download it below.</p></div>{{end}}
</div>
<div class="meta">
<h1>{{.Name}}</h1>
<div class="chips">
<span class="chip">{{.KindLabel}}</span>
<span class="chip">{{.Size}}</span>
<span class="chip">⬇️ {{.Downloads}} downloads</span>
<span class="chip">⏳ {{.Expires}}</span>
</div>
<div class="actions">
<a class="btn" href="{{.DownloadURL}}">⬇️ Download</a>
<button class="btn alt" id="copy">🔗 Copy link</button>
<label class="speed" id="speedBox">Speed
<select id="speed"><option>0.5</option><option>0.75</option><option selected>1</option><option>1.25</option><option>1.5</option><option>2</option></select></label>
</div>
</div>
</div>
<footer>Powered by {{.BotName}}</footer>
</div>
<script>
(function(){
var media=document.getElementById('media');
if(media){
 var box=document.getElementById('speedBox');box.style.display='flex';
 document.getElementById('speed').addEventListener('change',function(e){media.playbackRate=parseFloat(e.target.value)});
 document.addEventListener('keydown',function(e){
  if(e.target.tagName==='SELECT')return;
  if(e.key===' '){e.preventDefault();media.paused?media.play():media.pause()}
  else if(e.key==='ArrowRight'){media.currentTime+=10}
  else if(e.key==='ArrowLeft'){media.currentTime-=10}
  else if(e.key==='f'&&media.requestFullscreen){media.requestFullscreen()}
 });
}
document.getElementById('copy').addEventListener('click',function(){
 var u=location.origin+{{.StreamURL}};
 var b=this;
 (navigator.clipboard?navigator.clipboard.writeText(u):Promise.reject()).then(function(){b.textContent='✅ Copied'},function(){prompt('Copy this link',u)});
});
})();
</script>
</body>
</html>`))
