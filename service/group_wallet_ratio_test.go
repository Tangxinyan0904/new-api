package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
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
