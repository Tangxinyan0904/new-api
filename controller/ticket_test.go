package controller

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	ticketOwnerID = 501
	ticketOtherID = 502
	ticketStaffID = 1
)

var (
	ticketPNG  = append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0}, 32)...)
	ticketJPEG = append([]byte("\xff\xd8\xff\xe0"), bytes.Repeat([]byte{0}, 32)...)
	ticketSVG  = []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
)

type ticketResponse struct {
	Success    bool            `json:"success"`
	Message    string          `json:"message"`
	MessageKey string          `json:"message_key"`
	Data       json.RawMessage `json:"data"`
}

func setupTicketControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Ticket{}, &model.TicketMessage{}, &model.TicketAttachment{}, &model.TicketLog{}, &model.Log{}, &model.TopUp{}, &model.Redemption{}))
	require.NoError(t, db.Create(&[]model.User{
		{Id: ticketStaffID, Username: "root", AffCode: "ticket-aff-root", Role: common.RoleRootUser, Status: common.UserStatusEnabled},
		{Id: ticketOwnerID, Username: "owner", AffCode: "ticket-aff-owner", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
		{Id: ticketOtherID, Username: "other", AffCode: "ticket-aff-other", Role: common.RoleCommonUser, Status: common.UserStatusEnabled},
	}).Error)
	return db
}

// callTicketHandler runs handler with a multipart form (fields and images) or
// a JSON body (jsonBody) as user id.
func callTicketHandler(t *testing.T, handler gin.HandlerFunc, method, target string, userID int, params gin.Params, fields map[string]string, images [][]byte, jsonBody string) ticketResponse {
	t.Helper()
	var body bytes.Buffer
	contentType := "application/json"
	if jsonBody != "" {
		body.WriteString(jsonBody)
	} else if fields != nil || images != nil {
		writer := multipart.NewWriter(&body)
		for name, value := range fields {
			require.NoError(t, writer.WriteField(name, value))
		}
		for i, image := range images {
			part, err := writer.CreateFormFile("images", "image"+strconv.Itoa(i)+".png")
			require.NoError(t, err)
			_, err = part.Write(image)
			require.NoError(t, err)
		}
		require.NoError(t, writer.Close())
		contentType = writer.FormDataContentType()
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, &body)
	ctx.Request.Header.Set("Content-Type", contentType)
	ctx.Params = params
	ctx.Set("id", userID)
	ctx.Set("username", "user"+strconv.Itoa(userID))
	handler(ctx)

	var response ticketResponse
	if strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json") {
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	} else {
		response.Success = recorder.Code == http.StatusOK
		response.Data = recorder.Body.Bytes()
	}
	return response
}

func createTestTicket(t *testing.T, userID int, images [][]byte) model.Ticket {
	t.Helper()
	response := callTicketHandler(t, CreateTicket, http.MethodPost, "/api/ticket/", userID, nil, map[string]string{
		"subject":  "Cannot  call\nthe API",
		"category": model.TicketCategoryAPI,
		"content":  "Requests fail with 401.",
	}, images, "")
	require.True(t, response.Success, response.Message)
	var ticket model.Ticket
	require.NoError(t, common.Unmarshal(response.Data, &ticket))
	return ticket
}

func idParam(id int) gin.Params {
	return gin.Params{{Key: "id", Value: strconv.Itoa(id)}}
}

func TestCreateTicketValidatesImagesByContent(t *testing.T) {
	tests := []struct {
		name       string
		images     [][]byte
		messageKey string
	}{
		{name: "png and jpeg are stored", images: [][]byte{ticketPNG, ticketJPEG}},
		{name: "svg sent as png is rejected", images: [][]byte{ticketSVG}, messageKey: "Only PNG, JPEG, and WebP images are supported"},
		{name: "more than three images are rejected", images: [][]byte{ticketPNG, ticketPNG, ticketPNG, ticketPNG}, messageKey: "You can attach at most {{max}} images"},
		{name: "image over 2 MB is rejected", images: [][]byte{append(bytes.Clone(ticketPNG), make([]byte, model.TicketMaxImageBytes)...)}, messageKey: "Each image must be {{max}} MB or smaller"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTicketControllerTestDB(t)
			response := callTicketHandler(t, CreateTicket, http.MethodPost, "/api/ticket/", ticketOwnerID, nil, map[string]string{
				"subject":  "Screenshot",
				"category": model.TicketCategoryOther,
				"content":  "See attached.",
			}, tt.images, "")

			var attachments []model.TicketAttachment
			require.NoError(t, db.Find(&attachments).Error)
			if tt.messageKey != "" {
				assert.False(t, response.Success)
				assert.Equal(t, tt.messageKey, response.MessageKey)
				assert.Empty(t, attachments)
				return
			}
			require.True(t, response.Success, response.Message)
			require.Len(t, attachments, 2)
			assert.Equal(t, "image/png", attachments[0].MimeType)
			assert.Equal(t, "image/jpeg", attachments[1].MimeType)
			assert.Equal(t, ticketPNG, attachments[0].Data)
		})
	}
}

func TestCreateTicketNormalizesSubjectAndValidatesFields(t *testing.T) {
	setupTicketControllerTestDB(t)
	ticket := createTestTicket(t, ticketOwnerID, nil)
	assert.Equal(t, "Cannot call the API", ticket.Subject)
	assert.Equal(t, model.TicketStatusOpen, ticket.Status)
	assert.Equal(t, model.TicketPriorityNormal, ticket.Priority)

	tests := []struct {
		name       string
		fields     map[string]string
		messageKey string
	}{
		{name: "blank subject", fields: map[string]string{"subject": " \n ", "category": "api", "content": "x"}, messageKey: "Ticket subject is required"},
		{name: "long subject", fields: map[string]string{"subject": strings.Repeat("字", model.TicketSubjectMaxRunes+1), "category": "api", "content": "x"}, messageKey: "Ticket subject must be at most {{max}} characters"},
		{name: "unknown category", fields: map[string]string{"subject": "s", "category": "sales", "content": "x"}, messageKey: "Invalid ticket category"},
		{name: "blank content", fields: map[string]string{"subject": "s", "category": "api", "content": "  "}, messageKey: "Ticket content is required"},
		{name: "long content", fields: map[string]string{"subject": "s", "category": "api", "content": strings.Repeat("a", model.TicketContentMaxRunes+1)}, messageKey: "Ticket content must be at most {{max}} characters"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := callTicketHandler(t, CreateTicket, http.MethodPost, "/api/ticket/", ticketOwnerID, nil, tt.fields, nil, "")
			assert.False(t, response.Success)
			assert.Equal(t, tt.messageKey, response.MessageKey)
		})
	}
}

func TestCreateTicketEnforcesOpenTicketLimit(t *testing.T) {
	setupTicketControllerTestDB(t)
	var first model.Ticket
	for i := range model.TicketMaxOpenPerUser {
		ticket := createTestTicket(t, ticketOwnerID, nil)
		if i == 0 {
			first = ticket
		}
	}

	response := callTicketHandler(t, CreateTicket, http.MethodPost, "/api/ticket/", ticketOwnerID, nil, map[string]string{
		"subject": "One more", "category": "other", "content": "x",
	}, nil, "")
	assert.False(t, response.Success)
	assert.Equal(t, model.ErrTicketOpenLimit.Key, response.MessageKey)

	closeResponse := callTicketHandler(t, CloseSelfTicket, http.MethodPost, "/", ticketOwnerID, idParam(first.Id), nil, nil, "")
	require.True(t, closeResponse.Success, closeResponse.Message)
	createTestTicket(t, ticketOwnerID, nil)
}

func TestTicketsAreHiddenFromOtherUsers(t *testing.T) {
	setupTicketControllerTestDB(t)
	ticket := createTestTicket(t, ticketOwnerID, [][]byte{ticketPNG})
	detail := callTicketHandler(t, GetSelfTicket, http.MethodGet, "/", ticketOwnerID, idParam(ticket.Id), nil, nil, "")
	require.True(t, detail.Success)
	var view model.TicketDetail
	require.NoError(t, common.Unmarshal(detail.Data, &view))
	attachmentID := view.Messages[0].Attachments[0].Id

	for name, handler := range map[string]gin.HandlerFunc{"detail": GetSelfTicket, "reply": ReplySelfTicket, "close": CloseSelfTicket} {
		t.Run(name, func(t *testing.T) {
			response := callTicketHandler(t, handler, http.MethodPost, "/", ticketOtherID, idParam(ticket.Id), map[string]string{"content": "hi"}, nil, "")
			assert.False(t, response.Success)
			assert.Equal(t, model.ErrTicketNotFound.Key, response.MessageKey)
		})
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	ctx.Params = idParam(attachmentID)
	ctx.Set("id", ticketOtherID)
	GetSelfTicketAttachment(ctx)
	assert.Equal(t, http.StatusNotFound, recorder.Code)

	owner := callTicketHandler(t, GetSelfTicketAttachment, http.MethodGet, "/", ticketOwnerID, idParam(attachmentID), nil, nil, "")
	require.True(t, owner.Success)
	assert.Equal(t, ticketPNG, []byte(owner.Data))
}

func TestTicketReplyLifecycleTracksStatusAndUnread(t *testing.T) {
	setupTicketControllerTestDB(t)
	ticket := createTestTicket(t, ticketOwnerID, nil)

	staffReply := callTicketHandler(t, ReplyTicket, http.MethodPost, "/", ticketStaffID, idParam(ticket.Id), map[string]string{"content": "Please check your key."}, nil, "")
	require.True(t, staffReply.Success, staffReply.Message)
	var afterStaff model.Ticket
	require.NoError(t, common.Unmarshal(staffReply.Data, &afterStaff))
	assert.Equal(t, model.TicketStatusWaitingUser, afterStaff.Status)
	assert.True(t, afterStaff.UserUnread)

	unread := callTicketHandler(t, GetSelfTicketUnreadCount, http.MethodGet, "/", ticketOwnerID, nil, nil, nil, "")
	assert.JSONEq(t, `{"unread":1}`, string(unread.Data))

	detail := callTicketHandler(t, GetSelfTicket, http.MethodGet, "/", ticketOwnerID, idParam(ticket.Id), nil, nil, "")
	var view model.TicketDetail
	require.NoError(t, common.Unmarshal(detail.Data, &view))
	require.Len(t, view.Messages, 2)
	assert.True(t, view.Messages[1].IsStaff)
	assert.Empty(t, view.Messages[1].AuthorName, "users must not see staff account names")
	unread = callTicketHandler(t, GetSelfTicketUnreadCount, http.MethodGet, "/", ticketOwnerID, nil, nil, nil, "")
	assert.JSONEq(t, `{"unread":0}`, string(unread.Data))

	userReply := callTicketHandler(t, ReplySelfTicket, http.MethodPost, "/", ticketOwnerID, idParam(ticket.Id), map[string]string{"content": "Still failing."}, nil, "")
	require.True(t, userReply.Success, userReply.Message)
	var afterUser model.Ticket
	require.NoError(t, common.Unmarshal(userReply.Data, &afterUser))
	assert.Equal(t, model.TicketStatusOpen, afterUser.Status)

	closing := callTicketHandler(t, ReplyTicket, http.MethodPost, "/", ticketStaffID, idParam(ticket.Id), map[string]string{"content": "Fixed.", "status": model.TicketStatusClosed}, nil, "")
	require.True(t, closing.Success, closing.Message)
	var closed model.Ticket
	require.NoError(t, common.Unmarshal(closing.Data, &closed))
	assert.Equal(t, model.TicketStatusClosed, closed.Status)
	assert.Equal(t, model.TicketClosedByStaff, closed.ClosedBy)

	rejected := callTicketHandler(t, ReplySelfTicket, http.MethodPost, "/", ticketOwnerID, idParam(ticket.Id), map[string]string{"content": "Thanks"}, nil, "")
	assert.False(t, rejected.Success)
	assert.Equal(t, model.ErrTicketClosed.Key, rejected.MessageKey)

	staffDetail := callTicketHandler(t, GetTicket, http.MethodGet, "/", ticketStaffID, idParam(ticket.Id), nil, nil, "")
	var staffView model.TicketDetail
	require.NoError(t, common.Unmarshal(staffDetail.Data, &staffView))
	assert.Equal(t, "owner", staffView.Username)
	assert.Equal(t, "root", staffView.Messages[1].AuthorName)
}

func TestUpdateTicketValidatesStatusAndPriority(t *testing.T) {
	setupTicketControllerTestDB(t)
	ticket := createTestTicket(t, ticketOwnerID, nil)

	updated := callTicketHandler(t, UpdateTicket, http.MethodPut, "/", ticketStaffID, idParam(ticket.Id), nil, nil, `{"priority":4,"status":"processing"}`)
	require.True(t, updated.Success, updated.Message)
	var view model.Ticket
	require.NoError(t, common.Unmarshal(updated.Data, &view))
	assert.Equal(t, model.TicketPriorityUrgent, view.Priority)
	assert.Equal(t, model.TicketStatusProcessing, view.Status)

	for body, key := range map[string]string{
		`{"priority":5}`:      model.ErrTicketPriority.Key,
		`{"status":"solved"}`: model.ErrTicketStatus.Key,
	} {
		response := callTicketHandler(t, UpdateTicket, http.MethodPut, "/", ticketStaffID, idParam(ticket.Id), nil, nil, body)
		assert.False(t, response.Success, body)
		assert.Equal(t, key, response.MessageKey, body)
	}
}

func TestListTicketsOmitsAttachmentDataAndFilters(t *testing.T) {
	setupTicketControllerTestDB(t)
	createTestTicket(t, ticketOwnerID, [][]byte{ticketPNG})
	other := createTestTicket(t, ticketOtherID, nil)
	callTicketHandler(t, UpdateTicket, http.MethodPut, "/", ticketStaffID, idParam(other.Id), nil, nil, `{"priority":3}`)

	response := callTicketHandler(t, ListTickets, http.MethodGet, "/api/ticket/admin?p=1&page_size=10&priority=3", ticketStaffID, nil, nil, nil, "")
	require.True(t, response.Success, response.Message)
	var page struct {
		Total int                    `json:"total"`
		Items []model.TicketListItem `json:"items"`
	}
	require.NoError(t, common.Unmarshal(response.Data, &page))
	require.Equal(t, 1, page.Total)
	assert.Equal(t, other.Id, page.Items[0].Id)
	assert.Equal(t, "other", page.Items[0].Username)

	byUser := callTicketHandler(t, ListTickets, http.MethodGet, "/api/ticket/admin?keyword=owner", ticketStaffID, nil, nil, nil, "")
	require.NoError(t, common.Unmarshal(byUser.Data, &page))
	require.Equal(t, 1, page.Total)
	assert.Equal(t, ticketOwnerID, page.Items[0].UserId)
	assert.NotContains(t, string(byUser.Data), `"data"`)
}

func TestCloseStaleWaitingTicketsOnlyClosesExpiredWaitingTickets(t *testing.T) {
	db := setupTicketControllerTestDB(t)
	now := common.GetTimestamp()
	cutoff := now - model.TicketAutoCloseSeconds
	tickets := []model.Ticket{
		{Id: 1, UserId: ticketOwnerID, Subject: "stale", Status: model.TicketStatusWaitingUser, LastReplyAt: cutoff - 1},
		{Id: 2, UserId: ticketOwnerID, Subject: "recent", Status: model.TicketStatusWaitingUser, LastReplyAt: cutoff + 60},
		{Id: 3, UserId: ticketOwnerID, Subject: "awaiting staff", Status: model.TicketStatusOpen, LastReplyAt: cutoff - 1},
	}
	require.NoError(t, db.Create(&tickets).Error)

	stale, err := model.HasStaleWaitingTickets(cutoff)
	require.NoError(t, err)
	assert.True(t, stale)
	closed, err := model.CloseStaleWaitingTickets(cutoff, now)
	require.NoError(t, err)
	assert.Equal(t, int64(1), closed)

	var stored []model.Ticket
	require.NoError(t, db.Order("id").Find(&stored).Error)
	assert.Equal(t, model.TicketStatusClosed, stored[0].Status)
	assert.Equal(t, model.TicketClosedByAuto, stored[0].ClosedBy)
	assert.Equal(t, model.TicketStatusWaitingUser, stored[1].Status)
	assert.Equal(t, model.TicketStatusOpen, stored[2].Status)

	stale, err = model.HasStaleWaitingTickets(cutoff)
	require.NoError(t, err)
	assert.False(t, stale)
}

func TestApiTicketsAttachTheOwnersCallLogs(t *testing.T) {
	db := setupTicketControllerTestDB(t)
	require.NoError(t, db.Create(&[]model.Log{
		{Id: 1, UserId: ticketOwnerID, Type: model.LogTypeConsume, ModelName: "gpt-4o", Quota: 120, PromptTokens: 30, ChannelId: 7, RequestId: "req-1", CreatedAt: 1_790_000_000},
		{Id: 2, UserId: ticketOwnerID, Type: model.LogTypeError, ModelName: "gpt-4o", Content: "upstream timeout", ChannelId: 7, RequestId: "req-2", CreatedAt: 1_790_000_100},
		{Id: 3, UserId: ticketOwnerID, Type: model.LogTypeTopup, Quota: 5000, CreatedAt: 1_790_000_200},
		{Id: 4, UserId: ticketOtherID, Type: model.LogTypeConsume, ModelName: "gpt-4o", CreatedAt: 1_790_000_300},
	}).Error)

	candidates := callTicketHandler(t, ListSelfTicketLogs, http.MethodGet, "/api/ticket/logs?p=1&page_size=10", ticketOwnerID, nil, nil, nil, "")
	require.True(t, candidates.Success, candidates.Message)
	var page struct {
		Total int               `json:"total"`
		Items []model.TicketLog `json:"items"`
	}
	require.NoError(t, common.Unmarshal(candidates.Data, &page))
	assert.Equal(t, 2, page.Total, "only the owner's consumed and failed requests")
	require.Len(t, page.Items, 2)
	assert.Equal(t, []int{2, 1}, []int{page.Items[0].LogId, page.Items[1].LogId})
	assert.Zero(t, page.Items[0].ChannelId, "users never see the channel")

	fields := func(category, logIds string) map[string]string {
		return map[string]string{"subject": "Slow requests", "category": category, "content": "See attached logs.", "log_ids": logIds}
	}
	for name, test := range map[string]struct{ category, logIds, key string }{
		"a log of another user": {model.TicketCategoryAPI, "1,4", model.ErrTicketLogNotFound.Key},
		"a top-up log":          {model.TicketCategoryAPI, "3", model.ErrTicketLogNotFound.Key},
		"too many logs":         {model.TicketCategoryAPI, "1,2,5,6,7,8", model.ErrTicketLogLimit.Key},
		"a non-API ticket":      {model.TicketCategoryBilling, "1", model.ErrTicketLogCategory.Key},
		"a malformed log id":    {model.TicketCategoryAPI, "1,abc", "Invalid parameters"},
	} {
		t.Run(name, func(t *testing.T) {
			response := callTicketHandler(t, CreateTicket, http.MethodPost, "/api/ticket/", ticketOwnerID, nil, fields(test.category, test.logIds), nil, "")
			assert.False(t, response.Success)
			assert.Equal(t, test.key, response.MessageKey)
		})
	}

	created := callTicketHandler(t, CreateTicket, http.MethodPost, "/api/ticket/", ticketOwnerID, nil, fields(model.TicketCategoryAPI, "1, 2,1"), nil, "")
	require.True(t, created.Success, created.Message)
	var ticket model.Ticket
	require.NoError(t, common.Unmarshal(created.Data, &ticket))

	var userView model.TicketDetail
	userDetail := callTicketHandler(t, GetSelfTicket, http.MethodGet, "/", ticketOwnerID, idParam(ticket.Id), nil, nil, "")
	require.NoError(t, common.Unmarshal(userDetail.Data, &userView))
	require.Len(t, userView.Logs, 2, "a repeated id is attached once")
	assert.Equal(t, "upstream timeout", userView.Logs[0].Content)
	assert.Equal(t, "req-1", userView.Logs[1].RequestId)
	assert.Equal(t, 120, userView.Logs[1].Quota)
	assert.Zero(t, userView.Logs[1].ChannelId)

	// The copy survives the original log being cleaned up.
	require.NoError(t, db.Delete(&model.Log{}, 1).Error)
	var staffView model.TicketDetail
	staffDetail := callTicketHandler(t, GetTicket, http.MethodGet, "/", ticketStaffID, idParam(ticket.Id), nil, nil, "")
	require.NoError(t, common.Unmarshal(staffDetail.Data, &staffView))
	require.Len(t, staffView.Logs, 2)
	assert.Equal(t, 7, staffView.Logs[1].ChannelId)
	assert.Equal(t, "req-1", staffView.Logs[1].RequestId)
}

func TestTicketRefundInfoSumsPaymentsAndRecentUsage(t *testing.T) {
	db := setupTicketControllerTestDB(t)
	now := common.GetTimestamp()
	todayStart := now - 3600
	weekStart := todayStart - 6*24*3600
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", ticketOwnerID).Update("quota", 1234).Error)
	require.NoError(t, db.Create(&[]model.TopUp{
		{UserId: ticketOwnerID, Amount: 5, Money: 10, TradeNo: "t1", PaymentMethod: "alipay", Status: common.TopUpStatusSuccess, CompleteTime: now - 200},
		{UserId: ticketOwnerID, Amount: 10, Money: 20.5, TradeNo: "t2", PaymentMethod: "wxpay", Status: common.TopUpStatusSuccess, CompleteTime: now - 100},
		{UserId: ticketOwnerID, Amount: 50, Money: 100, TradeNo: "t3", PaymentMethod: "alipay", Status: common.TopUpStatusPending},
		{UserId: ticketOtherID, Amount: 70, Money: 140, TradeNo: "t4", PaymentMethod: "alipay", Status: common.TopUpStatusSuccess, CompleteTime: now},
	}).Error)
	require.NoError(t, db.Create(&[]model.Redemption{
		{Key: "redeemed-code-a", Name: "a", Quota: 500, Status: common.RedemptionCodeStatusUsed, UsedUserId: ticketOwnerID},
		{Key: "redeemed-code-b", Name: "b", Quota: 300, Status: common.RedemptionCodeStatusUsed, UsedUserId: ticketOwnerID},
		{Key: "unused-code-c", Name: "c", Quota: 900, Status: common.RedemptionCodeStatusEnabled},
	}).Error)
	// A deleted code was still redeemed.
	require.NoError(t, db.Where("name = ?", "b").Delete(&model.Redemption{}).Error)
	require.NoError(t, db.Create(&[]model.Log{
		{UserId: ticketOwnerID, Type: model.LogTypeConsume, Quota: 100, CreatedAt: now - 60},
		{UserId: ticketOwnerID, Type: model.LogTypeRefund, Quota: 30, CreatedAt: now - 30},
		{UserId: ticketOwnerID, Type: model.LogTypeConsume, Quota: 200, CreatedAt: now - 3*24*3600},
		{UserId: ticketOwnerID, Type: model.LogTypeConsume, Quota: 999, CreatedAt: now - 10*24*3600},
		{UserId: ticketOwnerID, Type: model.LogTypeTopup, Quota: 5000, CreatedAt: now - 60},
		{UserId: ticketOtherID, Type: model.LogTypeConsume, Quota: 77, CreatedAt: now - 60},
	}).Error)

	created := callTicketHandler(t, CreateTicket, http.MethodPost, "/api/ticket/", ticketOwnerID, nil, map[string]string{
		"subject": "Please refund", "category": model.TicketCategoryRefund, "content": "I no longer need the balance.",
	}, nil, "")
	require.True(t, created.Success, created.Message)
	var refundTicket model.Ticket
	require.NoError(t, common.Unmarshal(created.Data, &refundTicket))
	target := "/?today_start=" + strconv.FormatInt(todayStart, 10) + "&week_start=" + strconv.FormatInt(weekStart, 10)

	response := callTicketHandler(t, GetTicketRefundInfo, http.MethodGet, target, ticketStaffID, idParam(refundTicket.Id), nil, nil, "")
	require.True(t, response.Success, response.Message)
	var info model.TicketRefundInfo
	require.NoError(t, common.Unmarshal(response.Data, &info))
	assert.Equal(t, model.TicketRefundInfo{
		Quota:           1234,
		TopUpCount:      2,
		TopUpAmount:     15,
		TopUpMoney:      30.5,
		LatestTopUp:     &model.TicketRefundTopUp{PaymentMethod: "wxpay", Amount: 10, Money: 20.5, CompleteTime: now - 100},
		RedemptionCount: 2,
		RedemptionQuota: 800,
		TodayQuota:      70,
		WeekQuota:       270,
	}, info)

	other := createTestTicket(t, ticketOwnerID, nil)
	notRefund := callTicketHandler(t, GetTicketRefundInfo, http.MethodGet, target, ticketStaffID, idParam(other.Id), nil, nil, "")
	assert.False(t, notRefund.Success)
	assert.Equal(t, "Refund details are only available for refund tickets", notRefund.MessageKey)

	badRange := callTicketHandler(t, GetTicketRefundInfo, http.MethodGet, "/?today_start=1&week_start=2", ticketStaffID, idParam(refundTicket.Id), nil, nil, "")
	assert.False(t, badRange.Success)
	assert.Equal(t, "Invalid parameters", badRange.MessageKey)
}
