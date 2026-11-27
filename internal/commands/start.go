package commands

import (
	"EverythingSuckz/fsb/config"
	"EverythingSuckz/fsb/internal/i18n"

	"github.com/celestix/gotgproto/dispatcher"
	"github.com/celestix/gotgproto/dispatcher/handlers"
	"github.com/celestix/gotgproto/ext"
	"github.com/gotd/td/tg"
)

func (m *command) LoadStart(d dispatcher.Dispatcher) {
	log := m.log.Named("start")
	defer log.Sugar().Info("Loaded")
	d.AddHandler(handlers.NewCommand("start", start))
}

func homeKeyboard(lang string) *tg.ReplyInlineMarkup {
	rows := [][]tg.KeyboardButtonClass{
		{btnCB(i18n.T(lang, "btn_help"), "ahar:help"), btnCB(i18n.T(lang, "btn_files"), "ahar:files")},
		{btnCB(i18n.T(lang, "btn_about"), "ahar:about"), btnCB(i18n.T(lang, "btn_lang"), "ahar:langmenu")},
	}
	if isHTTPS() {
		rows = append([][]tg.KeyboardButtonClass{{&tg.KeyboardButtonWebView{Text: i18n.T(lang, "btn_openapp"), URL: config.ValueOf.Host + "/app"}}}, rows...)
	}
	if config.ValueOf.SupportURL != "" {
		rows[len(rows)-1] = append(rows[len(rows)-1], btnURL("💬 Support", config.ValueOf.SupportURL))
	}
	return kb(rows...)
}

func homeText(name, lang string) []part {
	if name == "" {
		name = "there"
	}
	return parts(
		b(i18n.T(lang, "welcome_title", name)), nl(), nl(),
		p(i18n.T(lang, "welcome_body", config.ValueOf.BotName)), nl(), nl(),
		i(i18n.T(lang, "welcome_hint")),
	)
}

func start(ctx *ext.Context, u *ext.Update) error {
	id, ok := gate(ctx, u)
	if !ok {
		return endGroups()
	}
	name := ""
	if eu := u.EffectiveUser(); eu != nil {
		name = eu.FirstName
	}
	lang := userLang(id)
	replyKb(ctx, u, homeKeyboard(lang), homeText(name, lang)...)
	return endGroups()
}
