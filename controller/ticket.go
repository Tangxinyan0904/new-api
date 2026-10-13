package controller

import (
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// ticketRefundMaxRangeSeconds bounds how far back refund usage may be asked
// for: the 7-day range plus a day of time zone slack.
const ticketRefundMaxRangeSeconds = 8 * 24 * 3600

// ticketFormMaxBytes bounds a ticket form: the largest allowed images plus
// room for the text fields and multipart framing.
const ticketFormMaxBytes = model.TicketMaxImagesPerMessage*model.TicketMaxImageBytes + 1<<20

// ticketImageTypes are the content types served back inline. SVG is excluded
// because it can carry scripts.
var ticketImageTypes = []string{"image/png", "image/jpeg", "image/webp"}

// parseTicketForm parses a multipart ticket form and returns its validated
// images. Image types come from the file content, not the client.
func parseTicketForm(c *gin.Context) ([]model.TicketImage, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ticketFormMaxBytes)
	if err := c.Request.ParseMultipartForm(ticketFormMaxBytes); err != nil {
		return nil, common.NewMessage("Invalid ticket form or attachments are too large")
	}
	form := c.Request.MultipartForm
	defer form.RemoveAll()

	files := form.File["images"]
	if len(files) > model.TicketMaxImagesPerMessage {
		return nil, common.NewMessage("You can attach at most {{max}} images", map[string]any{"max": model.TicketMaxImagesPerMessage})
	}
	tooLarge := common.NewMessage("Each image must be {{max}} MB or smaller", map[string]any{"max": model.TicketMaxImageBytes >> 20})
	images := make([]model.TicketImage, 0, len(files))
	for _, header := range files {
		if header.Size > model.TicketMaxImageBytes {
			return nil, tooLarge
		}
		file, err := header.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(file, model.TicketMaxImageBytes+1))
		_ = file.Close()
		if err != nil {
			return nil, err
		}
		if len(data) > model.TicketMaxImageBytes {
			return nil, tooLarge
		}
		mimeType := http.DetectContentType(data)
		if !slices.Contains(ticketImageTypes, mimeType) {
			return nil, common.NewMessage("Only PNG, JPEG, and WebP images are supported")
		}
		images = append(images, model.TicketImage{MimeType: mimeType, Data: data})
	}
	return images, nil
}

func ticketContentError(content string, required bool) error {
	if required && content == "" {
		return common.NewMessage("Ticket content is required")
	}
	if utf8.RuneCountInString(content) > model.TicketContentMaxRunes {
		return common.NewMessage("Ticket content must be at most {{max}} characters", map[string]any{"max": model.TicketContentMaxRunes})
	}
	return nil
}

func ticketIdParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiError(c, model.ErrTicketNotFound)
		return 0, false
	}
	return id, true
}

// serveTicketAttachment writes an attachment image. A non-zero ownerId limits
// it to that user's tickets.
func serveTicketAttachment(c *gin.Context, ownerId int) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorStatus(c, http.StatusNotFound, model.ErrTicketAttachment)
		return
	}
	attachment, err := model.GetTicketAttachment(id, ownerId)
	if err != nil {
		common.ApiErrorStatus(c, http.StatusNotFound, err)
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'")
	c.Header("Content-Disposition", "inline")
	c.Header("Cache-Control", "private, no-store")
	c.Data(http.StatusOK, attachment.MimeType, attachment.Data)
}

func ListSelfTickets(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	status := c.Query("status")
	if status != "" && !model.IsValidTicketStatus(status) {
		common.ApiError(c, model.ErrTicketStatus)
		return
	}
	tickets, total, err := model.ListUserTickets(c.GetInt("id"), status, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tickets)
	common.ApiSuccess(c, pageInfo)
}

func GetSelfTicketUnreadCount(c *gin.Context) {
	count, err := model.CountUserUnreadTickets(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"unread": count})
}

func CreateTicket(c *gin.Context) {
	images, err := parseTicketForm(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	subject := strings.Join(strings.Fields(c.PostForm("subject")), " ")
	if subject == "" {
		common.ApiErrorT(c, "Ticket subject is required")
		return
	}
	if utf8.RuneCountInString(subject) > model.TicketSubjectMaxRunes {
		common.ApiErrorT(c, "Ticket subject must be at most {{max}} characters", map[string]any{"max": model.TicketSubjectMaxRunes})
		return
	}
	category := c.PostForm("category")
	if !model.IsValidTicketCategory(category) {
		common.ApiErrorT(c, "Invalid ticket category")
		return
	}
	content := strings.TrimSpace(c.PostForm("content"))
	if err := ticketContentError(content, true); err != nil {
		common.ApiError(c, err)
		return
	}
	// log_ids is a comma-separated list of the user's call logs to attach.
	var logIds []int
	for field := range strings.SplitSeq(c.PostForm("log_ids"), ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		logId, err := strconv.Atoi(field)
		if err != nil || logId <= 0 {
			common.ApiErrorT(c, "Invalid parameters")
			return
		}
		if !slices.Contains(logIds, logId) {
			logIds = append(logIds, logId)
		}
	}
	ticket, err := model.CreateTicket(c.GetInt("id"), subject, category, content, images, logIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	service.NotifyTicketStaff(*ticket, c.GetString("username"), true)
	common.ApiSuccess(c, ticket)
}

// ListSelfTicketLogs lists the user's call logs that can be attached to an
// API ticket, newest first.
func ListSelfTicketLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListTicketLogCandidates(c.GetInt("id"), strings.TrimSpace(c.Query("model_name")), pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetSelfTicket(c *gin.Context) {
	id, ok := ticketIdParam(c)
	if !ok {
		return
	}
	detail, err := model.GetUserTicketDetail(id, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, detail)
}

func ReplySelfTicket(c *gin.Context) {
	id, ok := ticketIdParam(c)
	if !ok {
		return
	}
	images, err := parseTicketForm(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	content := strings.TrimSpace(c.PostForm("content"))
	if content == "" && len(images) == 0 {
		common.ApiErrorT(c, "Reply content or an image is required")
		return
	}
	if err := ticketContentError(content, false); err != nil {
		common.ApiError(c, err)
		return
	}
	ticket, err := model.AddTicketUserReply(id, c.GetInt("id"), content, images)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	service.NotifyTicketStaff(*ticket, c.GetString("username"), false)
	common.ApiSuccess(c, ticket)
}

func CloseSelfTicket(c *gin.Context) {
	id, ok := ticketIdParam(c)
	if !ok {
		return
	}
	if err := model.CloseTicketByUser(id, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func GetSelfTicketAttachment(c *gin.Context) {
	serveTicketAttachment(c, c.GetInt("id"))
}

func ListTickets(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	filter := model.TicketFilter{
		Status:         c.Query("status"),
		Category:       c.Query("category"),
		Keyword:        c.Query("keyword"),
		SortByPriority: c.Query("sort") == "priority",
	}
	if filter.Status != "" && !model.IsValidTicketStatus(filter.Status) {
		common.ApiError(c, model.ErrTicketStatus)
		return
	}
	if filter.Category != "" && !model.IsValidTicketCategory(filter.Category) {
		common.ApiErrorT(c, "Invalid ticket category")
		return
	}
	if raw := c.Query("priority"); raw != "" {
		priority, err := strconv.Atoi(raw)
		if err != nil || !model.IsValidTicketPriority(priority) {
			common.ApiError(c, model.ErrTicketPriority)
			return
		}
		filter.Priority = priority
	}
	tickets, total, err := model.ListTickets(filter, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tickets)
	common.ApiSuccess(c, pageInfo)
}

func GetTicketSummary(c *gin.Context) {
	count, err := model.CountTicketsAwaitingStaff()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"awaiting": count})
}

func GetTicket(c *gin.Context) {
	id, ok := ticketIdParam(c)
	if !ok {
		return
	}
	detail, err := model.GetStaffTicketDetail(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, detail)
}

// GetTicketRefundInfo returns the payment and usage facts of a refund
// ticket's owner. The client sends the day starts in its own time zone.
func GetTicketRefundInfo(c *gin.Context) {
	id, ok := ticketIdParam(c)
	if !ok {
		return
	}
	todayStart, errToday := strconv.ParseInt(c.Query("today_start"), 10, 64)
	weekStart, errWeek := strconv.ParseInt(c.Query("week_start"), 10, 64)
	now := common.GetTimestamp()
	if errToday != nil || errWeek != nil || weekStart > todayStart || todayStart > now ||
		now-weekStart > ticketRefundMaxRangeSeconds {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	detail, err := model.GetStaffTicketDetail(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if detail.Category != model.TicketCategoryRefund {
		common.ApiErrorT(c, "Refund details are only available for refund tickets")
		return
	}
	info, err := model.GetTicketRefundInfo(detail.UserId, todayStart, weekStart)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, info)
}

func ReplyTicket(c *gin.Context) {
	id, ok := ticketIdParam(c)
	if !ok {
		return
	}
	images, err := parseTicketForm(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	content := strings.TrimSpace(c.PostForm("content"))
	if content == "" && len(images) == 0 {
		common.ApiErrorT(c, "Reply content or an image is required")
		return
	}
	if err := ticketContentError(content, false); err != nil {
		common.ApiError(c, err)
		return
	}
	ticket, err := model.AddTicketStaffReply(id, c.GetInt("id"), content, images, c.PostForm("status"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	service.NotifyTicketOwner(*ticket)
	common.ApiSuccess(c, ticket)
}

type updateTicketRequest struct {
	Status   *string `json:"status"`
	Priority *int    `json:"priority"`
}

func UpdateTicket(c *gin.Context) {
	id, ok := ticketIdParam(c)
	if !ok {
		return
	}
	var request updateTicketRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	ticket, err := model.UpdateTicketByStaff(id, request.Status, request.Priority)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, ticket)
}

func GetTicketAttachment(c *gin.Context) {
	serveTicketAttachment(c, 0)
}
