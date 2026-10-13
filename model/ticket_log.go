package model

import (
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
)

// TicketMaxLogs is how many call logs a ticket can attach.
const TicketMaxLogs = 5

// ticketLogContentMaxRunes keeps a long upstream error from bloating a ticket.
const ticketLogContentMaxRunes = 2000

var (
	ErrTicketLogNotFound     = common.NewMessage("Some selected call logs were not found")
	ErrTicketLogCategory     = common.NewMessage("Call logs can only be attached to API & models tickets")
	ErrTicketLogLimit        = common.NewMessage("You can attach at most {{max}} call logs", map[string]any{"max": TicketMaxLogs})
	ticketAttachableLogTypes = []int{LogTypeConsume, LogTypeError}
)

// TicketLog is a copy of a call log the user attached to an API ticket, taken
// when the ticket is opened so it survives log cleanup.
type TicketLog struct {
	Id               int    `json:"id"`
	TicketId         int    `json:"ticket_id" gorm:"index"`
	LogId            int    `json:"log_id"`
	CreatedAt        int64  `json:"created_at"`
	Type             int    `json:"type"`
	ModelName        string `json:"model_name" gorm:"type:varchar(255)"`
	TokenName        string `json:"token_name" gorm:"type:varchar(255)"`
	Group            string `json:"group" gorm:"column:group_name;type:varchar(64)"`
	Quota            int    `json:"quota"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	UseTime          int    `json:"use_time"`
	IsStream         bool   `json:"is_stream"`
	RequestId        string `json:"request_id" gorm:"type:varchar(64)"`
	Content          string `json:"content" gorm:"type:text"`
	// ChannelId is only shown to staff.
	ChannelId int `json:"channel_id,omitempty"`
}

func ticketLogFromLog(log Log) TicketLog {
	content := log.Content
	if utf8.RuneCountInString(content) > ticketLogContentMaxRunes {
		content = string([]rune(content)[:ticketLogContentMaxRunes])
	}
	return TicketLog{
		LogId:            log.Id,
		CreatedAt:        log.CreatedAt,
		Type:             log.Type,
		ModelName:        log.ModelName,
		TokenName:        log.TokenName,
		Group:            log.Group,
		Quota:            log.Quota,
		PromptTokens:     log.PromptTokens,
		CompletionTokens: log.CompletionTokens,
		UseTime:          log.UseTime,
		IsStream:         log.IsStream,
		RequestId:        log.RequestId,
		Content:          content,
		ChannelId:        log.ChannelId,
	}
}

// ListTicketLogCandidates returns the user's call logs (consumed and failed
// requests) newest first, for picking which ones to attach to a ticket.
func ListTicketLogCandidates(userId int, modelName string, pageInfo *common.PageInfo) ([]TicketLog, int64, error) {
	tx := LOG_DB.Model(&Log{}).Where("user_id = ? AND type IN ?", userId, ticketAttachableLogTypes)
	tx, err := applyExplicitLogTextFilter(tx, "model_name", modelName)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []Log
	if err := tx.Order("id DESC").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	items := make([]TicketLog, 0, len(logs))
	for _, log := range logs {
		item := ticketLogFromLog(log)
		item.ChannelId = 0
		items = append(items, item)
	}
	return items, total, nil
}

// snapshotTicketLogs copies the user's call logs with the given ids. Every id
// must be one of the user's consumed or failed requests.
func snapshotTicketLogs(userId int, logIds []int) ([]TicketLog, error) {
	if len(logIds) > TicketMaxLogs {
		return nil, ErrTicketLogLimit
	}
	var logs []Log
	if err := LOG_DB.Where("id IN ? AND user_id = ? AND type IN ?", logIds, userId, ticketAttachableLogTypes).
		Order("id DESC").Find(&logs).Error; err != nil {
		return nil, err
	}
	if len(logs) != len(logIds) {
		return nil, ErrTicketLogNotFound
	}
	snapshots := make([]TicketLog, 0, len(logs))
	for _, log := range logs {
		snapshots = append(snapshots, ticketLogFromLog(log))
	}
	return snapshots, nil
}

func ticketLogs(ticketId int, staffView bool) ([]TicketLog, error) {
	logs := make([]TicketLog, 0)
	if err := DB.Where("ticket_id = ?", ticketId).Order("id ASC").Find(&logs).Error; err != nil {
		return nil, err
	}
	if !staffView {
		for i := range logs {
			logs[i].ChannelId = 0
		}
	}
	return logs, nil
}
