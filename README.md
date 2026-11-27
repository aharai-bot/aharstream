<h1 align="center">⚡ Ahar File Streamer</h1>
<p align="center">
  A Telegram bot that turns any file into a <b>fast, direct, streamable link</b> —<br>
  with a web player, expiring links, download stats and a full admin toolkit.
</p>

Built on [TG-FileStreamBot](https://github.com/EverythingSuckz/TG-FileStreamBot) by EverythingSuckz (AGPL-3.0).

## ✨ Features

**For users**
- Send any file / video / audio / photo / document → get a direct link instantly
- Download, Stream and **Open in Web Player** buttons (`/watch/<id>?hash=<hash>`)
- Web player: video, audio, images and PDFs, playback speed, keyboard shortcuts, copy-link button
- Expiring links (`/expiry 24h`, `/expiry 7d`, `/expiry off`) and one-tap **Revoke**
- Download counters and bandwidth stats per link
- Inline-button navigation (Home / Help / My Files / About)
- Slash-command menu is registered automatically

**Inside Telegram**
- **Mini App** – the bot's *Open App* button and menu button open the My Files page and the web player right inside Telegram (needs an `https` `HOST`)
- **Inline mode** – type `@yourbot` in any chat to share one of your links (enable once with @BotFather → `/setinline`)
- **Channel mode** – add the bot to your channel as admin, post `/watchbuttons`, the owner approves, and new media posts get Watch / Download buttons
- **6 languages** for bot replies: English, हिन्दी, தமிழ், తెలుగు, മലയാളം, ಕನ್ನಡ (`/language`)

**Smarter links**
- **Passwords** – `/password <id> <pass>` or the 🔒 button; visitors see an unlock page
- **One-time / limited-view links** – `/views <id> 1`, or the 👁 buttons under a new link
- **Expiry per link** – ⏳ 1h / 1d / 7d / ♾ buttons, or `/linkexpiry <id> 12h`
- **Analytics** – unique views, devices and (when your host/CDN sends a country header) countries per link

**Personal cloud**
- **My Files app** (`/app`) – search, rename, folders, edit, revoke, delete; sign in with the Mini App, `/login`, or the Telegram login button
- **Collections** – group links into one shareable page (`/c/<id>?k=<token>`)
- **Storage summary** – `/storage` and the stats cards in the app

**For developers**
- **Public API with keys** – `/apikey new <name>`, docs at `/docs`
- **Signed webhooks** – `link.created` and `link.viewed` events (`/webhook <https-url>`)
- **One-click deploy** and a **first-run setup wizard** (see below)

**For admins**
- Admin panel with live status, users and top-files views
- Ban / unban users (with reason, user is notified), maintenance mode
- Broadcast to all users (text, or reply to a message)
- Revoke / restore any link, purge expired records
- Add / remove admins at runtime (owner only)

**Under the hood**
- Persistent JSON database (users, links, bans, admins, stats) — no extra services needed
- Per-user rate limit and optional max file size
- Fixed: response headers are now sent before the status line on full downloads
- Safer: HTML/SVG/XML files are never rendered inline from your domain; `nosniff` header; RFC 6266 filenames

## 🤖 Commands

| Command | Who | Description |
|---|---|---|
| `/start` `/help` `/about` `/ping` | everyone | Basics |
| `/myfiles` | everyone | Your 10 latest links with status and download counts |
| `/mystats` | everyone | Your usage statistics |
| `/link` | everyone | Reply to a file to generate a fresh link |
| `/info` | everyone | Reply to a file to see name, type, size, MIME, ID |
| `/expiry <24h\|7d\|off\|default>` | everyone | Default lifetime of your new links |
| `/revoke <id>` | owner of link / admin | Disable a link |
| `/app` · `/login` | everyone | Open My Files (Mini App) · sign in from a browser |
| `/language` | everyone | Choose the bot language |
| `/password <id> <pass\|off>` | link owner | Protect a link with a password (your message is deleted) |
| `/views <id> <n\|off>` | link owner | Limit how many visitors can open a link (1 = one-time) |
| `/linkexpiry <id> <24h\|7d\|off>` | link owner | Expiry of one link |
| `/storage` · `/collections` | everyone | Storage summary · your collections |
| `/apikey [new <name>\|revoke <id>]` | everyone | Manage API keys (max 5) |
| `/webhook <https-url\|off\|test>` | everyone | Event webhook |
| `/cancel` | everyone | Cancel a pending password prompt |
| `/admin` | admin | Admin panel |
| `/status` | admin | Uptime, memory, workers, users, links, downloads, bandwidth |
| `/users` · `/user <id>` | admin | User statistics / details |
| `/ban <id> [reason]` · `/unban <id>` · `/banned` | admin | Moderation |
| `/broadcast <text>` | admin | Message every (non-banned) user |
| `/maintenance on\|off` | admin | Only admins can use the bot while ON |
| `/topfiles` | admin | Most downloaded files |
| `/restore <id>` | admin | Re-enable a revoked link |
| `/workers` | admin | List streaming bots |
| `/purge` | admin | Remove expired link records |
| `/admins` | admin | List owners and admins |
| `/addadmin <id>` · `/rmadmin <id>` | owner | Manage admins |

## 🚀 Deploy

> The buttons need the URL of **your** GitHub repository: replace `YOUR_USER/YOUR_REPO` (they have not been tested from here).

[![Deploy to Heroku](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/YOUR_USER/YOUR_REPO)
[![Deploy to Render](https://render.com/images/deploy-to-render-button.svg)](https://render.com/deploy?repo=https://github.com/YOUR_USER/YOUR_REPO)

- **Render** reads `render.yaml` (Blueprint) – fill in the secrets it asks for.
- **Railway** – *New Project → Deploy from GitHub repo*; it builds the `Dockerfile` (`railway.json`). Add the variables below.
- **Koyeb** – *Create service → GitHub → builder: Dockerfile*, port `8080`.
- **Docker** – `touch fsb.env && docker compose up -d`, then open `http://<server>:8038/?token=<token from the logs>`.
- **Heroku** – uses `app.json` and the `Procfile`.

Free hosts usually have a disk that is wiped on restart: use a persistent volume for `data/` (the database) or your links, users and stats are lost.

### First-run setup wizard

If `API_ID`, `API_HASH`, `BOT_TOKEN` or `LOG_CHANNEL` are missing, the server does not exit. It starts a setup page on `PORT` and prints a line like

```
Setup wizard: open http://<your-server>:8080/?token=3fa91c7e04b2 in a browser.
```

Open that address, fill in the form, and the bot starts right away. The values are saved to `fsb.env` (mode 0600). The token only appears in the server logs, so only someone with log access can use the wizard. On hosts with an ephemeral disk, set the variables in the host's dashboard instead.

## ⚙️ Setup

1. Copy `fsb.sample.env` to `fsb.env` and fill in `API_ID`, `API_HASH`, `BOT_TOKEN`, `LOG_CHANNEL`.
2. **Set `ADMINS=<your numeric Telegram ID>`** to unlock the admin commands.
3. Set `HOST` to your public URL (e.g. `https://files.example.com`).
4. Run:

```bash
go run ./cmd/fsb run
# or
docker compose up -d
```

### New environment variables

| Variable | Default | Description |
|---|---|---|
| `ADMINS` | – | Owner IDs, comma separated |
| `BOT_NAME` | `Ahar File Streamer` | Branding in messages and web player |
| `SUPPORT_URL` | – | Adds a Support button |
| `DB_PATH` | `data/ahar.json` | Database file (mount `./data` in Docker) |
| `DEFAULT_EXPIRY_HOURS` | `0` | Default link lifetime (0 = never) |
| `MAX_LINKS_PER_HOUR` | `60` | Per-user rate limit (admins exempt, 0 = off) |
| `MAX_FILE_SIZE_MB` | `0` | Max size for non-admins (0 = unlimited) |
| `ENABLE_PLAYER` | `true` | Enable the `/watch` web player (needed for passwords) |
| `SESSION_SECRET` | derived from `BOT_TOKEN` | Optional secret that signs web sign-in cookies and link tokens |

All original variables (`MULTI_TOKEN*`, `ALLOWED_USERS`, `STREAM_*`, …) keep working.

### HTTP endpoints

- `GET /` – status JSON · `GET /health` – liveness · `GET /api/stats` – public totals
- `GET /stream/<id>?hash=<hash>[&d=true]` – file stream (Range supported)
- `GET /watch/<id>?hash=<hash>` – web player (shows an unlock page for password-protected links)
- `GET /app` – My Files app · `GET /docs` – API documentation · `GET /c/<id>?k=<token>` – shared collection
- `GET /auth/login?t=…` (from `/login`) · `GET /auth/telegram` (login widget) · `POST /auth/webapp` (Mini App)
- `GET|PATCH|DELETE /api/v1/…` – public API (Bearer key) · `/api/me/…` – the app's own API (session cookie)

### Good to know
- A *view* is one unique visitor per link per 10 minutes. Link previews (Telegram, WhatsApp…) and `HEAD` requests never use up views.
- Countries are only recorded when your host or CDN sends a country header (e.g. Cloudflare `CF-IPCountry`); there is no GeoIP database.
- The in-Telegram Mini App and menu button need `HOST` to be an `https://` URL. The Telegram login button on `/app` also needs `/setdomain` in @BotFather.
- Webhook URLs must be `https` and cannot point to private or local addresses.

## Notes
- Links created before upgrading keep working (they simply have no stats/expiry record).
- Revoked and expired links return `410 Gone`.

## License
GNU AGPL-3.0 — forks must stay open source under the same license. See `LICENSE`.
