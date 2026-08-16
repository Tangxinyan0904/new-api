package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	channelMonitorProbeChallenge        = "NEWAPI_MONITOR_OK"
	channelMonitorProbePrompt           = "Reply with exactly NEWAPI_MONITOR_OK and nothing else."
	channelMonitorProbeMaxOutputTokens  = 16
	channelMonitorProbeMaxResponseBytes = 256 * 1024
	channelMonitorProbeMaxErrorBytes    = 512
)

var channelMonitorProbeNow = time.Now

type ChannelMonitorProbeResult struct {
	Status        string `json:"status"`
	LatencyMs     int64  `json:"latency_ms"`
	ErrorCategory string `json:"error_category"`
	HTTPStatus    int    `json:"http_status"`
	ErrorMessage  string `json:"error_message"`
}

type channelMonitorMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type channelMonitorChatRequest struct {
	Model       string                  `json:"model"`
	Messages    []channelMonitorMessage `json:"messages"`
	MaxTokens   int                     `json:"max_tokens"`
	Temperature float64                 `json:"temperature"`
	Stream      bool                    `json:"stream"`
}

type channelMonitorResponsesRequest struct {
	Model           string `json:"model"`
	Input           string `json:"input"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	Stream          bool   `json:"stream"`
}

type channelMonitorAnthropicRequest struct {
	Model       string                  `json:"model"`
	Messages    []channelMonitorMessage `json:"messages"`
	MaxTokens   int                     `json:"max_tokens"`
	Temperature float64                 `json:"temperature"`
	Stream      bool                    `json:"stream"`
}

type channelMonitorGeminiPart struct {
	Text string `json:"text"`
}

type channelMonitorGeminiContent struct {
	Role  string                     `json:"role"`
	Parts []channelMonitorGeminiPart `json:"parts"`
}

type channelMonitorGeminiGenerationConfig struct {
	MaxOutputTokens int     `json:"maxOutputTokens"`
	Temperature     float64 `json:"temperature"`
}

type channelMonitorGeminiRequest struct {
	Contents         []channelMonitorGeminiContent        `json:"contents"`
	GenerationConfig channelMonitorGeminiGenerationConfig `json:"generationConfig"`
}

func ProbeChannelMonitorModel(ctx context.Context, monitor *model.ChannelMonitor, modelName string, credential string, client *http.Client) ChannelMonitorProbeResult {
	if monitor == nil || strings.TrimSpace(modelName) == "" || credential == "" {
		return channelMonitorProbeFailure(model.ChannelMonitorErrorConfig, 0, 0, "invalid channel monitor probe configuration", credential)
	}
	request, err := newChannelMonitorProbeRequest(ctx, monitor, strings.TrimSpace(modelName), credential)
	if err != nil {
		return channelMonitorProbeFailure(model.ChannelMonitorErrorConfig, 0, 0, err.Error(), credential)
	}

	timeoutSeconds := monitor.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = channelMonitorDefaultTimeoutSeconds
	}
	requestContext, cancel := context.WithTimeout(request.Context(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	request = request.WithContext(requestContext)

	if client == nil {
		client = GetHttpClient()
		if client == nil {
			client = http.DefaultClient
		}
	}
	probeClient := *client
	probeClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	startedAt := channelMonitorProbeNow()
	response, err := probeClient.Do(request)
	if err != nil {
		latency := channelMonitorProbeLatency(startedAt, channelMonitorProbeNow())
		category := model.ChannelMonitorErrorNetwork
		message := "network request failed"
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			category = model.ChannelMonitorErrorTimeout
			message = "request timed out"
		} else if isChannelMonitorTLSError(err) {
			category = model.ChannelMonitorErrorTLS
			message = "TLS connection failed"
		}
		return channelMonitorProbeFailure(category, latency, 0, message, credential)
	}
	defer response.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(response.Body, channelMonitorProbeMaxResponseBytes+1))
	latency := channelMonitorProbeLatency(startedAt, channelMonitorProbeNow())
	if readErr != nil {
		return channelMonitorProbeFailure(model.ChannelMonitorErrorNetwork, latency, response.StatusCode, "read upstream response failed", credential)
	}
	if len(body) > channelMonitorProbeMaxResponseBytes {
		return channelMonitorProbeFailure(model.ChannelMonitorErrorInvalidResponse, latency, response.StatusCode, "upstream response exceeded the size limit", credential)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		category := model.ChannelMonitorErrorUpstream
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
			category = model.ChannelMonitorErrorAuth
		} else if response.StatusCode == http.StatusTooManyRequests {
			category = model.ChannelMonitorErrorRateLimit
		}
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = fmt.Sprintf("upstream returned HTTP %d", response.StatusCode)
		}
		return channelMonitorProbeFailure(category, latency, response.StatusCode, message, credential)
	}

	text, err := extractChannelMonitorProbeText(monitor.APIMode, body)
	if err != nil {
		return channelMonitorProbeFailure(model.ChannelMonitorErrorInvalidResponse, latency, response.StatusCode, "invalid upstream response", credential)
	}
	if strings.TrimSpace(text) != channelMonitorProbeChallenge {
		return channelMonitorProbeFailure(model.ChannelMonitorErrorInvalidResponse, latency, response.StatusCode, "upstream challenge response did not match", credential)
	}

	status := model.ChannelMonitorStatusOperational
	if latency > int64(monitor.DegradedThresholdMs) {
		status = model.ChannelMonitorStatusDegraded
	}
	return ChannelMonitorProbeResult{
		Status:     status,
		LatencyMs:  latency,
		HTTPStatus: response.StatusCode,
	}
}

func newChannelMonitorProbeRequest(ctx context.Context, monitor *model.ChannelMonitor, modelName string, credential string) (*http.Request, error) {
	var (
		requestURL string
		body       any
	)
	switch monitor.APIMode {
	case model.ChannelMonitorAPIModeChatCompletions:
		requestURL = joinChannelMonitorEndpoint(monitor.Endpoint, "/v1", "/chat/completions")
		body = channelMonitorChatRequest{
			Model:       modelName,
			Messages:    []channelMonitorMessage{{Role: "user", Content: channelMonitorProbePrompt}},
			MaxTokens:   channelMonitorProbeMaxOutputTokens,
			Temperature: 0,
			Stream:      false,
		}
	case model.ChannelMonitorAPIModeResponses:
		requestURL = joinChannelMonitorEndpoint(monitor.Endpoint, "/v1", "/responses")
		body = channelMonitorResponsesRequest{
			Model:           modelName,
			Input:           channelMonitorProbePrompt,
			MaxOutputTokens: channelMonitorProbeMaxOutputTokens,
			Stream:          false,
		}
	case model.ChannelMonitorAPIModeMessages:
		requestURL = joinChannelMonitorEndpoint(monitor.Endpoint, "/v1", "/messages")
		body = channelMonitorAnthropicRequest{
			Model:       modelName,
			Messages:    []channelMonitorMessage{{Role: "user", Content: channelMonitorProbePrompt}},
			MaxTokens:   channelMonitorProbeMaxOutputTokens,
			Temperature: 0,
			Stream:      false,
		}
	case model.ChannelMonitorAPIModeGenerateContent:
		requestURL = joinChannelMonitorEndpoint(
			monitor.Endpoint,
			"/v1beta",
			"/models/"+url.PathEscape(modelName)+":generateContent",
		)
		parsed, err := url.Parse(requestURL)
		if err != nil {
			return nil, errors.New("invalid Gemini channel monitor endpoint")
		}
		query := parsed.Query()
		query.Set("key", credential)
		parsed.RawQuery = query.Encode()
		requestURL = parsed.String()
		body = channelMonitorGeminiRequest{
			Contents:         []channelMonitorGeminiContent{{Role: "user", Parts: []channelMonitorGeminiPart{{Text: channelMonitorProbePrompt}}}},
			GenerationConfig: channelMonitorGeminiGenerationConfig{MaxOutputTokens: channelMonitorProbeMaxOutputTokens, Temperature: 0},
		}
	default:
		return nil, errors.New("unsupported channel monitor API mode")
	}

	payload, err := common.Marshal(body)
	if err != nil {
		return nil, errors.New("encode channel monitor probe request failed")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		return nil, errors.New("create channel monitor probe request failed")
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	switch monitor.APIMode {
	case model.ChannelMonitorAPIModeChatCompletions, model.ChannelMonitorAPIModeResponses:
		request.Header.Set("Authorization", "Bearer "+credential)
	case model.ChannelMonitorAPIModeMessages:
		request.Header.Set("x-api-key", credential)
		request.Header.Set("anthropic-version", "2023-06-01")
	}
	return request, nil
}

func joinChannelMonitorEndpoint(endpoint string, versionPath string, requestPath string) string {
	base := strings.TrimRight(endpoint, "/")
	if strings.HasSuffix(strings.ToLower(base), strings.ToLower(versionPath)) {
		return base + requestPath
	}
	return base + versionPath + requestPath
}

func extractChannelMonitorProbeText(apiMode string, body []byte) (string, error) {
	texts := make([]string, 0)
	switch apiMode {
	case model.ChannelMonitorAPIModeChatCompletions:
		var response struct {
			Choices []struct {
				Message struct {
					Content any `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := common.Unmarshal(body, &response); err != nil {
			return "", err
		}
		for _, choice := range response.Choices {
			texts = append(texts, channelMonitorTextValues(choice.Message.Content)...)
		}
	case model.ChannelMonitorAPIModeResponses:
		var response struct {
			OutputText string `json:"output_text"`
			Output     []struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"output"`
		}
		if err := common.Unmarshal(body, &response); err != nil {
			return "", err
		}
		texts = append(texts, response.OutputText)
		for _, output := range response.Output {
			for _, content := range output.Content {
				texts = append(texts, content.Text)
			}
		}
	case model.ChannelMonitorAPIModeMessages:
		var response struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := common.Unmarshal(body, &response); err != nil {
			return "", err
		}
		for _, content := range response.Content {
			texts = append(texts, content.Text)
		}
	case model.ChannelMonitorAPIModeGenerateContent:
		var response struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := common.Unmarshal(body, &response); err != nil {
			return "", err
		}
		for _, candidate := range response.Candidates {
			for _, part := range candidate.Content.Parts {
				texts = append(texts, part.Text)
			}
		}
	default:
		return "", errors.New("unsupported channel monitor API mode")
	}

	var builder strings.Builder
	for _, text := range texts {
		builder.WriteString(text)
	}
	if builder.Len() == 0 {
		return "", errors.New("channel monitor response did not contain text")
	}
	return builder.String(), nil
}

func channelMonitorTextValues(value any) []string {
	switch content := value.(type) {
	case string:
		return []string{content}
	case []any:
		texts := make([]string, 0, len(content))
		for _, item := range content {
			part, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text, ok := part["text"].(string); ok {
				texts = append(texts, text)
			}
		}
		return texts
	default:
		return nil
	}
}

func isChannelMonitorTLSError(err error) bool {
	var recordHeaderError tls.RecordHeaderError
	if errors.As(err, &recordHeaderError) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "tls") || strings.Contains(strings.ToLower(err.Error()), "x509")
}

func channelMonitorProbeFailure(category string, latencyMs int64, httpStatus int, message string, credential string) ChannelMonitorProbeResult {
	return ChannelMonitorProbeResult{
		Status:        model.ChannelMonitorStatusFailed,
		LatencyMs:     latencyMs,
		ErrorCategory: category,
		HTTPStatus:    httpStatus,
		ErrorMessage:  sanitizeChannelMonitorProbeError(message, credential),
	}
}

func sanitizeChannelMonitorProbeError(message string, credential string) string {
	message = strings.TrimSpace(message)
	if credential != "" {
		message = strings.ReplaceAll(message, credential, "[redacted]")
		message = strings.ReplaceAll(message, url.QueryEscape(credential), "[redacted]")
	}
	if len(message) <= channelMonitorProbeMaxErrorBytes {
		return message
	}
	end := channelMonitorProbeMaxErrorBytes
	for end > 0 && !utf8.ValidString(message[:end]) {
		end--
	}
	return message[:end]
}

func channelMonitorProbeLatency(startedAt time.Time, completedAt time.Time) int64 {
	latency := completedAt.Sub(startedAt).Milliseconds()
	if latency < 0 {
		return 0
	}
	return latency
}
