package operation_setting

import (
	"maps"
	"math"

	"github.com/QuantumNous/new-api/setting/config"
)

// MaxChannelCostRatio bounds the cost ratio an administrator can enter.
const MaxChannelCostRatio = 100

// ChannelCostSetting holds the cost ratios of the cost statistics page. A cost
// ratio is priced like a group ratio: cost = usage at the original price ×
// ratio. Channels without a ratio count their whole usage as cost.
type ChannelCostSetting struct {
	ChannelRatios map[int]float64 `json:"channel_ratios"`
}

var channelCostSetting = ChannelCostSetting{
	ChannelRatios: map[int]float64{},
}

func init() {
	config.GlobalConfig.Register("channel_cost_setting", &channelCostSetting)
}

// GetChannelCostSetting returns a copy of the current cost ratios.
func GetChannelCostSetting() ChannelCostSetting {
	setting := channelCostSetting
	setting.ChannelRatios = maps.Clone(channelCostSetting.ChannelRatios)
	if setting.ChannelRatios == nil {
		setting.ChannelRatios = map[int]float64{}
	}
	return setting
}

// IsValidChannelCostRatio reports whether ratio can be saved.
func IsValidChannelCostRatio(ratio float64) bool {
	return !math.IsNaN(ratio) && !math.IsInf(ratio, 0) && ratio >= 0 && ratio <= MaxChannelCostRatio
}
