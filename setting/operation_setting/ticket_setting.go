package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// TicketSetting holds the limits on opening support tickets.
type TicketSetting struct {
	// DailyLimit caps how many tickets a user can open in any 24 hours,
	// closed tickets included. Zero or less means no limit.
	DailyLimit int `json:"daily_limit"`
}

var ticketSetting = TicketSetting{
	DailyLimit: 3,
}

func init() {
	config.GlobalConfig.Register("ticket_setting", &ticketSetting)
}

// GetTicketDailyLimit returns how many tickets a user can open in 24 hours;
// zero or less means no limit.
func GetTicketDailyLimit() int {
	return ticketSetting.DailyLimit
}
