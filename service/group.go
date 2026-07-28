package service

import (
	"math"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func GetUserUsableGroups(userGroup string) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	if userGroup != "" {
		specialSettings, b := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(userGroup)
		if b {
			// 处理特殊可用分组
			for specialGroup, desc := range specialSettings {
				if strings.HasPrefix(specialGroup, "-:") {
					// 移除分组
					groupToRemove := strings.TrimPrefix(specialGroup, "-:")
					delete(groupsCopy, groupToRemove)
				} else if strings.HasPrefix(specialGroup, "+:") {
					// 添加分组
					groupToAdd := strings.TrimPrefix(specialGroup, "+:")
					groupsCopy[groupToAdd] = desc
				} else {
					// 直接添加分组
					groupsCopy[specialGroup] = desc
				}
			}
		}
		// 如果userGroup不在UserUsableGroups中，返回UserUsableGroups + userGroup
		if _, ok := groupsCopy[userGroup]; !ok {
			groupsCopy[userGroup] = "用户分组"
		}
	}
	return groupsCopy
}

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	_, ok := GetUserUsableGroups(userGroup)[groupName]
	return ok
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	groups := GetUserUsableGroups(userGroup)
	autoGroups := make([]string, 0)
	for _, group := range setting.GetAutoGroups() {
		if _, ok := groups[group]; ok {
			autoGroups = append(autoGroups, group)
		}
	}
	return autoGroups
}

// GetUserGroupRatio 获取用户使用某个分组的倍率
// userGroup 用户分组
// group 需要获取倍率的分组
func GetUserGroupRatio(userGroup, group string) float64 {
	ratio, ok := ratio_setting.GetGroupGroupRatio(userGroup, group)
	if ok {
		return ratio
	}
	return ratio_setting.GetGroupRatio(group)
}

type WalletSpecialRatioRule struct {
	UserGroup    string  `json:"user_group"`
	BillingGroup string  `json:"billing_group"`
	SpecialRatio float64 `json:"special_ratio"`
	BaseRatio    float64 `json:"base_ratio"`
}

func GetWalletSpecialRatioRules() []WalletSpecialRatioRule {
	snapshot := ratio_setting.GetWalletRatioSettingsSnapshot()
	return buildWalletSpecialRatioRules(
		snapshot.BaseRatios,
		snapshot.SpecialRatios,
		snapshot.WalletDisplay,
	)
}

func buildWalletSpecialRatioRules(
	base map[string]float64,
	special map[string]map[string]float64,
	display map[string]map[string]bool,
) []WalletSpecialRatioRule {
	rules := make([]WalletSpecialRatioRule, 0)
	for userGroup, targets := range display {
		userRatios, ok := special[userGroup]
		if !ok {
			continue
		}
		for billingGroup, visible := range targets {
			specialRatio, specialOK := userRatios[billingGroup]
			baseRatio, baseOK := base[billingGroup]
			if !visible || !specialOK || !baseOK ||
				math.IsNaN(specialRatio) || math.IsInf(specialRatio, 0) ||
				math.IsNaN(baseRatio) || math.IsInf(baseRatio, 0) {
				continue
			}
			rules = append(rules, WalletSpecialRatioRule{
				UserGroup:    userGroup,
				BillingGroup: billingGroup,
				SpecialRatio: specialRatio,
				BaseRatio:    baseRatio,
			})
		}
	}
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].UserGroup == rules[j].UserGroup {
			return rules[i].BillingGroup < rules[j].BillingGroup
		}
		return rules[i].UserGroup < rules[j].UserGroup
	})
	return rules
}
