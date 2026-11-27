<div align="center">

<img src="https://readme-typing-svg.demolab.com?font=Fira+Code&weight=700&size=34&duration=3000&pause=1000&color=2AABEE&center=true&vCenter=true&width=640&lines=%E2%9A%A1+Ahar+File+Streamer;Telegram+files+%E2%86%92+direct+links;Web+player+%C2%B7+Mini+App+%C2%B7+API" alt="Ahar File Streamer" />

<p>
  A Telegram bot that turns any file into a <b>fast, direct, streamable link</b><br>
  with a web player, Mini App, expiring links, passwords, an API and a full admin toolkit.
</p>

<p>
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Docker-ghcr.io-2496ED?style=flat-square&logo=docker&logoColor=white" alt="Docker">
  <img src="https://img.shields.io/badge/Telegram-Mini_App-26A5E4?style=flat-square&logo=telegram&logoColor=white" alt="Telegram">
  <img src="https://img.shields.io/badge/License-AGPL--3.0-green?style=flat-square" alt="License">
</p>

<p>
  <a href="#-features">Features</a> ·
  <a href="#-deploy">Deploy</a> ·
  <a href="#-docker-package">Docker package</a> ·
  <a href="#-environment-variables">Env variables</a> ·
  <a href="#-commands">Commands</a> ·
  <a href="#-http-endpoints">Endpoints</a>
</p>

<b>Remodified by Ahar Developers</b>

</div>

Built on [TG-FileStreamBot](https://github.com/EverythingSuckz/TG-FileStreamBot) by EverythingSuckz (AGPL-3.0).
Repository: <https://github.com/aharai-bot/aharstream>

---

## ✨ Features

<table>
<tr>
<td width="50%" valign="top">

### 👤 For users
- Send any file / video / audio / photo / document → get a direct link instantly
- **Download**, **Stream** and **Open in Web Player** buttons (`/watch/<id>?hash=<hash>`)
- Web player: video, audio, images and PDFs, playback speed, keyboard shortcuts, copy-link button
- Expiring links (`/expiry 24h`, `/expiry 7d`, `/expiry off`) and one-tap **Revoke**
- Download counters and bandwidth stats per link
- Inline-button navigation (Home / Help / My Files / About)
- Slash-command menu is registered automatically

</td>
<td width="50%" valign="top">

### 💬 Inside Telegram
- **Mini App**: the *Open App* and menu buttons open My Files and the web player inside Telegram (needs an `https` `HOST`)
- **Inline mode**: type `@yourbot` in any chat to share a link (enable once with @BotFather → `/setinline`)
- **Channel mode**: add the bot as channel admin, post `/watchbuttons`, the owner approves, and new media posts get Watch / Download buttons
- **6 languages**: English, हिन्दी, தமிழ், తెలుగు, മലയാളം, ಕನ್ನಡ (`/language`)

</td>
</tr>
<tr>
<td width="50%" valign="top">

### 🔗 Smarter links
- **Passwords**: `/password <id> <pass>` or the 🔒 button; visitors see an unlock page
- **One-time / limited views**: `/views <id> 1`, or the 👁 buttons under a new link
- **Expiry per link**: ⏳ 1h / 1d / 7d / ♾ buttons, or `/linkexpiry <id> 12h`
- **Analytics**: unique views, devices and (when your host/CDN sends a country header) countries

</td>
<td width="50%" valign="top">

### ☁️ Personal cloud
- **My Files app** (`/app`): search, rename, folders, edit, revoke, delete; sign in with the Mini App, `/login`, or the Telegram login button
- **Collections**: group links into one shareable page (`/c/<id>?k=<token>`)
- **Storage summary**: `/storage` and the stats cards in the app

</td>
</tr>
<tr>
<td width="50%" valign="top">

### 🧑‍💻 For developers
- **Public API with keys**: `/apikey new <name>`, docs at `/docs`
- **Signed webhooks**: `link.created` and `link.viewed` events (`/webhook <https-url>`)
- **Docker package** on GitHub Packages and a **first-run setup wizard**

</td>
<td width="50%" valign="top">

### 🛡️ For admins
- Admin panel with live status, users and top-files views
- Ban / unban users (with reason, user is notified), maintenance mode
- Broadcast to all users (text, or reply to a message)
- Revoke / restore any link, purge expired records
- Add / remove admins at runtime (owner only)

</td>
</tr>
</table>

**⚙️ Under the hood**
- Persistent JSON database (users, links, bans, admins, stats), no extra services needed
- Per-user rate limit and optional max file size
- Response headers are sent before the status line on full downloads (fixed)
- HTML/SVG/XML files are never rendered inline from your domain; `nosniff` header; RFC 6266 filenames
- Multi-arch image (`linux/amd64`, `linux/arm64`) built automatically on every push

---

## 🚀 Deploy

<p align="center">
  <a href="https://heroku.com/deploy?template=https://github.com/aharai-bot/aharstream"><img height="44" src="https://img.shields.io/badge/DEPLOY%20TO-HEROKU-430098?style=for-the-badge&logo=heroku&logoColor=white" alt="Deploy to Heroku"></a>
  <a href="https://render.com/deploy?repo=https://github.com/aharai-bot/aharstream"><img height="44" src="https://img.shields.io/badge/DEPLOY%20TO-RENDER-000000?style=for-the-badge&logo=render&logoColor=white" alt="Deploy to Render"></a>
  <a href="https://app.koyeb.com/deploy?name=aharstream&type=git&repository=github.com/aharai-bot/aharstream&branch=main&builder=dockerfile&ports=8080;http;/&env[PORT]=8080&env[API_ID]=&env[API_HASH]=&env[BOT_TOKEN]=&env[LOG_CHANNEL]=&env[ADMINS]=&env[HOST]="><img height="44" src="https://img.shields.io/badge/DEPLOY%20TO-KOYEB-121212?style=for-the-badge&logo=koyeb&logoColor=white" alt="Deploy to Koyeb"></a>
  <a href="#-docker-package"><img height="44" src="https://img.shields.io/badge/DEPLOY%20WITH-DOCKER-2496ED?style=for-the-badge&logo=docker&logoColor=white" alt="Deploy with Docker"></a>
</p>

> [!NOTE]
> The deploy buttons and steps below have not been tested end to end. If a button does not open the right page, use the manual steps next to it.

| Host | How |
|---|---|
| **Heroku** | Use the button above. It reads `app.json` and the `Procfile`. |
| **Render** | Use the button above. It reads `render.yaml` (Blueprint); fill in the secrets it asks for. |
| **Koyeb** | Use the button above, or *Create service → GitHub → `aharai-bot/aharstream` → builder: Dockerfile*, port `8080`. Or deploy the [Docker package](#-docker-package) directly. |
| **Railway** | *New Project → Deploy from GitHub repo → `aharai-bot/aharstream`*. It builds the `Dockerfile` (`railway.json`). Add the variables below, then *Settings → Networking → Generate domain* and put that URL in `HOST`. |
| **Fly.io** | `git clone https://github.com/aharai-bot/aharstream && cd aharstream`<br>`fly launch` (detects the `Dockerfile`; internal port `8080`)<br>`fly volumes create ahar_data --size 1`, mount it at `/app/data` in `fly.toml`<br>`fly secrets set API_ID=… API_HASH=… BOT_TOKEN=… LOG_CHANNEL=… ADMINS=… HOST=https://<app>.fly.dev`<br>`fly deploy` |
| **runsite.app** | New service from the GitHub repo `aharai-bot/aharstream`, `Dockerfile` build, port `8080`, add the variables below. (No one-click button format confirmed; use their dashboard.) |
| **Any Docker host** | Run the [Docker package](#-docker-package), no build needed. |

> [!IMPORTANT]
> Free hosts usually wipe the disk on restart. Use a persistent volume for `data/` (the database) or your links, users and stats are lost.

---

## 🐳 Docker package

Every push to `main` (and every `v*` tag) publishes a multi-arch image to GitHub Packages:

```
ghcr.io/aharai-bot/fsb:latest
```

> [!TIP]
> If `docker pull` says *unauthorized*, open the package on GitHub (*Packages → fsb → Package settings → Change visibility*) and make it **Public**.

### Option 1: `docker run` (direct host, no clone)

```bash
docker run -d --name ahar-streamer --restart unless-stopped \
  -p 8080:8080 \
  -e API_ID=1234567 \
  -e API_HASH=your_api_hash \
  -e BOT_TOKEN=123456:ABC-your-bot-token \
  -e LOG_CHANNEL=-1001234567890 \
  -e ADMINS=123456789 \
  -e HOST=https://files.example.com \
  -v ahar-data:/app/data \
  ghcr.io/aharai-bot/fsb:latest
```

### Option 2: Docker Compose

```bash
git clone https://github.com/aharai-bot/aharstream && cd aharstream
cp fsb.sample.env fsb.env        # fill in the variables (or `touch fsb.env` to use the setup wizard)
docker compose up -d
```

`docker-compose.yaml` pulls `ghcr.io/aharai-bot/fsb:latest` and keeps `./data` and `./logs` on the host. The app listens on `8080` inside the container; change the host port with `PORT=9000 docker compose up -d`.

### Option 3: Compose file without cloning

```yaml
services:
  ahar-streamer:
    image: ghcr.io/aharai-bot/fsb:latest
    restart: always
    ports: ["8080:8080"]
    environment:
      API_ID: "1234567"
      API_HASH: "your_api_hash"
      BOT_TOKEN: "123456:ABC-your-bot-token"
      LOG_CHANNEL: "-1001234567890"
      ADMINS: "123456789"
      HOST: "https://files.example.com"
    volumes:
      - ./data:/app/data
```

### Option 4: Koyeb straight from the image

[Deploy ghcr.io/aharai-bot/fsb on Koyeb](https://app.koyeb.com/deploy?name=aharstream&type=docker&image=ghcr.io/aharai-bot/fsb:latest&ports=8080;http;/&env[PORT]=8080&env[API_ID]=&env[API_HASH]=&env[BOT_TOKEN]=&env[LOG_CHANNEL]=&env[ADMINS]=&env[HOST]=) · the same image also works on Fly.io (`fly launch --image ghcr.io/aharai-bot/fsb:latest`), Railway, Render and any VPS.

### Update

```bash
docker pull ghcr.io/aharai-bot/fsb:latest
docker compose up -d          # or recreate your `docker run` container
```

### First-run setup wizard

If `API_ID`, `API_HASH`, `BOT_TOKEN` or `LOG_CHANNEL` are missing, the server does not exit. It starts a setup page on `PORT` and prints a line like

```
Setup wizard: open http://<your-server>:8080/?token=3fa91c7e04b2 in a browser.
```

Open that address, fill in the form, and the bot starts right away. The values are saved to `fsb.env` (mode 0600), so mount that file (`-v ./fsb.env:/app/fsb.env`) if you want them to survive a restart. The token only appears in the server logs. On hosts with an ephemeral disk, set the variables in the host's dashboard instead.

---

## 🔧 Environment variables

Set them in `fsb.env`, in your host's dashboard, or with `-e` in Docker. `fsb.sample.env` is a ready template.

### Required

| Variable | Description |
|---|---|
| `API_ID` | Telegram API ID from <https://my.telegram.org> |
| `API_HASH` | Telegram API hash from <https://my.telegram.org> |
| `BOT_TOKEN` | Bot token from [@BotFather](https://t.me/BotFather) |
| `LOG_CHANNEL` | ID of the channel where the bot stores your files (e.g. `-1001234567890`); the bot must be an admin there |

### Server

| Variable | Default | Description |
|---|---|---|
| `HOST` | auto (`http://<ip>:<PORT>`) | Public URL used in links. Use an `https://` URL for the Mini App and menu button |
| `PORT` | `8080` | Port the web server listens on |
| `HASH_LENGTH` | `6` | Length of the hash in links (5 to 32) |
| `USE_PUBLIC_IP` | `false` | Use the public IP when `HOST` is not set (not recommended) |
| `USE_SESSION_FILE` | `true` | Reuse worker sessions from files for faster startup |
| `USER_SESSION` | – | Optional user session string for userbot features |
| `DEV` | `false` | Development mode |
| `ALLOWED_USERS` | – | Allowlist of Telegram user IDs, comma separated (empty = everyone) |
| `SESSION_SECRET` | derived from `BOT_TOKEN` | Secret that signs web sign-in cookies and link tokens |

### Ahar File Streamer

| Variable | Default | Description |
|---|---|---|
| `ADMINS` | – | Owner IDs, comma separated (unlocks admin commands) |
| `BOT_NAME` | `Ahar File Streamer` | Branding in messages and the web player |
| `SUPPORT_URL` | – | Adds a Support button |
| `DB_PATH` | `data/ahar.json` | Database file (mount `./data` in Docker) |
| `DEFAULT_EXPIRY_HOURS` | `0` | Default link lifetime in hours (0 = never) |
| `MAX_LINKS_PER_HOUR` | `60` | Per-user rate limit (admins exempt, 0 = off) |
| `MAX_FILE_SIZE_MB` | `0` | Max size for non-admins (0 = unlimited) |
| `ENABLE_PLAYER` | `true` | Enable the `/watch` web player (needed for passwords) |

### Streaming performance

| Variable | Default | Description |
|---|---|---|
| `STREAM_CONCURRENCY` | `4` | Parallel Telegram chunk downloads per stream request |
| `STREAM_BUFFER_COUNT` | `8` | Chunks kept ready in memory |
| `STREAM_TIMEOUT_SEC` | `30` | Per-chunk timeout in seconds |
| `STREAM_MAX_RETRIES` | `3` | Retries per failed chunk |

### Multiple bots (faster streaming)

| Variable | Description |
|---|---|
| `MULTI_TOKEN1`, `MULTI_TOKEN2`, … | Extra bot tokens used as workers to spread the load. Add each bot to `LOG_CHANNEL` as admin |

---

## 🛠️ Run from source

```bash
cp fsb.sample.env fsb.env      # fill in API_ID, API_HASH, BOT_TOKEN, LOG_CHANNEL, ADMINS, HOST
go run ./cmd/fsb run
```

---

## 🤖 Commands

| Command | Who | Description |
|---|---|---|
| `/start` `/help` `/about` `/ping` | everyone | Basics |
| `/myfiles` | everyone | Your 10 latest links with status and download counts |
| `/mystats` | everyone | Your usage statistics |
| `/link` | everyone | Reply to a file to generate a fresh link |
| `/info` | everyone | Reply to a file to see name, type, size, MIME, ID |
| `/expiry <24h\|7d\|off\|default>` | everyone | Default lifetime of your new links |
| `/revoke <id>` | link owner / admin | Disable a link |
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

---

## 🌐 HTTP endpoints

- `GET /` home page in a browser, status JSON for API clients · `GET /health` liveness · `GET /api/stats` public totals
- `GET /stream/<id>?hash=<hash>[&d=true]` file stream (Range supported)
- `GET /watch/<id>?hash=<hash>` web player (unlock page for password-protected links)
- `GET /app` My Files app · `GET /docs` API documentation · `GET /c/<id>?k=<token>` shared collection
- `GET /auth/login?t=…` (from `/login`) · `GET /auth/telegram` (login widget) · `POST /auth/webapp` (Mini App)
- `GET|PATCH|DELETE /api/v1/…` public API (Bearer key) · `/api/me/…` the app's own API (session cookie)

## 💡 Good to know

- A *view* is one unique visitor per link per 10 minutes. Link previews (Telegram, WhatsApp…) and `HEAD` requests never use up views.
- Countries are only recorded when your host or CDN sends a country header (e.g. Cloudflare `CF-IPCountry`); there is no GeoIP database.
- The Mini App and menu button need `HOST` to be an `https://` URL. The Telegram login button on `/app` also needs `/setdomain` in @BotFather.
- Webhook URLs must be `https` and cannot point to private or local addresses.
- Links created before upgrading keep working (they simply have no stats/expiry record).
- Revoked and expired links return `410 Gone`.

---

## 📄 License

GNU AGPL-3.0. Forks must stay open source under the same license. See [`LICENSE`](LICENSE).

<p align="center"><b>Remodified by Ahar Developers</b> · <a href="https://github.com/aharai-bot/aharstream">github.com/aharai-bot/aharstream</a></p>
