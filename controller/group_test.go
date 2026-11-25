package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetGroupRatiosListsBaseAndSpecialRatios(t *testing.T) {
	originalRatios := ratio_setting.GroupRatio2JSONString()
	originalSpecialRatios := ratio_setting.GroupGroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"cheap":0.5}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"vip":{"cheap":0.4}}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(originalRatios))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(originalSpecialRatios))
	})

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/group/ratios", nil)
	GetGroupRatios(c)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			GroupRatio      map[string]float64            `json:"group_ratio"`
			GroupGroupRatio map[string]map[string]float64 `json:"group_group_ratio"`
		} `json:"data"`
	}
	require.Equal(t, http.StatusOK, recorder.Code)
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, map[string]float64{"default": 1, "cheap": 0.5}, response.Data.GroupRatio)
	assert.Equal(t, map[string]map[string]float64{"vip": {"cheap": 0.4}}, response.Data.GroupGroupRatio)
}

func TestGroupAndPricingAPIsShowOnlyTheViewersApplicableOriginalRatios(t *testing.T) {
	base, special, usable := ratio_setting.GroupRatio2JSONString(), ratio_setting.GroupGroupRatio2JSONString(), setting.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(base))
		require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(special))
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(usable))
	})
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","free":"Free","premium":"Premium","same":"Same","auto":"Auto"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"free":3,"premium":0,"same":1,"hidden":2}`))
	require.NoError(t, ratio_setting.UpdateGroupGroupRatioByJSONString(`{"member":{"default":0.5,"free":0,"premium":2,"same":1,"hidden":0.1,"auto":0.5},"other":{"same":0.2},"":{"default":0.25}}`))
	db := setupModelListControllerTestDB(t)
	member := model.User{Username: "ratio-member", Group: "member"}
	require.NoError(t, db.Create(&member).Error)
	model.InvalidatePricingCache()
	t.Cleanup(model.InvalidatePricingCache)

	for _, tc := range []struct {
		name, group       string
		id                int
		ratios, originals map[string]float64
	}{
		{"member", member.Group, member.Id, map[string]float64{"default": 0.5, "free": 0, "premium": 2, "same": 1}, map[string]float64{"default": 1, "free": 3, "premium": 0}},
		{"guest", "", 0, map[string]float64{"default": 1, "free": 3, "premium": 0, "same": 1}, map[string]float64{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pricing := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(pricing)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/pricing", nil)
			common.SetContextKey(c, constant.ContextKeyUserGroup, tc.group)
			GetPricing(c)
			var prices struct {
				GroupRatio         map[string]float64 `json:"group_ratio"`
				OriginalGroupRatio map[string]float64 `json:"original_group_ratio"`
			}
			require.Equal(t, http.StatusOK, pricing.Code)
			require.NoError(t, common.Unmarshal(pricing.Body.Bytes(), &prices))
			assert.Equal(t, tc.ratios, prices.GroupRatio)
			assert.Equal(t, tc.originals, prices.OriginalGroupRatio)

			groups := httptest.NewRecorder()
			c, _ = gin.CreateTestContext(groups)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/user/self/groups", nil)
			c.Set("id", tc.id)
			common.SetContextKey(c, constant.ContextKeyUserGroup, tc.group)
			GetUserGroups(c)
			var result struct {
				Data map[string]map[string]any `json:"data"`
			}
			require.Equal(t, http.StatusOK, groups.Code)
			require.NoError(t, common.Unmarshal(groups.Body.Bytes(), &result))
			require.Len(t, result.Data, len(tc.ratios)+1)
			assert.Equal(t, "auto", result.Data["auto"]["ratio"])
			assert.NotContains(t, result.Data["auto"], "original_ratio")
			for group, effective := range tc.ratios {
				require.Contains(t, result.Data, group)
				assert.Equal(t, effective, result.Data[group]["ratio"])
				if original, ok := tc.originals[group]; ok {
					assert.Equal(t, original, result.Data[group]["original_ratio"])
				} else {
					assert.NotContains(t, result.Data[group], "original_ratio")
				}
			}
		})
	}
}
