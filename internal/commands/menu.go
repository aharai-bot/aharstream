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
		{Command: "app", Description: "Open the My Files app"},
		{Command: "login", Description: "Sign in to the web app"},
		{Command: "storage", Description: "Your storage summary"},
		{Command: "language", Description: "Change the bot language"},
		{Command: "password", Description: "Protect a link with a password"},
		{Command: "views", Description: "Limit how often a link can be opened"},
		{Command: "linkexpiry", Description: "Change one link's expiry"},
		{Command: "collections", Description: "Your shareable collections"},
		{Command: "apikey", Description: "Manage API keys"},
		{Command: "webhook", Description: "Set a webhook URL"},
		{Command: "cancel", Description: "Cancel the current action"},
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

	// The blue "Open" button next to the message box launches the Mini App.
	if isHTTPS() {
		_, err := Client.API().BotsSetBotMenuButton(ctx, &tg.BotsSetBotMenuButtonRequest{
			UserID: &tg.InputUserEmpty{},
			Button: &tg.BotMenuButton{Text: "My Files", URL: baseURL() + "/app"},
		})
		if err != nil {
			log.Warn("Failed to set the Mini App menu button", zap.Error(err))
		}
	}
}
