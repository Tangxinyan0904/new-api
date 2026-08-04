package service

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildWalletSpecialRatioRulesFiltersAndSorts(t *testing.T) {
	got := buildWalletSpecialRatioRules(
		map[string]float64{"default": 1, "premium": 0.5},
		map[string]map[string]float64{
			"vip":   {"premium": 0.3, "default": 0.8},
			"staff": {"premium": math.Inf(1)},
		},
		map[string]map[string]bool{
			"vip":      {"premium": true, "default": true, "missing": true},
			"staff":    {"premium": true},
			"orphaned": {"default": true},
		},
	)

	assert.Equal(t, []WalletSpecialRatioRule{
		{UserGroup: "vip", BillingGroup: "default", SpecialRatio: 0.8, BaseRatio: 1},
		{UserGroup: "vip", BillingGroup: "premium", SpecialRatio: 0.3, BaseRatio: 0.5},
	}, got)
}

func TestBuildWalletSpecialRatioRulesWarnsOnceForEachInvalidVisiblePair(t *testing.T) {
	var output bytes.Buffer
	common.LogWriterMu.Lock()
	originalWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &output
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = originalWriter
		common.LogWriterMu.Unlock()
	})

	base := map[string]float64{
		"valid":             1,
		"missing-special":   1,
		"nonfinite-special": 1,
		"nonfinite-base":    math.Inf(1),
	}
	special := map[string]map[string]float64{
		"diagnostic-user": {
			"missing-base":      0.8,
			"nonfinite-special": math.NaN(),
			"nonfinite-base":    0.5,
		},
	}
	display := map[string]map[string]bool{
		"diagnostic-user": {
			"missing-special":   true,
			"missing-base":      true,
			"nonfinite-special": true,
			"nonfinite-base":    true,
			"hidden-missing":    false,
		},
		"missing-special-user": {"valid": true},
	}

	require.Empty(t, buildWalletSpecialRatioRules(base, special, display))
	require.Empty(t, buildWalletSpecialRatioRules(base, special, display))

	logs := output.String()
	warnings := []string{
		`user_group="diagnostic-user" billing_group="missing-special" reason="missing special ratio"`,
		`user_group="diagnostic-user" billing_group="missing-base" reason="missing base ratio"`,
		`user_group="diagnostic-user" billing_group="nonfinite-special" reason="non-finite special ratio"`,
		`user_group="diagnostic-user" billing_group="nonfinite-base" reason="non-finite base ratio"`,
		`user_group="missing-special-user" billing_group="valid" reason="missing special ratio group"`,
	}
	for _, warning := range warnings {
		assert.Equal(t, 1, strings.Count(logs, warning), warning)
	}
	assert.NotContains(t, logs, "hidden-missing")
}
