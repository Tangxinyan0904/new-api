package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalletDisplayRulesNormalizeAndReplace(t *testing.T) {
	originalRatios := GroupGroupRatio2JSONString()
	originalDisplay := GroupGroupRatioWalletDisplay2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupGroupRatioByJSONString(originalRatios))
		require.NoError(t, UpdateGroupGroupRatioWalletDisplayByJSONString(originalDisplay))
	})

	require.NoError(t, UpdateGroupGroupRatioByJSONString(`{"vip":{"premium":0.3},"staff":{"default":0.8}}`))
	require.NoError(t, UpdateGroupGroupRatioWalletDisplayByJSONString(`{"vip":{"premium":true},"staff":{"default":false}}`))
	assert.JSONEq(t, `{"vip":{"premium":true}}`, GroupGroupRatioWalletDisplay2JSONString())

	require.NoError(t, UpdateGroupGroupRatioWalletDisplayByJSONString(`{}`))
	assert.JSONEq(t, `{}`, GroupGroupRatioWalletDisplay2JSONString())
}

func TestValidateWalletDisplayRulesAgainstSpecialRatios(t *testing.T) {
	originalRatios := GroupGroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, UpdateGroupGroupRatioByJSONString(originalRatios))
	})

	require.NoError(t, UpdateGroupGroupRatioByJSONString(`{"vip":{"premium":0.3}}`))
	require.NoError(t, ValidateGroupGroupRatioWalletDisplay(`{"vip":{"premium":true}}`))
	assert.ErrorContains(t, ValidateGroupGroupRatioWalletDisplay(`{"vip":{"missing":true}}`), "vip -> missing")
	assert.ErrorContains(t, ValidateGroupGroupRatioWalletDisplay(`{"":{"premium":true}}`), "user group")
	assert.Error(t, ValidateGroupGroupRatioWalletDisplay(`[]`))
}
