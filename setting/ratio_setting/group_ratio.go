package ratio_setting

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/types"
)

const GroupGroupRatioWalletDisplayOption = "group_ratio_setting.group_group_ratio_wallet_display"

var defaultGroupRatio = map[string]float64{
	"default": 1,
	"vip":     1,
	"svip":    1,
}

var groupRatioMap = types.NewRWMap[string, float64]()

var defaultGroupGroupRatio = map[string]map[string]float64{
	"vip": {
		"edit_this": 0.9,
	},
}

var groupGroupRatioMap = types.NewRWMap[string, map[string]float64]()

var groupGroupRatioWalletDisplayMap = types.NewRWMap[string, map[string]bool]()

var groupPricingSnapshotMutex sync.RWMutex

var defaultGroupSpecialUsableGroup = map[string]map[string]string{}

type GroupRatioSetting struct {
	GroupRatio                   *types.RWMap[string, float64]            `json:"group_ratio"`
	GroupGroupRatio              *types.RWMap[string, map[string]float64] `json:"group_group_ratio"`
	GroupGroupRatioWalletDisplay *types.RWMap[string, map[string]bool]    `json:"group_group_ratio_wallet_display"`
	GroupSpecialUsableGroup      *types.RWMap[string, map[string]string]  `json:"group_special_usable_group"`
}

var groupRatioSetting GroupRatioSetting

func init() {
	groupSpecialUsableGroup := types.NewRWMap[string, map[string]string]()
	groupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)

	groupRatioMap.AddAll(defaultGroupRatio)
	groupGroupRatioMap.AddAll(defaultGroupGroupRatio)
	groupGroupRatioWalletDisplayMap.AddAll(map[string]map[string]bool{})

	groupRatioSetting = GroupRatioSetting{
		GroupSpecialUsableGroup:      groupSpecialUsableGroup,
		GroupRatio:                   groupRatioMap,
		GroupGroupRatio:              groupGroupRatioMap,
		GroupGroupRatioWalletDisplay: groupGroupRatioWalletDisplayMap,
	}

	config.GlobalConfig.Register("group_ratio_setting", &groupRatioSetting)
}

func GetGroupRatioSetting() *GroupRatioSetting {
	groupPricingSnapshotMutex.Lock()
	defer groupPricingSnapshotMutex.Unlock()

	if groupRatioSetting.GroupSpecialUsableGroup == nil {
		groupRatioSetting.GroupSpecialUsableGroup = types.NewRWMap[string, map[string]string]()
		groupRatioSetting.GroupSpecialUsableGroup.AddAll(defaultGroupSpecialUsableGroup)
	}
	if groupRatioSetting.GroupGroupRatioWalletDisplay == nil {
		groupRatioSetting.GroupGroupRatioWalletDisplay = groupGroupRatioWalletDisplayMap
	}
	return &groupRatioSetting
}

func GetGroupRatioCopy() map[string]float64 {
	return groupRatioMap.ReadAll()
}

func ContainsGroupRatio(name string) bool {
	_, ok := groupRatioMap.Get(name)
	return ok
}

func GroupRatio2JSONString() string {
	return groupRatioMap.MarshalJSONString()
}

func UpdateGroupRatioByJSONString(jsonStr string) error {
	groupPricingSnapshotMutex.Lock()
	defer groupPricingSnapshotMutex.Unlock()
	return types.LoadFromJsonString(groupRatioMap, jsonStr)
}

func GetGroupRatio(name string) float64 {
	ratio, ok := groupRatioMap.Get(name)
	if !ok {
		common.SysLog("group ratio not found: " + name)
		return 1
	}
	return ratio
}

func GetGroupGroupRatio(userGroup, usingGroup string) (float64, bool) {
	gp, ok := groupGroupRatioMap.Get(userGroup)
	if !ok {
		return -1, false
	}
	ratio, ok := gp[usingGroup]
	if !ok {
		return -1, false
	}
	return ratio, true
}

func GroupGroupRatio2JSONString() string {
	return groupGroupRatioMap.MarshalJSONString()
}

func UpdateGroupGroupRatioByJSONString(jsonStr string) error {
	groupPricingSnapshotMutex.Lock()
	defer groupPricingSnapshotMutex.Unlock()
	return types.LoadFromJsonString(groupGroupRatioMap, jsonStr)
}

func parseGroupGroupRatioWalletDisplay(jsonStr string) (map[string]map[string]bool, error) {
	raw := make(map[string]map[string]bool)
	if err := common.UnmarshalJsonStr(jsonStr, &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, errors.New("wallet display rules must be a JSON object")
	}

	normalized := make(map[string]map[string]bool)
	for userGroup, targets := range raw {
		if strings.TrimSpace(userGroup) == "" {
			return nil, errors.New("wallet display user group must not be empty")
		}
		if targets == nil {
			return nil, errors.New("wallet display billing groups must be a JSON object")
		}
		for billingGroup, visible := range targets {
			if strings.TrimSpace(billingGroup) == "" {
				return nil, errors.New("wallet display billing group must not be empty")
			}
			if !visible {
				continue
			}
			if normalized[userGroup] == nil {
				normalized[userGroup] = make(map[string]bool)
			}
			normalized[userGroup][billingGroup] = true
		}
	}
	return normalized, nil
}

func ValidateGroupGroupRatioWalletDisplay(jsonStr string) error {
	display, err := parseGroupGroupRatioWalletDisplay(jsonStr)
	if err != nil {
		return err
	}

	groupPricingSnapshotMutex.RLock()
	defer groupPricingSnapshotMutex.RUnlock()
	special := groupGroupRatioMap.ReadAll()
	for userGroup, targets := range display {
		ratios, ok := special[userGroup]
		if !ok {
			return fmt.Errorf("wallet display rule %s has no special ratio group", userGroup)
		}
		for billingGroup := range targets {
			if _, ok := ratios[billingGroup]; !ok {
				return fmt.Errorf("wallet display rule %s -> %s has no special ratio", userGroup, billingGroup)
			}
		}
	}
	return nil
}

func UpdateGroupGroupRatioWalletDisplayByJSONString(jsonStr string) error {
	normalized, err := parseGroupGroupRatioWalletDisplay(jsonStr)
	if err != nil {
		return err
	}
	encoded, err := common.Marshal(normalized)
	if err != nil {
		return err
	}

	groupPricingSnapshotMutex.Lock()
	defer groupPricingSnapshotMutex.Unlock()
	return types.LoadFromJsonString(groupGroupRatioWalletDisplayMap, string(encoded))
}

func GroupGroupRatioWalletDisplay2JSONString() string {
	groupPricingSnapshotMutex.RLock()
	defer groupPricingSnapshotMutex.RUnlock()
	return groupGroupRatioWalletDisplayMap.MarshalJSONString()
}

type WalletRatioSettingsSnapshot struct {
	BaseRatios    map[string]float64
	SpecialRatios map[string]map[string]float64
	WalletDisplay map[string]map[string]bool
}

func GetWalletRatioSettingsSnapshot() WalletRatioSettingsSnapshot {
	groupPricingSnapshotMutex.RLock()
	defer groupPricingSnapshotMutex.RUnlock()
	return WalletRatioSettingsSnapshot{
		BaseRatios:    copyFloatMap(groupRatioMap.ReadAll()),
		SpecialRatios: copyNestedFloatMap(groupGroupRatioMap.ReadAll()),
		WalletDisplay: copyNestedBoolMap(groupGroupRatioWalletDisplayMap.ReadAll()),
	}
}

func copyFloatMap(source map[string]float64) map[string]float64 {
	copyOfSource := make(map[string]float64, len(source))
	for key, value := range source {
		copyOfSource[key] = value
	}
	return copyOfSource
}

func copyNestedFloatMap(source map[string]map[string]float64) map[string]map[string]float64 {
	copyOfSource := make(map[string]map[string]float64, len(source))
	for key, values := range source {
		copyOfSource[key] = copyFloatMap(values)
	}
	return copyOfSource
}

func copyNestedBoolMap(source map[string]map[string]bool) map[string]map[string]bool {
	copyOfSource := make(map[string]map[string]bool, len(source))
	for key, values := range source {
		copiedValues := make(map[string]bool, len(values))
		for nestedKey, value := range values {
			copiedValues[nestedKey] = value
		}
		copyOfSource[key] = copiedValues
	}
	return copyOfSource
}

func CheckGroupRatio(jsonStr string) error {
	checkGroupRatio := make(map[string]float64)
	err := common.UnmarshalJsonStr(jsonStr, &checkGroupRatio)
	if err != nil {
		return err
	}
	for name, ratio := range checkGroupRatio {
		if ratio < 0 {
			return errors.New("group ratio must be not less than 0: " + name)
		}
	}
	return nil
}
