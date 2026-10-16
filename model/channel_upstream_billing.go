package model

import "gorm.io/gorm"

const (
	// UpstreamBillingSub2API reads the spend of the channel's keys from a
	// sub2api upstream.
	UpstreamBillingSub2API = "sub2api"
	// UpstreamBillingNewAPI reads the spend of the channel's keys from a
	// new-api upstream account's usage statistics.
	UpstreamBillingNewAPI = "newapi"
)

// ChannelUpstreamBilling makes the cost statistics take a channel's cost from
// what its upstream actually charged instead of estimating it with a ratio.
type ChannelUpstreamBilling struct {
	ChannelId int    `json:"channel_id" gorm:"primaryKey;autoIncrement:false"`
	Kind      string `json:"kind" gorm:"type:varchar(16);not null"`
	// UsdRate is how many of this site's dollars one upstream dollar is worth.
	UsdRate float64 `json:"usd_rate"`
	// AccessToken and UpstreamUserId sign in to a new-api upstream account.
	// The token is never sent back to the browser.
	AccessToken    string `json:"-" gorm:"type:text"`
	UpstreamUserId int    `json:"upstream_user_id"`
	// BaseURL is the channel address the access token was entered for; the
	// token is not sent to any other address.
	BaseURL   string `json:"-" gorm:"type:text"`
	UpdatedAt int64  `json:"updated_at"`
}

// GetChannelUpstreamBillings returns every channel's upstream billing by
// channel id.
func GetChannelUpstreamBillings() (map[int]ChannelUpstreamBilling, error) {
	var billings []ChannelUpstreamBilling
	if err := DB.Find(&billings).Error; err != nil {
		return nil, err
	}
	byChannel := make(map[int]ChannelUpstreamBilling, len(billings))
	for _, billing := range billings {
		byChannel[billing.ChannelId] = billing
	}
	return byChannel, nil
}

// GetChannelUpstreamBilling returns a channel's upstream billing, or nil when
// it has none.
func GetChannelUpstreamBilling(channelId int) (*ChannelUpstreamBilling, error) {
	var billings []ChannelUpstreamBilling
	if err := DB.Where("channel_id = ?", channelId).Limit(1).Find(&billings).Error; err != nil {
		return nil, err
	}
	if len(billings) == 0 {
		return nil, nil
	}
	return &billings[0], nil
}

// SaveChannelUpstreamBilling replaces the channel's upstream billing.
func SaveChannelUpstreamBilling(billing *ChannelUpstreamBilling) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id = ?", billing.ChannelId).Delete(&ChannelUpstreamBilling{}).Error; err != nil {
			return err
		}
		return tx.Create(billing).Error
	})
}

func DeleteChannelUpstreamBilling(channelId int) error {
	return DB.Where("channel_id = ?", channelId).Delete(&ChannelUpstreamBilling{}).Error
}
