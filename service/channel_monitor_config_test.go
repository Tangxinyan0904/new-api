package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeChannelMonitorInputAcceptsSupportedProviderModes(t *testing.T) {
	previousSecret := common.CryptoSecret
	common.CryptoSecret = "channel-monitor-normalization-secret"
	t.Cleanup(func() { common.CryptoSecret = previousSecret })

	tests := []struct {
		provider string
		mode     string
	}{
		{provider: model.ChannelMonitorProviderOpenAI, mode: model.ChannelMonitorAPIModeChatCompletions},
		{provider: model.ChannelMonitorProviderOpenAI, mode: model.ChannelMonitorAPIModeResponses},
		{provider: model.ChannelMonitorProviderAnthropic, mode: model.ChannelMonitorAPIModeMessages},
		{provider: model.ChannelMonitorProviderGemini, mode: model.ChannelMonitorAPIModeGenerateContent},
	}

	for _, test := range tests {
		t.Run(test.provider+"_"+test.mode, func(t *testing.T) {
			monitor, err := NormalizeChannelMonitorInput(ChannelMonitorInput{
				InternalName: " Internal ",
				PublicName:   " Public ",
				Provider:     test.provider,
				APIMode:      test.mode,
				Endpoint:     "https://Example.COM/gateway/",
				APIKey:       "monitor-secret",
				PrimaryModel: " model-primary ",
				ExtraModels:  []string{" model-extra "},
			}, nil, 1700)
			require.NoError(t, err)
			assert.Equal(t, "Internal", monitor.InternalName)
			assert.Equal(t, "Public", monitor.PublicName)
			assert.Equal(t, "https://example.com/gateway", monitor.Endpoint)
			assert.Equal(t, "model-primary", monitor.PrimaryModel)
			assert.Equal(t, 60, monitor.IntervalSeconds)
			assert.Equal(t, 6000, monitor.DegradedThresholdMs)
			assert.Equal(t, 30, monitor.TimeoutSeconds)
			assert.True(t, monitor.Enabled)
			assert.False(t, monitor.PublicEnabled)
			assert.Equal(t, int64(1700), monitor.NextCheckAt)
			assert.NotEmpty(t, monitor.APIKeyEncrypted)
			assert.Equal(t, "cret", monitor.APIKeySuffix)

			var extras []string
			require.NoError(t, common.UnmarshalJsonStr(monitor.ExtraModels, &extras))
			assert.Equal(t, []string{"model-extra"}, extras)
		})
	}
}

func TestNormalizeChannelMonitorInputRejectsUnsupportedProviderModes(t *testing.T) {
	tests := []struct {
		provider string
		mode     string
	}{
		{provider: model.ChannelMonitorProviderOpenAI, mode: model.ChannelMonitorAPIModeMessages},
		{provider: model.ChannelMonitorProviderAnthropic, mode: model.ChannelMonitorAPIModeChatCompletions},
		{provider: model.ChannelMonitorProviderGemini, mode: model.ChannelMonitorAPIModeResponses},
		{provider: "unknown", mode: model.ChannelMonitorAPIModeChatCompletions},
	}

	for _, test := range tests {
		t.Run(test.provider+"_"+test.mode, func(t *testing.T) {
			_, err := NormalizeChannelMonitorInput(validChannelMonitorInput(test.provider, test.mode), nil, 1)
			require.Error(t, err)
		})
	}
}

func TestNormalizeChannelMonitorInputRejectsInvalidEndpoints(t *testing.T) {
	endpoints := []string{
		"ftp://example.com",
		"https://user:password@example.com",
		"https://example.com?token=secret",
		"https://example.com#fragment",
		"https:///missing-host",
		"https://example.com/v1/chat/completions",
		"https://example.com/v1/responses",
		"https://example.com/v1/messages",
		"https://example.com/v1beta/models/gemini:generateContent",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			input := validChannelMonitorInput(model.ChannelMonitorProviderOpenAI, model.ChannelMonitorAPIModeChatCompletions)
			input.Endpoint = endpoint
			_, err := NormalizeChannelMonitorInput(input, nil, 1)
			require.Error(t, err)
		})
	}
}

func TestNormalizeChannelMonitorInputRejectsInvalidModelsAndRanges(t *testing.T) {
	intervalBelow := 59
	intervalAbove := 3601
	thresholdZero := 0
	timeoutZero := 0
	timeoutAbove := 121

	tests := []struct {
		name   string
		mutate func(*ChannelMonitorInput)
	}{
		{name: "empty primary", mutate: func(input *ChannelMonitorInput) { input.PrimaryModel = " " }},
		{name: "duplicate primary", mutate: func(input *ChannelMonitorInput) { input.ExtraModels = []string{"gpt-test"} }},
		{name: "duplicate extras", mutate: func(input *ChannelMonitorInput) { input.ExtraModels = []string{"a", " a "} }},
		{name: "too many models", mutate: func(input *ChannelMonitorInput) {
			input.ExtraModels = make([]string, 20)
			for i := range input.ExtraModels {
				input.ExtraModels[i] = channelMonitorTestModelName(i)
			}
		}},
		{name: "interval below", mutate: func(input *ChannelMonitorInput) { input.IntervalSeconds = &intervalBelow }},
		{name: "interval above", mutate: func(input *ChannelMonitorInput) { input.IntervalSeconds = &intervalAbove }},
		{name: "threshold zero", mutate: func(input *ChannelMonitorInput) { input.DegradedThresholdMs = &thresholdZero }},
		{name: "timeout zero", mutate: func(input *ChannelMonitorInput) { input.TimeoutSeconds = &timeoutZero }},
		{name: "timeout above", mutate: func(input *ChannelMonitorInput) { input.TimeoutSeconds = &timeoutAbove }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validChannelMonitorInput(model.ChannelMonitorProviderOpenAI, model.ChannelMonitorAPIModeChatCompletions)
			test.mutate(&input)
			_, err := NormalizeChannelMonitorInput(input, nil, 1)
			require.Error(t, err)
		})
	}
}

func TestNormalizeChannelMonitorInputPreservesOrReplacesCredentialOnUpdate(t *testing.T) {
	previousSecret := common.CryptoSecret
	common.CryptoSecret = "channel-monitor-update-secret"
	t.Cleanup(func() { common.CryptoSecret = previousSecret })

	current := &model.ChannelMonitor{
		ID:                    7,
		APIKeyEncrypted:       "existing-ciphertext",
		APIKeySuffix:          "old1",
		IntervalSeconds:       60,
		DegradedThresholdMs:   6000,
		TimeoutSeconds:        30,
		ScheduleBlockedReason: "credential_decryption_failed",
		LeaseOwner:            "node-a",
		LeaseUntil:            3000,
		NextCheckAt:           0,
		Enabled:               true,
		CreatedAt:             100,
		CreatedBy:             9,
	}

	input := validChannelMonitorInput(model.ChannelMonitorProviderOpenAI, model.ChannelMonitorAPIModeResponses)
	input.APIKey = ""
	preserved, err := NormalizeChannelMonitorInput(input, current, 1700)
	require.NoError(t, err)
	assert.Equal(t, "existing-ciphertext", preserved.APIKeyEncrypted)
	assert.Equal(t, "old1", preserved.APIKeySuffix)
	assert.Equal(t, "credential_decryption_failed", preserved.ScheduleBlockedReason)
	assert.Equal(t, "node-a", preserved.LeaseOwner)
	assert.Equal(t, int64(3000), preserved.LeaseUntil)
	assert.Equal(t, int64(0), preserved.NextCheckAt)
	assert.Equal(t, 7, preserved.ID)
	assert.Equal(t, int64(100), preserved.CreatedAt)
	assert.Equal(t, 9, preserved.CreatedBy)

	input.APIKey = "replacement-secret"
	replaced, err := NormalizeChannelMonitorInput(input, current, 1800)
	require.NoError(t, err)
	assert.NotEqual(t, "existing-ciphertext", replaced.APIKeyEncrypted)
	assert.Equal(t, "cret", replaced.APIKeySuffix)
	assert.Empty(t, replaced.ScheduleBlockedReason)
	assert.Empty(t, replaced.LeaseOwner)
	assert.Zero(t, replaced.LeaseUntil)
	assert.Equal(t, int64(1800), replaced.NextCheckAt)
}

func TestNormalizeChannelMonitorInputRequiresCredentialWhenEnabled(t *testing.T) {
	disabled := false
	input := validChannelMonitorInput(model.ChannelMonitorProviderOpenAI, model.ChannelMonitorAPIModeChatCompletions)
	input.APIKey = ""
	input.Enabled = &disabled

	monitor, err := NormalizeChannelMonitorInput(input, nil, 1)
	require.NoError(t, err)
	assert.False(t, monitor.Enabled)
	assert.Zero(t, monitor.NextCheckAt)

	enabled := true
	input.Enabled = &enabled
	_, err = NormalizeChannelMonitorInput(input, nil, 1)
	require.Error(t, err)
}

func TestNormalizeChannelMonitorInputNameColor(t *testing.T) {
	t.Run("normalizes valid color", func(t *testing.T) {
		nameColor := "  #1a2b3c  "
		input := validChannelMonitorInput(model.ChannelMonitorProviderOpenAI, model.ChannelMonitorAPIModeChatCompletions)
		input.NameColor = &nameColor

		monitor, err := NormalizeChannelMonitorInput(input, nil, 1)
		require.NoError(t, err)
		assert.Equal(t, "#1A2B3C", monitor.NameColor)
	})

	t.Run("clears explicit empty color", func(t *testing.T) {
		nameColor := ""
		input := validChannelMonitorInput(model.ChannelMonitorProviderOpenAI, model.ChannelMonitorAPIModeChatCompletions)
		input.NameColor = &nameColor

		monitor, err := NormalizeChannelMonitorInput(input, &model.ChannelMonitor{
			NameColor:           "#ABCDEF",
			IntervalSeconds:     60,
			DegradedThresholdMs: 6000,
			TimeoutSeconds:      30,
		}, 1)
		require.NoError(t, err)
		assert.Empty(t, monitor.NameColor)
	})

	t.Run("preserves color when omitted", func(t *testing.T) {
		input := validChannelMonitorInput(model.ChannelMonitorProviderOpenAI, model.ChannelMonitorAPIModeChatCompletions)

		monitor, err := NormalizeChannelMonitorInput(input, &model.ChannelMonitor{
			NameColor:           "#ABCDEF",
			IntervalSeconds:     60,
			DegradedThresholdMs: 6000,
			TimeoutSeconds:      30,
		}, 1)
		require.NoError(t, err)
		assert.Equal(t, "#ABCDEF", monitor.NameColor)
	})

	for _, nameColor := range []string{"112233", "#123", "#12345G", "#1234567"} {
		t.Run("rejects "+nameColor, func(t *testing.T) {
			input := validChannelMonitorInput(model.ChannelMonitorProviderOpenAI, model.ChannelMonitorAPIModeChatCompletions)
			input.NameColor = &nameColor

			monitor, err := NormalizeChannelMonitorInput(input, nil, 1)
			require.Error(t, err)
			assert.Nil(t, monitor)
		})
	}
}

func validChannelMonitorInput(provider string, mode string) ChannelMonitorInput {
	return ChannelMonitorInput{
		InternalName: "Internal",
		PublicName:   "Public",
		Provider:     provider,
		APIMode:      mode,
		Endpoint:     "https://example.com",
		APIKey:       "monitor-secret",
		PrimaryModel: "gpt-test",
	}
}

func channelMonitorTestModelName(index int) string {
	return "model-" + string(rune('a'+index))
}
