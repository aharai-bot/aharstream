package commands

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/store"
	"EverythingSuckz/fsb/internal/utils"

	"github.com/celestix/gotgproto/dispatcher"
	"github.com/celestix/gotgproto/dispatcher/handlers"
	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
)

func (m *command) LoadAdmin(d dispatcher.Dispatcher) {
	log := m.log.Named("admin")
	defer log.Sugar().Info("Loaded")
	d.AddHandler(handlers.NewCommand("admin", adminPanel))
	d.AddHandler(handlers.NewCommand("status", status))
	d.AddHandler(handlers.NewCommand("users", users))
	d.AddHandler(handlers.NewCommand("user", userInfo))
	d.AddHandler(handlers.NewCommand("ban", ban))
	d.AddHandler(handlers.NewCommand("unban", unban))
	d.AddHandler(handlers.NewCommand("banned", banned))
	d.AddHandler(handlers.NewCommand("broadcast", broadcast))
	d.AddHandler(handlers.NewCommand("maintenance", maintenance))
	d.AddHandler(handlers.NewCommand("topfiles", topFiles))
	d.AddHandler(handlers.NewCommand("restore", restore))
	d.AddHandler(handlers.NewCommand("workers", workers))
	d.AddHandler(handlers.NewCommand("purge", purge))
	d.AddHandler(handlers.NewCommand("admins", admins))
	d.AddHandler(handlers.NewCommand("addadmin", addAdmin))
	d.AddHandler(handlers.NewCommand("rmadmin", rmAdmin))
}

// ---------- panel ----------

func adminPanelText() []part {
	total, bn := store.Default.UserCount()
	st, files := store.Default.GetStats()
	return parts(
		b("🛡 Admin panel — "+config.ValueOf.BotName), nl(), nl(),
		b("Users: "), c(fmt.Sprint(total)), p(" ("), c(fmt.Sprint(bn)), p(" banned)"), nl(),
		b("Links: "), c(fmt.Sprint(files)), nl(),
		b("Downloads: "), c(fmt.Sprint(st.TotalDownloads)), nl(),
		b("Maintenance: "), p(onOff(store.Default.Maintenance())), nl(), nl(),
		i("Send /help to see every admin command."),
	)
}

func adminPanelKeyboard() *tg.ReplyInlineMarkup {
	return kb(
		[]tg.KeyboardButtonClass{btnCB("📊 Status", "ahar:adm:status"), btnCB("👥 Users", "ahar:adm:users")},
		[]tg.KeyboardButtonClass{btnCB("🏆 Top files", "ahar:adm:top"), btnCB("🏠 Home", "ahar:home")},
	)
}

func onOff(v bool) string {
	if v {
		return "ON 🛠"
	}
	return "OFF ✅"
}

func adminPanel(ctx *ext.Context, u *ext.Update) error {
	if _, ok := adminGate(ctx, u); !ok {
		return endGroups()
	}
	replyKb(ctx, u, adminPanelKeyboard(), adminPanelText()...)
	return endGroups()
}

// ---------- status ----------

func statusParts() []part {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	st, files := store.Default.GetStats()
	total, bn := store.Default.UserCount()
	active24 := store.Default.ActiveSince(time.Now().Add(-24 * time.Hour).Unix())
	nWorkers := 0
	if WorkersInfo != nil {
		nWorkers = len(WorkersInfo())
	}
	return parts(
		b("📊 System status"), nl(), nl(),
		b("Version: "), c(utils.Version), nl(),
		b("Uptime: "), c(utils.TimeFormat(uint64(time.Since(StartTime).Seconds()))), nl(),
		b("Memory: "), c(utils.HumanSize(int64(ms.Alloc))), p(" in use / "), c(utils.HumanSize(int64(ms.Sys))), p(" reserved"), nl(),
		b("Goroutines: "), c(fmt.Sprint(runtime.NumGoroutine())), nl(),
		b("CPU cores: "), c(fmt.Sprint(runtime.NumCPU())), nl(),
		b("Streaming bots: "), c(fmt.Sprint(nWorkers)), nl(),
		b("Host: "), c(config.ValueOf.Host), nl(), nl(),
		b("👥 Users: "), c(fmt.Sprint(total)), p(" ("), c(fmt.Sprint(active24)), p(" active in 24h, "), c(fmt.Sprint(bn)), p(" banned)"), nl(),
		b("🔗 Links created: "), c(fmt.Sprint(st.TotalLinks)), p(" ("), c(fmt.Sprint(files)), p(" stored)"), nl(),
		b("⬇️ Downloads: "), c(fmt.Sprint(st.TotalDownloads)), nl(),
		b("📡 Data served: "), c(utils.HumanSize(st.TotalBytes)), nl(),
		b("🛠 Maintenance: "), p(onOff(store.Default.Maintenance())),
	)
}

func status(ctx *ext.Context, u *ext.Update) error {
	if _, ok := adminGate(ctx, u); !ok {
		return endGroups()
	}
	reply(ctx, u, statusParts()...)
	return endGroups()
}

// ---------- users ----------

func displayUser(usr store.User) string {
	name := usr.FirstName
	if name == "" {
		name = "user"
	}
	if usr.Username != "" {
		name += " (@" + usr.Username + ")"
	}
	return name
}

func usersParts() []part {
	total, bn := store.Default.UserCount()
	active24 := store.Default.ActiveSince(time.Now().Add(-24 * time.Hour).Unix())
	active7d := store.Default.ActiveSince(time.Now().Add(-7 * 24 * time.Hour).Unix())
	ps := parts(
		b("👥 Users"), nl(), nl(),
		b("Total: "), c(fmt.Sprint(total)), nl(),
		b("Active 24h: "), c(fmt.Sprint(active24)), nl(),
		b("Active 7d: "), c(fmt.Sprint(active7d)), nl(),
		b("Banned: "), c(fmt.Sprint(bn)), nl(), nl(),
		b("Most recent"), nl(),
	)
	for _, usr := range store.Default.RecentUsers(10) {
		ps = append(ps, p("• "+displayUser(usr)+" — "), c(fmt.Sprint(usr.ID)), nl())
	}
	return ps
}

func users(ctx *ext.Context, u *ext.Update) error {
	if _, ok := adminGate(ctx, u); !ok {
		return endGroups()
	}
	reply(ctx, u, usersParts()...)
	return endGroups()
}

func userInfo(ctx *ext.Context, u *ext.Update) error {
	if _, ok := adminGate(ctx, u); !ok {
		return endGroups()
	}
	target, ok := argInt64(u, 1)
	if !ok {
		reply(ctx, u, p("Usage: /user <user id>"))
		return endGroups()
	}
	usr, found := store.Default.GetUser(target)
	if !found {
		reply(ctx, u, p("⚠️ That user has never used the bot."))
		return endGroups()
	}
	var downloads, bytes int64
	for _, f := range store.Default.FilesOf(target, 0) {
		downloads += f.Downloads
		bytes += f.Bytes
	}
	role := "User"
	if IsOwner(target) {
		role = "Owner 👑"
	} else if store.Default.IsStoredAdmin(target) {
		role = "Admin 🛡"
	}
	ps := parts(
		b("👤 "+displayUser(usr)), nl(), nl(),
		b("ID: "), c(fmt.Sprint(usr.ID)), nl(),
		b("Role: "), p(role), nl(),
		b("Joined: "), p(utils.FormatUnix(usr.JoinedAt)), nl(),
		b("Last seen: "), p(utils.FormatUnix(usr.LastSeen)), nl(),
		b("Links: "), c(fmt.Sprint(usr.Files)), nl(),
		b("Downloads: "), c(fmt.Sprint(downloads)), nl(),
		b("Data served: "), c(utils.HumanSize(bytes)), nl(),
		b("Banned: "), p(fmt.Sprint(usr.Banned)),
	)
	if usr.Banned && usr.BanReason != "" {
		ps = append(ps, nl(), b("Reason: "), p(usr.BanReason))
	}
	reply(ctx, u, ps...)
	return endGroups()
}

// ---------- bans ----------

func ban(ctx *ext.Context, u *ext.Update) error {
	if _, ok := adminGate(ctx, u); !ok {
		return endGroups()
	}
	target, ok := argInt64(u, 1)
	if !ok {
		reply(ctx, u, p("Usage: /ban <user id> [reason]"))
		return endGroups()
	}
	if IsAdmin(target) {
		reply(ctx, u, p("⛔ You can't ban an admin."))
		return endGroups()
	}
	reason := ""
	if f := strings.Fields(argsAfterCommand(u)); len(f) > 1 {
		reason = strings.Join(f[1:], " ")
	}
	store.Default.SetBan(target, true, reason)
	notify(ctx, target, "🚫 You have been banned from using "+config.ValueOf.BotName+"."+reasonSuffix(reason))
	reply(ctx, u, p(fmt.Sprintf("🔨 User %d banned.", target)))
	return endGroups()
}

func reasonSuffix(r string) string {
	if r == "" {
		return ""
	}
	return "\nReason: " + r
}

func unban(ctx *ext.Context, u *ext.Update) error {
	if _, ok := adminGate(ctx, u); !ok {
		return endGroups()
	}
	target, ok := argInt64(u, 1)
	if !ok {
		reply(ctx, u, p("Usage: /unban <user id>"))
		return endGroups()
	}
	store.Default.SetBan(target, false, "")
	notify(ctx, target, "✅ You have been unbanned. Welcome back!")
	reply(ctx, u, p(fmt.Sprintf("✅ User %d unbanned.", target)))
	return endGroups()
}

func banned(ctx *ext.Context, u *ext.Update) error {
	if _, ok := adminGate(ctx, u); !ok {
		return endGroups()
	}
	list := store.Default.BannedUsers()
	if len(list) == 0 {
		reply(ctx, u, p("✅ Nobody is banned."))
		return endGroups()
	}
	ps := parts(b(fmt.Sprintf("🔨 Banned users (%d)", len(list))), nl(), nl())
	for idx, usr := range list {
		if idx >= 40 {
			ps = append(ps, i("…and more"))
			break
		}
		ps = append(ps, p("• "+displayUser(usr)+" — "), c(fmt.Sprint(usr.ID)))
		if usr.BanReason != "" {
			ps = append(ps, p(" — "+usr.BanReason))
		}
		ps = append(ps, nl())
	}
	reply(ctx, u, ps...)
	return endGroups()
}

// notify sends a best-effort plain message to a user.
func notify(ctx *ext.Context, userID int64, text string) {
	_, _ = ctx.SendMessage(userID, &tg.MessagesSendMessageRequest{Message: text})
}

// ---------- broadcast ----------

func broadcast(ctx *ext.Context, u *ext.Update) error {
	adminID, ok := adminGate(ctx, u)
	if !ok {
		return endGroups()
	}
	text := argsAfterCommand(u)
	if text == "" {
		if rm := repliedMessage(ctx, u, adminID); rm != nil {
			text = rm.Message
		}
	}
	if strings.TrimSpace(text) == "" {
		reply(ctx, u, p("Usage: /broadcast <text>\nOr reply to a text message with /broadcast."))
		return endGroups()
	}
	ids := store.Default.AllUserIDs()
	reply(ctx, u, p(fmt.Sprintf("📣 Broadcasting to %d users… I'll report back when done.", len(ids))))

	// Run in the background with a long-lived context so the update handler returns immediately.
	go func() {
		bctx := Client.CreateContext()
		sent, failed := 0, 0
		for _, id := range ids {
			if _, err := bctx.SendMessage(id, &tg.MessagesSendMessageRequest{Message: text}); err != nil {
				failed++
			} else {
				sent++
			}
			time.Sleep(100 * time.Millisecond) // stay far below Telegram's flood limits
		}
		_, _ = bctx.SendMessage(adminID, &tg.MessagesSendMessageRequest{
			Message: fmt.Sprintf("📣 Broadcast finished.\n✅ Delivered: %d\n❌ Failed: %d", sent, failed),
		})
	}()
	return endGroups()
}

// ---------- maintenance / purge / files ----------

func maintenance(ctx *ext.Context, u *ext.Update) error {
	if _, ok := adminGate(ctx, u); !ok {
		return endGroups()
	}
	switch strings.ToLower(argsAfterCommand(u)) {
	case "on", "enable", "true":
		store.Default.SetMaintenance(true)
		reply(ctx, u, p("🛠 Maintenance mode is now ON. Only admins can use the bot."))
	case "off", "disable", "false":
		store.Default.SetMaintenance(false)
		reply(ctx, u, p("✅ Maintenance mode is now OFF."))
	default:
		reply(ctx, u, p("Maintenance is "+onOff(store.Default.Maintenance())+"\nUsage: /maintenance on | off"))
	}
	return endGroups()
}

func topFilesParts() []part {
	top := store.Default.TopFiles(10)
	if len(top) == 0 || top[0].Downloads == 0 {
		return parts(b("🏆 Top files"), nl(), nl(), p("No downloads recorded yet."))
	}
	ps := parts(b("🏆 Most downloaded files"), nl(), nl())
	for idx, f := range top {
		if f.Downloads == 0 {
			break
		}
		ps = append(ps,
			b(fmt.Sprintf("%d. ", idx+1)), p(utils.ShortName(f.Name, 40)), nl(),
			p(fmt.Sprintf("   ⬇️ %d • %s • owner ", f.Downloads, utils.HumanSize(f.Bytes))), c(fmt.Sprint(f.OwnerID)), p(" • ID "), c(fmt.Sprint(f.MessageID)), nl(),
		)
	}
	return ps
}

func topFiles(ctx *ext.Context, u *ext.Update) error {
	if _, ok := adminGate(ctx, u); !ok {
		return endGroups()
	}
	reply(ctx, u, topFilesParts()...)
	return endGroups()
}

func restore(ctx *ext.Context, u *ext.Update) error {
	if _, ok := adminGate(ctx, u); !ok {
		return endGroups()
	}
	mid, ok := argInt64(u, 1)
	if !ok {
		reply(ctx, u, p("Usage: /restore <link id>"))
		return endGroups()
	}
	if !store.Default.RevokeFile(int(mid), true) {
		reply(ctx, u, p("⚠️ No link with that ID."))
		return endGroups()
	}
	reply(ctx, u, p(fmt.Sprintf("♻️ Link %d restored.", mid)))
	return endGroups()
}

func purge(ctx *ext.Context, u *ext.Update) error {
	if _, ok := adminGate(ctx, u); !ok {
		return endGroups()
	}
	n := store.Default.PurgeExpired(0)
	reply(ctx, u, p(fmt.Sprintf("🧹 Removed %d expired link record(s) from the database.", n)))
	return endGroups()
}

func workers(ctx *ext.Context, u *ext.Update) error {
	if _, ok := adminGate(ctx, u); !ok {
		return endGroups()
	}
	if WorkersInfo == nil || len(WorkersInfo()) == 0 {
		reply(ctx, u, p("No worker information available."))
		return endGroups()
	}
	ps := parts(b("🤖 Streaming bots"), nl(), nl())
	for _, line := range WorkersInfo() {
		ps = append(ps, p("• "+line), nl())
	}
	reply(ctx, u, ps...)
	return endGroups()
}

// ---------- admin management ----------

func admins(ctx *ext.Context, u *ext.Update) error {
	if _, ok := adminGate(ctx, u); !ok {
		return endGroups()
	}
	ps := parts(b("🛡 Admins"), nl(), nl(), b("Owners"), nl())
	for _, id := range config.ValueOf.Admins {
		ps = append(ps, p("• 👑 "), c(fmt.Sprint(id)), nl())
	}
	stored := store.Default.Admins()
	ps = append(ps, nl(), b("Added admins"), nl())
	if len(stored) == 0 {
		ps = append(ps, i("none"), nl())
	}
	for _, id := range stored {
		ps = append(ps, p("• "), c(fmt.Sprint(id)), nl())
	}
	reply(ctx, u, ps...)
	return endGroups()
}

func addAdmin(ctx *ext.Context, u *ext.Update) error {
	if _, ok := ownerGate(ctx, u); !ok {
		return endGroups()
	}
	target, ok := argInt64(u, 1)
	if !ok {
		reply(ctx, u, p("Usage: /addadmin <user id>"))
		return endGroups()
	}
	store.Default.AddAdmin(target)
	notify(ctx, target, "🛡 You are now an admin of "+config.ValueOf.BotName+". Send /help to see admin commands.")
	reply(ctx, u, p(fmt.Sprintf("✅ %d is now an admin.", target)))
	return endGroups()
}

func rmAdmin(ctx *ext.Context, u *ext.Update) error {
	if _, ok := ownerGate(ctx, u); !ok {
		return endGroups()
	}
	target, ok := argInt64(u, 1)
	if !ok {
		reply(ctx, u, p("Usage: /rmadmin <user id>"))
		return endGroups()
	}
	if IsOwner(target) {
		reply(ctx, u, p("⛔ Owners are set via the ADMINS environment variable and can't be removed here."))
		return endGroups()
	}
	if !store.Default.RemoveAdmin(target) {
		reply(ctx, u, p("⚠️ That user is not an added admin."))
		return endGroups()
	}
	reply(ctx, u, p(fmt.Sprintf("✅ %d is no longer an admin.", target)))
	return endGroups()
}
