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
| `ENABLE_PLAYER` | `true` | Enable the `/watch` web player |

All original variables (`MULTI_TOKEN*`, `ALLOWED_USERS`, `STREAM_*`, …) keep working.

### HTTP endpoints

- `GET /` – status JSON · `GET /health` – liveness · `GET /api/stats` – public totals
- `GET /stream/<id>?hash=<hash>[&d=true]` – file stream (Range supported)
- `GET /watch/<id>?hash=<hash>` – web player

## Notes
- Links created before upgrading keep working (they simply have no stats/expiry record).
- Revoked and expired links return `410 Gone`.

## License
GNU AGPL-3.0 — forks must stay open source under the same license. See `LICENSE`.
