package service

import (
	"html"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/bytedance/gopkg/util/gopool"
)

// NotifyTypeTicket is the notification type, and rate-limit bucket, of ticket
// notices.
const NotifyTypeTicket = "ticket"

// NotifyTicketStaff tells every administrator who receives ticket notices
// that a ticket was opened (isNew) or that its owner replied, each in their
// saved language. It runs in the background.
func NotifyTicketStaff(ticket model.Ticket, username string, isNew bool) {
	gopool.Go(func() {
		subjectKey, contentKey := i18n.MsgTicketStaffReplySubject, i18n.MsgTicketStaffReplyContent
		if isNew {
			subjectKey, contentKey = i18n.MsgTicketStaffNewSubject, i18n.MsgTicketStaffNewContent
		}
		params := ticketNotifyParams(ticket, username, "/ticket-management/")
		notifyAdmins(NotifyTypeTicket, receivesTicketNotices, func(lang string) (string, string) {
			return i18n.Translate(lang, subjectKey, params), i18n.Translate(lang, contentKey, params)
		})
	})
}

// receivesTicketNotices reports whether an administrator gets ticket notices.
// Until they choose, the root user does and other administrators do not.
func receivesTicketNotices(role int, setting dto.UserSetting) bool {
	if setting.TicketNotifyEnabled != nil {
		return *setting.TicketNotifyEnabled
	}
	return role == common.RoleRootUser
}

// NotifyTicketOwner tells the ticket owner that staff replied. It runs in the
// background.
func NotifyTicketOwner(ticket model.Ticket) {
	gopool.Go(func() {
		owner, err := model.GetUserById(ticket.UserId, true)
		if err != nil {
			common.SysLog(common.LogText("failed to load owner of ticket %d: %s", ticket.Id, err.Error()))
			return
		}
		setting := owner.GetSetting()
		params := ticketNotifyParams(ticket, owner.Username, "/tickets/")
		subject := i18n.Translate(setting.Language, i18n.MsgTicketUserReplySubject, params)
		content := i18n.Translate(setting.Language, i18n.MsgTicketUserReplyContent, params)
		if err := NotifyUser(owner.Id, owner.Email, setting, dto.NewNotify(NotifyTypeTicket, subject, content, nil)); err != nil {
			common.SysLog(common.LogText("failed to notify owner of ticket %d: %s", ticket.Id, err.Error()))
		}
	})
}

// ticketNotifyParams escapes user-written text because email notices are sent
// as HTML.
func ticketNotifyParams(ticket model.Ticket, username, pagePath string) map[string]any {
	return map[string]any{
		"Id":       ticket.Id,
		"Subject":  html.EscapeString(ticket.Subject),
		"Username": html.EscapeString(username),
		"Link":     strings.TrimRight(system_setting.ServerAddress, "/") + pagePath + strconv.Itoa(ticket.Id),
	}
}
