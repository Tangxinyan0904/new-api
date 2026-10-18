package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
)

// RenderVerificationCodeEmail lays out an email that carries a verification
// code, in lang. The title, intro and notice are message keys whose text may
// use SystemName.
func RenderVerificationCodeEmail(lang, code string, validMinutes int, titleKey, introKey, noticeKey string) (string, error) {
	params := map[string]any{"SystemName": common.SystemName, "Code": code, "Minutes": validMinutes}
	return common.RenderVerificationEmail(common.VerificationEmail{
		Preview: i18n.Translate(lang, i18n.MsgEmailCodePreview, params),
		Title:   i18n.Translate(lang, titleKey, params),
		Intro:   i18n.Translate(lang, introKey, params),
		Code:    code,
		Hints: []string{
			i18n.Translate(lang, i18n.MsgEmailCodeValidity, params),
			i18n.Translate(lang, i18n.MsgEmailCodeHint, params),
		},
		Notice: i18n.Translate(lang, noticeKey, params),
		Footer: i18n.Translate(lang, i18n.MsgEmailFooter, params),
	})
}
