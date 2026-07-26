package setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRateLimitPolicyFixtures(
	t *testing.T,
	groups map[string][2]int,
	users map[int][2]int,
) {
	t.Helper()

	ModelRequestRateLimitMutex.Lock()
	originalGroups := make(map[string][2]int, len(ModelRequestRateLimitGroup))
	for key, value := range ModelRequestRateLimitGroup {
		originalGroups[key] = value
	}
	originalUsers := make(map[int][2]int, len(ModelRequestRateLimitUser))
	for key, value := range ModelRequestRateLimitUser {
		originalUsers[key] = value
	}
	ModelRequestRateLimitGroup = groups
	ModelRequestRateLimitUser = users
	ModelRequestRateLimitMutex.Unlock()

	t.Cleanup(func() {
		ModelRequestRateLimitMutex.Lock()
		ModelRequestRateLimitGroup = originalGroups
		ModelRequestRateLimitUser = originalUsers
		ModelRequestRateLimitMutex.Unlock()
	})
}

func TestResolveModelRequestRateLimitPrefersUserThenGroup(t *testing.T) {
	setRateLimitPolicyFixtures(
		t,
		map[string][2]int{"vip": {200, 100}},
		map[int][2]int{42: {20, 10}},
	)

	total, success := ResolveModelRequestRateLimit(42, "vip", 500, 300)
	assert.Equal(t, 20, total)
	assert.Equal(t, 10, success)

	total, success = ResolveModelRequestRateLimit(7, "vip", 500, 300)
	assert.Equal(t, 200, total)
	assert.Equal(t, 100, success)

	total, success = ResolveModelRequestRateLimit(7, "standard", 500, 300)
	assert.Equal(t, 500, total)
	assert.Equal(t, 300, success)
}

func TestRateLimitMapUpdateReplacesOnlyAfterValidation(t *testing.T) {
	setRateLimitPolicyFixtures(
		t,
		map[string][2]int{"old": {10, 5}},
		map[int][2]int{9: {30, 20}},
	)

	require.NoError(t, UpdateModelRequestRateLimitGroupByJSONString(`{"vip":[200,100]}`))
	require.NoError(t, UpdateModelRequestRateLimitUserByJSONString(`{"42":[20,10]}`))

	groupTotal, groupSuccess, groupFound := GetGroupRateLimit("vip")
	assert.True(t, groupFound)
	assert.Equal(t, 200, groupTotal)
	assert.Equal(t, 100, groupSuccess)

	userTotal, userSuccess, userFound := GetUserRateLimit(42)
	assert.True(t, userFound)
	assert.Equal(t, 20, userTotal)
	assert.Equal(t, 10, userSuccess)

	require.Error(t, UpdateModelRequestRateLimitGroupByJSONString(`{"broken":[1]}`))
	require.Error(t, UpdateModelRequestRateLimitUserByJSONString(`{"broken":[1,1]}`))

	_, _, oldGroupFound := GetGroupRateLimit("vip")
	_, _, oldUserFound := GetUserRateLimit(42)
	assert.True(t, oldGroupFound)
	assert.True(t, oldUserFound)
}

func TestCheckModelRequestRateLimitUserRejectsInvalidIDsAndLimits(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "non numeric id", value: `{"alice":[20,10]}`},
		{name: "zero id", value: `{"0":[20,10]}`},
		{name: "negative id", value: `{"-1":[20,10]}`},
		{name: "non canonical id", value: `{"01":[20,10]}`},
		{name: "negative total", value: `{"42":[-1,10]}`},
		{name: "zero success", value: `{"42":[20,0]}`},
		{name: "overflow", value: `{"42":[2147483648,10]}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Error(t, CheckModelRequestRateLimitUser(test.value))
		})
	}

	require.NoError(t, CheckModelRequestRateLimitUser(`{"42":[0,10]}`))
}

func TestValidateDistillationRateLimitSettings(t *testing.T) {
	require.NoError(t, ValidateDistillationRateLimitSettings(DistillationRateLimitSettings{}))
	require.NoError(t, ValidateDistillationRateLimitSettings(DistillationRateLimitSettings{
		Enabled:            true,
		Threshold:          60,
		RPM:                10,
		PenaltyMinutes:     30,
		ObservationMinutes: 1440,
	}))

	tests := []struct {
		name     string
		settings DistillationRateLimitSettings
	}{
		{
			name: "enabled zero threshold",
			settings: DistillationRateLimitSettings{
				Enabled: true, RPM: 10, PenaltyMinutes: 30, ObservationMinutes: 1440,
			},
		},
		{
			name: "rpm equals threshold",
			settings: DistillationRateLimitSettings{
				Enabled: true, Threshold: 10, RPM: 10, PenaltyMinutes: 30, ObservationMinutes: 1440,
			},
		},
		{
			name: "rpm exceeds threshold",
			settings: DistillationRateLimitSettings{
				Enabled: true, Threshold: 10, RPM: 11, PenaltyMinutes: 30, ObservationMinutes: 1440,
			},
		},
		{
			name: "zero penalty",
			settings: DistillationRateLimitSettings{
				Enabled: true, Threshold: 60, RPM: 10, ObservationMinutes: 1440,
			},
		},
		{
			name: "zero observation",
			settings: DistillationRateLimitSettings{
				Enabled: true, Threshold: 60, RPM: 10, PenaltyMinutes: 30,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Error(t, ValidateDistillationRateLimitSettings(test.settings))
		})
	}
}
