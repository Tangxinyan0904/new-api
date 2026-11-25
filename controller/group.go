package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

// GetGroupRatios lists what each group charges: its own ratio, and the special
// ratios that users of a given group pay when billed as it (keyed by user
// group, then by billing group).
func GetGroupRatios(c *gin.Context) {
	snapshot := ratio_setting.GetWalletRatioSettingsSnapshot()
	common.ApiSuccess(c, gin.H{
		"group_ratio":       snapshot.BaseRatios,
		"group_group_ratio": snapshot.SpecialRatios,
	})
}

func GetUserGroups(c *gin.Context) {
	usableGroups := make(map[string]map[string]any)
	userGroup := ""
	if userID := c.GetInt("id"); userID > 0 {
		userGroup, _ = model.GetUserGroup(userID, false)
	}
	userUsableGroups := service.GetUserUsableGroups(userGroup)
	ratios, originals := ratio_setting.GetVisibleGroupRatios(userGroup, userUsableGroups)
	for groupName, ratio := range ratios {
		details := map[string]any{"ratio": ratio, "desc": userUsableGroups[groupName]}
		if original, ok := originals[groupName]; ok {
			details["original_ratio"] = original
		}
		usableGroups[groupName] = details
	}
	if _, ok := userUsableGroups["auto"]; ok {
		usableGroups["auto"] = map[string]any{
			"ratio": "auto",
			"desc":  setting.GetUsableGroupDescription("auto"),
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}

func GetWalletSpecialRatioRules(c *gin.Context) {
	common.ApiSuccess(c, service.GetWalletSpecialRatioRules())
}
