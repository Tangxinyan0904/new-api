package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrTicketBanned = common.NewMessage("You have been blocked from submitting tickets.")

// TicketBan blocks a user from opening tickets and replying to them. The rest
// of the account is unaffected.
type TicketBan struct {
	UserId int `json:"user_id" gorm:"primaryKey;autoIncrement:false"`
	// CreatedBy is the staff member who set the block.
	CreatedBy int   `json:"created_by"`
	CreatedAt int64 `json:"created_at"`
}

func isTicketBanned(db *gorm.DB, userId int) (bool, error) {
	var count int64
	err := db.Model(&TicketBan{}).Where("user_id = ?", userId).Count(&count).Error
	return count > 0, err
}

// SetTicketOwnerBanned blocks or unblocks the owner of a ticket. Repeating
// either action succeeds without changes.
func SetTicketOwnerBanned(ticketId, staffId int, banned bool) error {
	var ticket Ticket
	if err := DB.Select("id", "user_id").Where("id = ?", ticketId).First(&ticket).Error; err != nil {
		return ticketLookupError(err)
	}
	if !banned {
		return DB.Where("user_id = ?", ticket.UserId).Delete(&TicketBan{}).Error
	}
	ban := TicketBan{UserId: ticket.UserId, CreatedBy: staffId, CreatedAt: common.GetTimestamp()}
	return DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&ban).Error
}
