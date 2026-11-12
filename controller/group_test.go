package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
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
