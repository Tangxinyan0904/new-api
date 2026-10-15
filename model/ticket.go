package model

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

const (
	// TicketStatusOpen waits for staff: a new ticket or one the user replied to.
	TicketStatusOpen        = "open"
	TicketStatusProcessing  = "processing"
	TicketStatusWaitingUser = "waiting_user"
	TicketStatusClosed      = "closed"

	TicketClosedByUser  = "user"
	TicketClosedByStaff = "staff"
	TicketClosedByAuto  = "auto"

	TicketReplyByUser  = "user"
	TicketReplyByStaff = "staff"

	TicketCategoryBilling = "billing"
	TicketCategoryAPI     = "api"
	TicketCategoryAccount = "account"
	TicketCategoryRefund  = "refund"
	TicketCategoryOther   = "other"

	TicketPriorityLow    = 1
	TicketPriorityNormal = 2
	TicketPriorityHigh   = 3
	TicketPriorityUrgent = 4

	TicketMaxOpenPerUser      = 5
	TicketDailyWindowSeconds  = 24 * 3600
	TicketSubjectMaxRunes     = 100
	TicketContentMaxRunes     = 5000
	TicketMaxImagesPerMessage = 3
	TicketMaxImageBytes       = 2 << 20
	// TicketAutoCloseSeconds closes a ticket that has waited this long for the
	// user to reply.
	TicketAutoCloseSeconds int64 = 7 * 24 * 3600
)

var (
	ErrTicketNotFound   = common.NewMessage("Ticket not found")
	ErrTicketClosed     = common.NewMessage("This ticket is closed. Please open a new ticket.")
	ErrTicketOpenLimit  = common.NewMessage("You can have at most {{max}} open tickets. Close a resolved ticket first.", map[string]any{"max": TicketMaxOpenPerUser})
	ErrTicketStatus     = common.NewMessage("Invalid ticket status")
	ErrTicketPriority   = common.NewMessage("Invalid ticket priority")
	ErrTicketAttachment = common.NewMessage("Attachment not found")
)

var ticketCategories = []string{TicketCategoryBilling, TicketCategoryAPI, TicketCategoryAccount, TicketCategoryRefund, TicketCategoryOther}

func IsValidTicketCategory(category string) bool {
	return slices.Contains(ticketCategories, category)
}

func IsValidTicketStatus(status string) bool {
	switch status {
	case TicketStatusOpen, TicketStatusProcessing, TicketStatusWaitingUser, TicketStatusClosed:
		return true
	}
	return false
}

func IsValidTicketPriority(priority int) bool {
	return priority >= TicketPriorityLow && priority <= TicketPriorityUrgent
}

type Ticket struct {
	Id       int    `json:"id"`
	UserId   int    `json:"user_id" gorm:"index"`
	Subject  string `json:"subject" gorm:"type:varchar(128);not null"`
	Category string `json:"category" gorm:"type:varchar(32);index"`
	Priority int    `json:"priority" gorm:"index"`
	Status   string `json:"status" gorm:"type:varchar(32);index"`
	// UserUnread is set when staff replies or closes the ticket and cleared
	// when the owner opens it.
	UserUnread  bool   `json:"user_unread"`
	LastReplyAt int64  `json:"last_reply_at" gorm:"index"`
	LastReplyBy string `json:"last_reply_by" gorm:"type:varchar(16)"`
	CreatedAt   int64  `json:"created_at" gorm:"index"`
	ClosedAt    int64  `json:"closed_at"`
	ClosedBy    string `json:"closed_by" gorm:"type:varchar(16)"`
}

type TicketMessage struct {
	Id        int    `json:"id"`
	TicketId  int    `json:"ticket_id" gorm:"index"`
	UserId    int    `json:"user_id" gorm:"index"`
	IsStaff   bool   `json:"is_staff"`
	Content   string `json:"content" gorm:"type:text"`
	CreatedAt int64  `json:"created_at"`
}

type TicketAttachment struct {
	Id        int    `json:"id"`
	TicketId  int    `json:"ticket_id" gorm:"index"`
	MessageId int    `json:"message_id" gorm:"index"`
	MimeType  string `json:"mime_type" gorm:"type:varchar(32)"`
	Size      int    `json:"size"`
	Data      []byte `json:"-"`
	CreatedAt int64  `json:"created_at"`
}

// TicketImage is a validated image to store with a message.
type TicketImage struct {
	MimeType string
	Data     []byte
}

type TicketAttachmentView struct {
	Id       int    `json:"id"`
	MimeType string `json:"mime_type"`
	Size     int    `json:"size"`
}

type TicketMessageView struct {
	Id      int    `json:"id"`
	IsStaff bool   `json:"is_staff"`
	Content string `json:"content"`
	// AuthorName is only filled for staff views; users see staff replies as
	// coming from the support team.
	AuthorName  string                 `json:"author_name,omitempty"`
	CreatedAt   int64                  `json:"created_at"`
	Attachments []TicketAttachmentView `json:"attachments"`
}

type TicketListItem struct {
	Ticket
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}

type TicketDetail struct {
	TicketListItem
	Messages []TicketMessageView `json:"messages"`
	// Logs are the call logs attached when the ticket was opened.
	Logs []TicketLog `json:"logs"`
	// OwnerBanned tells staff that the owner is blocked from tickets.
	OwnerBanned bool `json:"owner_banned,omitempty"`
}

type TicketFilter struct {
	Status   string
	Category string
	Priority int
	Keyword  string
	// SortByPriority orders by priority before last activity.
	SortByPriority bool
}

// CreateTicket opens a ticket with its first message and, for API tickets, a
// copy of the attached call logs. The owner row is locked so concurrent
// requests cannot exceed the open or daily ticket limits.
func CreateTicket(userId int, subject, category, content string, images []TicketImage, logIds []int) (*Ticket, error) {
	var logs []TicketLog
	if len(logIds) > 0 {
		if category != TicketCategoryAPI {
			return nil, ErrTicketLogCategory
		}
		var err error
		if logs, err = snapshotTicketLogs(userId, logIds); err != nil {
			return nil, err
		}
	}
	now := common.GetTimestamp()
	ticket := &Ticket{
		UserId:      userId,
		Subject:     subject,
		Category:    category,
		Priority:    TicketPriorityNormal,
		Status:      TicketStatusOpen,
		LastReplyAt: now,
		LastReplyBy: TicketReplyByUser,
		CreatedAt:   now,
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var owner User
		if err := lockForUpdate(tx).Select("id").Where("id = ?", userId).First(&owner).Error; err != nil {
			return err
		}
		banned, err := isTicketBanned(tx, userId)
		if err != nil {
			return err
		}
		if banned {
			return ErrTicketBanned
		}
		var openCount int64
		if err := tx.Model(&Ticket{}).Where("user_id = ? AND status <> ?", userId, TicketStatusClosed).Count(&openCount).Error; err != nil {
			return err
		}
		if openCount >= TicketMaxOpenPerUser {
			return ErrTicketOpenLimit
		}
		// Closed tickets count too, so closing a ticket does not free a slot.
		if dailyLimit := operation_setting.GetTicketDailyLimit(); dailyLimit > 0 {
			var recentCount int64
			if err := tx.Model(&Ticket{}).Where("user_id = ? AND created_at > ?", userId, now-TicketDailyWindowSeconds).Count(&recentCount).Error; err != nil {
				return err
			}
			if recentCount >= int64(dailyLimit) {
				return common.NewMessage("You can open at most {{max}} tickets in 24 hours. Please try again later.", map[string]any{"max": dailyLimit})
			}
		}
		if err := tx.Create(ticket).Error; err != nil {
			return err
		}
		if len(logs) > 0 {
			for i := range logs {
				logs[i].TicketId = ticket.Id
			}
			if err := tx.Create(&logs).Error; err != nil {
				return err
			}
		}
		return createTicketMessage(tx, ticket.Id, userId, false, content, images, now)
	})
	if err != nil {
		return nil, err
	}
	return ticket, nil
}

func createTicketMessage(tx *gorm.DB, ticketId, authorId int, staff bool, content string, images []TicketImage, now int64) error {
	message := &TicketMessage{
		TicketId:  ticketId,
		UserId:    authorId,
		IsStaff:   staff,
		Content:   content,
		CreatedAt: now,
	}
	if err := tx.Create(message).Error; err != nil {
		return err
	}
	// One insert per image keeps each statement under MySQL's default
	// max_allowed_packet (4 MB on 5.7).
	for _, image := range images {
		attachment := &TicketAttachment{
			TicketId:  ticketId,
			MessageId: message.Id,
			MimeType:  image.MimeType,
			Size:      len(image.Data),
			Data:      image.Data,
			CreatedAt: now,
		}
		if err := tx.Create(attachment).Error; err != nil {
			return err
		}
	}
	return nil
}

// AddTicketUserReply appends the owner's reply and hands the ticket back to
// staff. Closed tickets cannot be replied to, and blocked owners cannot reply.
func AddTicketUserReply(ticketId, userId int, content string, images []TicketImage) (*Ticket, error) {
	var ticket Ticket
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", ticketId, userId).First(&ticket).Error; err != nil {
			return ticketLookupError(err)
		}
		banned, err := isTicketBanned(tx, userId)
		if err != nil {
			return err
		}
		if banned {
			return ErrTicketBanned
		}
		if ticket.Status == TicketStatusClosed {
			return ErrTicketClosed
		}
		now := common.GetTimestamp()
		if err := createTicketMessage(tx, ticket.Id, userId, false, content, images, now); err != nil {
			return err
		}
		ticket.Status = TicketStatusOpen
		ticket.LastReplyAt = now
		ticket.LastReplyBy = TicketReplyByUser
		ticket.UserUnread = false
		return tx.Model(&Ticket{}).Where("id = ?", ticket.Id).Updates(map[string]any{
			"status":        ticket.Status,
			"last_reply_at": ticket.LastReplyAt,
			"last_reply_by": ticket.LastReplyBy,
			"user_unread":   false,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

// AddTicketStaffReply appends a staff reply and moves the ticket to status,
// which defaults to waiting for the user. Replying to a closed ticket reopens
// it unless status closes it again.
func AddTicketStaffReply(ticketId, staffId int, content string, images []TicketImage, status string) (*Ticket, error) {
	if status == "" {
		status = TicketStatusWaitingUser
	}
	if status != TicketStatusProcessing && status != TicketStatusWaitingUser && status != TicketStatusClosed {
		return nil, ErrTicketStatus
	}
	var ticket Ticket
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ?", ticketId).First(&ticket).Error; err != nil {
			return ticketLookupError(err)
		}
		now := common.GetTimestamp()
		if err := createTicketMessage(tx, ticket.Id, staffId, true, content, images, now); err != nil {
			return err
		}
		ticket.LastReplyAt = now
		ticket.LastReplyBy = TicketReplyByStaff
		ticket.UserUnread = true
		applyTicketStatus(&ticket, status, TicketClosedByStaff, now)
		return saveTicketState(tx, &ticket)
	})
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

// UpdateTicketByStaff changes status and/or priority. Closing marks the ticket
// unread for the owner so they notice it was closed.
func UpdateTicketByStaff(ticketId int, status *string, priority *int) (*Ticket, error) {
	if status != nil && !IsValidTicketStatus(*status) {
		return nil, ErrTicketStatus
	}
	if priority != nil && !IsValidTicketPriority(*priority) {
		return nil, ErrTicketPriority
	}
	var ticket Ticket
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where("id = ?", ticketId).First(&ticket).Error; err != nil {
			return ticketLookupError(err)
		}
		if priority != nil {
			ticket.Priority = *priority
		}
		if status != nil && *status != ticket.Status {
			applyTicketStatus(&ticket, *status, TicketClosedByStaff, common.GetTimestamp())
			if *status == TicketStatusClosed {
				ticket.UserUnread = true
			}
		}
		return saveTicketState(tx, &ticket)
	})
	if err != nil {
		return nil, err
	}
	return &ticket, nil
}

// CloseTicketByUser closes the owner's ticket. Closing an already closed
// ticket succeeds without changes.
func CloseTicketByUser(ticketId, userId int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var ticket Ticket
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", ticketId, userId).First(&ticket).Error; err != nil {
			return ticketLookupError(err)
		}
		if ticket.Status == TicketStatusClosed {
			return nil
		}
		applyTicketStatus(&ticket, TicketStatusClosed, TicketClosedByUser, common.GetTimestamp())
		return saveTicketState(tx, &ticket)
	})
}

func applyTicketStatus(ticket *Ticket, status, closedBy string, now int64) {
	ticket.Status = status
	if status == TicketStatusClosed {
		ticket.ClosedAt = now
		ticket.ClosedBy = closedBy
		return
	}
	ticket.ClosedAt = 0
	ticket.ClosedBy = ""
}

func saveTicketState(tx *gorm.DB, ticket *Ticket) error {
	return tx.Model(&Ticket{}).Where("id = ?", ticket.Id).Updates(map[string]any{
		"status":        ticket.Status,
		"priority":      ticket.Priority,
		"user_unread":   ticket.UserUnread,
		"last_reply_at": ticket.LastReplyAt,
		"last_reply_by": ticket.LastReplyBy,
		"closed_at":     ticket.ClosedAt,
		"closed_by":     ticket.ClosedBy,
	}).Error
}

func ticketLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrTicketNotFound
	}
	return err
}

func ListUserTickets(userId int, status string, pageInfo *common.PageInfo) ([]*Ticket, int64, error) {
	query := DB.Model(&Ticket{}).Where("user_id = ?", userId)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var tickets []*Ticket
	err := query.Order("last_reply_at DESC").Order("id DESC").
		Offset(pageInfo.GetStartIdx()).Limit(pageInfo.GetPageSize()).Find(&tickets).Error
	return tickets, total, err
}

func ListTickets(filter TicketFilter, pageInfo *common.PageInfo) ([]*TicketListItem, int64, error) {
	query := DB.Model(&Ticket{})
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}
	if filter.Priority != 0 {
		query = query.Where("priority = ?", filter.Priority)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		condition := DB.Where("subject LIKE ?", "%"+keyword+"%")
		if id, err := strconv.Atoi(strings.TrimPrefix(keyword, "#")); err == nil {
			condition = condition.Or("id = ?", id)
		}
		var userIds []int
		if err := DB.Model(&User{}).Where("username = ?", keyword).Pluck("id", &userIds).Error; err != nil {
			return nil, 0, err
		}
		if len(userIds) > 0 {
			condition = condition.Or("user_id IN ?", userIds)
		}
		query = query.Where(condition)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if filter.SortByPriority {
		query = query.Order("priority DESC")
	}
	var tickets []*Ticket
	err := query.Order("last_reply_at DESC").Order("id DESC").
		Offset(pageInfo.GetStartIdx()).Limit(pageInfo.GetPageSize()).Find(&tickets).Error
	if err != nil {
		return nil, 0, err
	}
	userIds := make([]int, 0, len(tickets))
	for _, ticket := range tickets {
		userIds = append(userIds, ticket.UserId)
	}
	users, err := ticketUserNames(userIds)
	if err != nil {
		return nil, 0, err
	}
	items := make([]*TicketListItem, 0, len(tickets))
	for _, ticket := range tickets {
		user := users[ticket.UserId]
		items = append(items, &TicketListItem{Ticket: *ticket, Username: user.Username, DisplayName: user.DisplayName})
	}
	return items, total, nil
}

func ticketUserNames(userIds []int) (map[int]User, error) {
	result := make(map[int]User, len(userIds))
	if len(userIds) == 0 {
		return result, nil
	}
	var users []User
	if err := DB.Unscoped().Select("id", "username", "display_name").Where("id IN ?", userIds).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		result[user.Id] = user
	}
	return result, nil
}

// GetUserTicketDetail returns the owner's ticket and marks it read.
func GetUserTicketDetail(ticketId, userId int) (*TicketDetail, error) {
	var ticket Ticket
	if err := DB.Where("id = ? AND user_id = ?", ticketId, userId).First(&ticket).Error; err != nil {
		return nil, ticketLookupError(err)
	}
	if ticket.UserUnread {
		if err := DB.Model(&Ticket{}).Where("id = ?", ticket.Id).Update("user_unread", false).Error; err != nil {
			return nil, err
		}
		ticket.UserUnread = false
	}
	messages, err := ticketMessageViews(ticket.Id, false)
	if err != nil {
		return nil, err
	}
	logs, err := ticketLogs(ticket.Id, false)
	if err != nil {
		return nil, err
	}
	return &TicketDetail{TicketListItem: TicketListItem{Ticket: ticket}, Messages: messages, Logs: logs}, nil
}

// GetStaffTicketDetail returns any ticket with its owner and author names.
func GetStaffTicketDetail(ticketId int) (*TicketDetail, error) {
	var ticket Ticket
	if err := DB.Where("id = ?", ticketId).First(&ticket).Error; err != nil {
		return nil, ticketLookupError(err)
	}
	messages, err := ticketMessageViews(ticket.Id, true)
	if err != nil {
		return nil, err
	}
	logs, err := ticketLogs(ticket.Id, true)
	if err != nil {
		return nil, err
	}
	users, err := ticketUserNames([]int{ticket.UserId})
	if err != nil {
		return nil, err
	}
	banned, err := isTicketBanned(DB, ticket.UserId)
	if err != nil {
		return nil, err
	}
	owner := users[ticket.UserId]
	return &TicketDetail{
		TicketListItem: TicketListItem{Ticket: ticket, Username: owner.Username, DisplayName: owner.DisplayName},
		Messages:       messages,
		Logs:           logs,
		OwnerBanned:    banned,
	}, nil
}

func ticketMessageViews(ticketId int, staffView bool) ([]TicketMessageView, error) {
	var messages []TicketMessage
	if err := DB.Where("ticket_id = ?", ticketId).Order("id ASC").Find(&messages).Error; err != nil {
		return nil, err
	}
	var attachments []TicketAttachment
	if err := DB.Select("id", "message_id", "mime_type", "size").Where("ticket_id = ?", ticketId).Order("id ASC").Find(&attachments).Error; err != nil {
		return nil, err
	}
	attachmentsByMessage := make(map[int][]TicketAttachmentView)
	for _, attachment := range attachments {
		attachmentsByMessage[attachment.MessageId] = append(attachmentsByMessage[attachment.MessageId], TicketAttachmentView{
			Id:       attachment.Id,
			MimeType: attachment.MimeType,
			Size:     attachment.Size,
		})
	}
	authors := map[int]User{}
	if staffView {
		authorIds := make([]int, 0, len(messages))
		for _, message := range messages {
			authorIds = append(authorIds, message.UserId)
		}
		var err error
		if authors, err = ticketUserNames(authorIds); err != nil {
			return nil, err
		}
	}
	views := make([]TicketMessageView, 0, len(messages))
	for _, message := range messages {
		view := TicketMessageView{
			Id:          message.Id,
			IsStaff:     message.IsStaff,
			Content:     message.Content,
			CreatedAt:   message.CreatedAt,
			Attachments: attachmentsByMessage[message.Id],
		}
		if view.Attachments == nil {
			view.Attachments = []TicketAttachmentView{}
		}
		if staffView {
			view.AuthorName = authors[message.UserId].Username
		}
		views = append(views, view)
	}
	return views, nil
}

// GetTicketAttachment loads an attachment with its data. A non-zero ownerId
// limits the lookup to that user's tickets.
func GetTicketAttachment(attachmentId, ownerId int) (*TicketAttachment, error) {
	query := DB.Model(&TicketAttachment{}).Where("ticket_attachments.id = ?", attachmentId)
	if ownerId != 0 {
		query = query.Joins("JOIN tickets ON tickets.id = ticket_attachments.ticket_id").Where("tickets.user_id = ?", ownerId)
	}
	var attachment TicketAttachment
	if err := query.Select("ticket_attachments.*").First(&attachment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTicketAttachment
		}
		return nil, err
	}
	return &attachment, nil
}

func CountUserUnreadTickets(userId int) (int64, error) {
	var count int64
	err := DB.Model(&Ticket{}).Where("user_id = ? AND user_unread = ?", userId, true).Count(&count).Error
	return count, err
}

// CountTicketsAwaitingStaff counts tickets whose last action needs staff.
func CountTicketsAwaitingStaff() (int64, error) {
	var count int64
	err := DB.Model(&Ticket{}).Where("status = ?", TicketStatusOpen).Count(&count).Error
	return count, err
}

func HasStaleWaitingTickets(cutoff int64) (bool, error) {
	var ticket Ticket
	err := DB.Select("id").Where("status = ? AND last_reply_at < ?", TicketStatusWaitingUser, cutoff).Limit(1).Find(&ticket).Error
	return ticket.Id != 0, err
}

// CloseStaleWaitingTickets closes tickets that have waited for the user since
// before cutoff.
func CloseStaleWaitingTickets(cutoff, now int64) (int64, error) {
	result := DB.Model(&Ticket{}).
		Where("status = ? AND last_reply_at < ?", TicketStatusWaitingUser, cutoff).
		Updates(map[string]any{
			"status":    TicketStatusClosed,
			"closed_at": now,
			"closed_by": TicketClosedByAuto,
		})
	return result.RowsAffected, result.Error
}
