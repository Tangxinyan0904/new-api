package service

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	channelMonitorDefaultIntervalSeconds     = 60
	channelMonitorDefaultDegradedThresholdMs = 6000
	channelMonitorDefaultTimeoutSeconds      = 30
	channelMonitorMinIntervalSeconds         = 60
	channelMonitorMaxIntervalSeconds         = 3600
	channelMonitorMaxTimeoutSeconds          = 120
	channelMonitorMaxModels                  = 20
)

type ChannelMonitorInput struct {
	InternalName        string   `json:"internal_name"`
	PublicName          string   `json:"public_name"`
	Provider            string   `json:"provider"`
	APIMode             string   `json:"api_mode"`
	Endpoint            string   `json:"endpoint"`
	APIKey              string   `json:"api_key"`
	PrimaryModel        string   `json:"primary_model"`
	ExtraModels         []string `json:"extra_models"`
	IntervalSeconds     *int     `json:"interval_seconds"`
	DegradedThresholdMs *int     `json:"degraded_threshold_ms"`
	TimeoutSeconds      *int     `json:"timeout_seconds"`
	Enabled             *bool    `json:"enabled"`
	PublicEnabled       *bool    `json:"public_enabled"`
}

func NormalizeChannelMonitorInput(input ChannelMonitorInput, current *model.ChannelMonitor, now int64) (*model.ChannelMonitor, error) {
	monitor := &model.ChannelMonitor{}
	if current != nil {
		*monitor = *current
	}

	monitor.InternalName = strings.TrimSpace(input.InternalName)
	if monitor.InternalName == "" {
		return nil, errors.New("channel monitor internal name is required")
	}
	if len([]rune(monitor.InternalName)) > 128 {
		return nil, errors.New("channel monitor internal name is too long")
	}

	monitor.PublicName = strings.TrimSpace(input.PublicName)
	if monitor.PublicName == "" {
		return nil, errors.New("channel monitor public name is required")
	}
	if len([]rune(monitor.PublicName)) > 128 {
		return nil, errors.New("channel monitor public name is too long")
	}

	monitor.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
	monitor.APIMode = strings.ToLower(strings.TrimSpace(input.APIMode))
	if !isSupportedChannelMonitorMode(monitor.Provider, monitor.APIMode) {
		return nil, errors.New("unsupported channel monitor provider and API mode")
	}

	endpoint, err := normalizeChannelMonitorEndpoint(input.Endpoint)
	if err != nil {
		return nil, err
	}
	monitor.Endpoint = endpoint

	monitor.PrimaryModel = strings.TrimSpace(input.PrimaryModel)
	if monitor.PrimaryModel == "" {
		return nil, errors.New("channel monitor primary model is required")
	}
	if len([]rune(monitor.PrimaryModel)) > 256 {
		return nil, errors.New("channel monitor primary model is too long")
	}

	models := make(map[string]struct{}, len(input.ExtraModels)+1)
	models[monitor.PrimaryModel] = struct{}{}
	extraModels := make([]string, 0, len(input.ExtraModels))
	for _, rawModel := range input.ExtraModels {
		modelName := strings.TrimSpace(rawModel)
		if modelName == "" {
			return nil, errors.New("channel monitor extra model cannot be empty")
		}
		if len([]rune(modelName)) > 256 {
			return nil, errors.New("channel monitor extra model is too long")
		}
		if _, exists := models[modelName]; exists {
			return nil, fmt.Errorf("duplicate channel monitor model: %s", modelName)
		}
		models[modelName] = struct{}{}
		extraModels = append(extraModels, modelName)
	}
	if len(models) > channelMonitorMaxModels {
		return nil, fmt.Errorf("channel monitor supports at most %d models", channelMonitorMaxModels)
	}
	extraModelsJSON, err := common.Marshal(extraModels)
	if err != nil {
		return nil, fmt.Errorf("encode channel monitor extra models: %w", err)
	}
	monitor.ExtraModels = string(extraModelsJSON)

	monitor.IntervalSeconds = channelMonitorDefaultIntervalSeconds
	if current != nil {
		monitor.IntervalSeconds = current.IntervalSeconds
	}
	if input.IntervalSeconds != nil {
		monitor.IntervalSeconds = *input.IntervalSeconds
	}
	if monitor.IntervalSeconds < channelMonitorMinIntervalSeconds || monitor.IntervalSeconds > channelMonitorMaxIntervalSeconds {
		return nil, fmt.Errorf("channel monitor interval must be between %d and %d seconds", channelMonitorMinIntervalSeconds, channelMonitorMaxIntervalSeconds)
	}

	monitor.DegradedThresholdMs = channelMonitorDefaultDegradedThresholdMs
	if current != nil {
		monitor.DegradedThresholdMs = current.DegradedThresholdMs
	}
	if input.DegradedThresholdMs != nil {
		monitor.DegradedThresholdMs = *input.DegradedThresholdMs
	}
	if monitor.DegradedThresholdMs <= 0 {
		return nil, errors.New("channel monitor degraded threshold must be positive")
	}

	monitor.TimeoutSeconds = channelMonitorDefaultTimeoutSeconds
	if current != nil {
		monitor.TimeoutSeconds = current.TimeoutSeconds
	}
	if input.TimeoutSeconds != nil {
		monitor.TimeoutSeconds = *input.TimeoutSeconds
	}
	if monitor.TimeoutSeconds <= 0 || monitor.TimeoutSeconds > channelMonitorMaxTimeoutSeconds {
		return nil, fmt.Errorf("channel monitor timeout must be between 1 and %d seconds", channelMonitorMaxTimeoutSeconds)
	}

	monitor.Enabled = true
	monitor.PublicEnabled = false
	if current != nil {
		monitor.Enabled = current.Enabled
		monitor.PublicEnabled = current.PublicEnabled
	}
	if input.Enabled != nil {
		monitor.Enabled = *input.Enabled
	}
	if input.PublicEnabled != nil {
		monitor.PublicEnabled = *input.PublicEnabled
	}

	credential := strings.TrimSpace(input.APIKey)
	credentialReplaced := credential != ""
	if credentialReplaced {
		monitor.APIKeyEncrypted, monitor.APIKeySuffix, err = EncryptChannelMonitorCredential(credential)
		if err != nil {
			return nil, err
		}
		monitor.ScheduleBlockedReason = ""
		monitor.LeaseOwner = ""
		monitor.LeaseUntil = 0
	}
	if monitor.Enabled && monitor.APIKeyEncrypted == "" {
		return nil, errors.New("channel monitor credential is required when enabled")
	}

	if !monitor.Enabled {
		monitor.NextCheckAt = 0
		monitor.LeaseOwner = ""
		monitor.LeaseUntil = 0
	} else if monitor.ScheduleBlockedReason == "" && (current == nil || !current.Enabled || credentialReplaced) {
		monitor.NextCheckAt = now
	}

	return monitor, nil
}

func isSupportedChannelMonitorMode(provider string, apiMode string) bool {
	switch provider {
	case model.ChannelMonitorProviderOpenAI:
		return apiMode == model.ChannelMonitorAPIModeChatCompletions || apiMode == model.ChannelMonitorAPIModeResponses
	case model.ChannelMonitorProviderAnthropic:
		return apiMode == model.ChannelMonitorAPIModeMessages
	case model.ChannelMonitorProviderGemini:
		return apiMode == model.ChannelMonitorAPIModeGenerateContent
	default:
		return false
	}
}

func normalizeChannelMonitorEndpoint(rawEndpoint string) (string, error) {
	endpoint := strings.TrimSpace(rawEndpoint)
	if endpoint == "" {
		return "", errors.New("channel monitor endpoint is required")
	}
	if len(endpoint) > 2048 {
		return "", errors.New("channel monitor endpoint is too long")
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", errors.New("invalid channel monitor endpoint")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("channel monitor endpoint must use HTTP or HTTPS")
	}
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", errors.New("channel monitor endpoint must be a base URL without credentials, query, or fragment")
	}
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")

	path := strings.ToLower(parsed.Path)
	protocolSuffixes := []string{
		"/v1/chat/completions",
		"/chat/completions",
		"/v1/responses",
		"/responses",
		"/v1/messages",
		"/messages",
	}
	for _, suffix := range protocolSuffixes {
		if strings.HasSuffix(path, suffix) {
			return "", errors.New("channel monitor endpoint must not include the protocol request path")
		}
	}
	if strings.HasSuffix(path, ":generatecontent") {
		return "", errors.New("channel monitor endpoint must not include the protocol request path")
	}

	return parsed.String(), nil
}
