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

// NotifyTicketStaff tells the root user that a ticket was opened (isNew) or
// that its owner replied. It runs in the background.
func NotifyTicketStaff(ticket model.Ticket, username string, isNew bool) {
	gopool.Go(func() {
		root := model.GetRootUser()
		if root == nil || root.Id == 0 {
			return
		}
		subjectKey, contentKey := i18n.MsgTicketStaffReplySubject, i18n.MsgTicketStaffReplyContent
		if isNew {
			subjectKey, contentKey = i18n.MsgTicketStaffNewSubject, i18n.MsgTicketStaffNewContent
		}
		setting := root.GetSetting()
		params := ticketNotifyParams(ticket, username, "/ticket-management/")
		subject := i18n.Translate(setting.Language, subjectKey, params)
		content := i18n.Translate(setting.Language, contentKey, params)
		if err := NotifyUser(root.Id, root.Email, setting, dto.NewNotify(NotifyTypeTicket, subject, content, nil)); err != nil {
			common.SysLog(common.LogText("failed to notify staff about ticket %d: %s", ticket.Id, err.Error()))
		}
	})
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
