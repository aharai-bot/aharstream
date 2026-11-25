package commands

import (
	"github.com/gotd/td/tg"
	"go.uber.org/zap"
)

// RegisterMenu publishes the slash-command menu shown by Telegram clients.
func RegisterMenu(log *zap.Logger) {
	if Client == nil {
		return
	}
	cmds := []tg.BotCommand{
		{Command: "start", Description: "Start the bot"},
		{Command: "help", Description: "How to use & all commands"},
		{Command: "myfiles", Description: "Your recent links"},
		{Command: "mystats", Description: "Your usage statistics"},
		{Command: "link", Description: "Re-generate a link (reply to a file)"},
		{Command: "info", Description: "File details (reply to a file)"},
		{Command: "expiry", Description: "Set default link lifetime"},
		{Command: "revoke", Description: "Disable a link by its ID"},
		{Command: "ping", Description: "Check the bot is alive"},
		{Command: "about", Description: "About this bot"},
	}
	ctx := Client.CreateContext()
	_, err := Client.API().BotsSetBotCommands(ctx, &tg.BotsSetBotCommandsRequest{
		Scope:    &tg.BotCommandScopeDefault{},
		LangCode: "",
		Commands: cmds,
	})
	if err != nil {
		log.Warn("Failed to set bot command menu", zap.Error(err))
		return
	}
	log.Info("Bot command menu registered")
}
