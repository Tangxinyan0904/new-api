package model

import (
	"strings"

	"gorm.io/gorm"
)

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
	// Every new-api channel on the same upstream address shares them. The
	// token is never sent back to the browser.
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

// UpstreamAddress is the upstream a channel address points at; addresses that
// differ only by a trailing slash are the same upstream.
func UpstreamAddress(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/")
}

// NewAPIUpstreamLogin returns the most recently saved new-api billing that
// holds an access token for the upstream address, or nil when there is none.
func NewAPIUpstreamLogin(billings map[int]ChannelUpstreamBilling, address string) *ChannelUpstreamBilling {
	var login *ChannelUpstreamBilling
	for _, billing := range billings {
		if billing.Kind != UpstreamBillingNewAPI || billing.AccessToken == "" || UpstreamAddress(billing.BaseURL) != address {
			continue
		}
		if login == nil || billing.UpdatedAt > login.UpdatedAt ||
			(billing.UpdatedAt == login.UpdatedAt && billing.ChannelId < login.ChannelId) {
			login = &billing
		}
	}
	return login
}

// SaveChannelUpstreamBilling replaces the channel's upstream billing. A new-api
// login is also written to the other new-api channels on the same upstream
// address, which share it.
func SaveChannelUpstreamBilling(billing *ChannelUpstreamBilling) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id = ?", billing.ChannelId).Delete(&ChannelUpstreamBilling{}).Error; err != nil {
			return err
		}
		if err := tx.Create(billing).Error; err != nil {
			return err
		}
		if billing.Kind != UpstreamBillingNewAPI {
			return nil
		}
		var others []ChannelUpstreamBilling
		if err := tx.Where("kind = ?", UpstreamBillingNewAPI).Where("channel_id <> ?", billing.ChannelId).Find(&others).Error; err != nil {
			return err
		}
		sharing := make([]int, 0)
		for _, other := range others {
			if UpstreamAddress(other.BaseURL) == UpstreamAddress(billing.BaseURL) {
				sharing = append(sharing, other.ChannelId)
			}
		}
		if len(sharing) == 0 {
			return nil
		}
		return tx.Model(&ChannelUpstreamBilling{}).Where("channel_id IN ?", sharing).Updates(map[string]any{
			"access_token":     billing.AccessToken,
			"upstream_user_id": billing.UpstreamUserId,
			"updated_at":       billing.UpdatedAt,
		}).Error
	})
}

func DeleteChannelUpstreamBilling(channelId int) error {
	return DB.Where("channel_id = ?", channelId).Delete(&ChannelUpstreamBilling{}).Error
}
