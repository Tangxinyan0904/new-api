package service

import (
	"errors"
	"html"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
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
	gopool.Go(func() { sendTicketStaffNotices(ticket, username, isNew) })
}

func sendTicketStaffNotices(ticket model.Ticket, username string, isNew bool) {
	params := ticketNotifyParams(ticket, username, "/ticket-management/")
	subjectKey, contentKey := i18n.MsgTicketStaffReplySubject, i18n.MsgTicketStaffReplyContent
	render := func(lang string) (string, string) {
		return i18n.Translate(lang, subjectKey, params), i18n.Translate(lang, contentKey, params)
	}
	if !isNew {
		notifyAdmins(NotifyTypeTicket, receivesTicketNotices, render, nil)
		return
	}
	subjectKey, contentKey = i18n.MsgTicketStaffNewSubject, i18n.MsgTicketStaffNewContent
	emailed := notificationMailboxes{}
	var delivery sync.WaitGroup
	if address := operation_setting.GetTicketAdminEmail(); common.ValidEmailRecipient(address) {
		emailed[strings.ToLower(address)] = true
		delivery.Go(func() {
			subject, content := render("")
			if err := common.SendEmail(subject, address, content); err != nil {
				common.SysLog(common.LogText("failed to email administrator for new ticket %d: %s", ticket.Id, err.Error()))
			}
		})
	}
	notifyAdmins(NotifyTypeTicket, receivesTicketNotices, render, emailed)
	delivery.Wait()
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
		if err := sendTicketOwnerNotice(owner.Id, owner.Email, setting, dto.NewNotify(NotifyTypeTicket, subject, content, nil)); err != nil {
			common.SysLog(common.LogText("failed to notify owner of ticket %d: %s", ticket.Id, err.Error()))
		}
	})
}

func sendTicketOwnerNotice(userID int, profileEmail string, setting dto.UserSetting, notice dto.Notify) error {
	profileEmail = strings.TrimSpace(profileEmail)
	if !common.ValidEmailRecipient(profileEmail) {
		profileEmail = ""
	}
	prefersEmail := setting.NotifyType == "" || setting.NotifyType == dto.NotifyTypeEmail
	if profileEmail == "" && prefersEmail {
		return nil
	}
	// Both deliveries belong to the same reply and consume one notification.
	if err := reserveUserNotification(userID, notice.Type); err != nil {
		return err
	}
	if prefersEmail {
		emailed := notificationMailboxes{strings.ToLower(profileEmail): true}
		setting.NotificationEmail = strings.Join(emailed.remaining(setting.NotificationEmail), ";")
		if setting.NotificationEmail == "" {
			return sendEmailNotify(profileEmail, notice)
		}
	}
	if profileEmail == "" {
		return deliverUserNotification(userID, profileEmail, setting, notice)
	}
	var delivery sync.WaitGroup
	var emailErr error
	delivery.Go(func() { emailErr = sendEmailNotify(profileEmail, notice) })
	preferenceErr := deliverUserNotification(userID, profileEmail, setting, notice)
	delivery.Wait()
	return errors.Join(emailErr, preferenceErr)
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
