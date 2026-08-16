package service

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProbeChannelMonitorModelBuildsSupportedProtocolRequests(t *testing.T) {
	const credential = "monitor-probe-secret"
	tests := []struct {
		name          string
		provider      string
		apiMode       string
		expectedPath  string
		responseBody  string
		assertRequest func(*testing.T, *http.Request, map[string]any)
	}{
		{
			name:         "openai chat completions",
			provider:     model.ChannelMonitorProviderOpenAI,
			apiMode:      model.ChannelMonitorAPIModeChatCompletions,
			expectedPath: "/v1/chat/completions",
			responseBody: `{"choices":[{"message":{"content":"NEWAPI_MONITOR_OK"}}]}`,
			assertRequest: func(t *testing.T, request *http.Request, body map[string]any) {
				assert.Equal(t, "Bearer "+credential, request.Header.Get("Authorization"))
				assert.Equal(t, "gpt-test", body["model"])
				assert.Equal(t, false, body["stream"])
				assert.Equal(t, float64(16), body["max_tokens"])
			},
		},
		{
			name:         "openai responses",
			provider:     model.ChannelMonitorProviderOpenAI,
			apiMode:      model.ChannelMonitorAPIModeResponses,
			expectedPath: "/v1/responses",
			responseBody: `{"output":[{"content":[{"type":"output_text","text":"NEWAPI_MONITOR_OK"}]}]}`,
			assertRequest: func(t *testing.T, request *http.Request, body map[string]any) {
				assert.Equal(t, "Bearer "+credential, request.Header.Get("Authorization"))
				assert.Equal(t, "gpt-test", body["model"])
				assert.Equal(t, false, body["stream"])
				assert.Equal(t, float64(16), body["max_output_tokens"])
			},
		},
		{
			name:         "anthropic messages",
			provider:     model.ChannelMonitorProviderAnthropic,
			apiMode:      model.ChannelMonitorAPIModeMessages,
			expectedPath: "/v1/messages",
			responseBody: `{"content":[{"type":"text","text":"NEWAPI_MONITOR_OK"}]}`,
			assertRequest: func(t *testing.T, request *http.Request, body map[string]any) {
				assert.Equal(t, credential, request.Header.Get("x-api-key"))
				assert.Equal(t, "2023-06-01", request.Header.Get("anthropic-version"))
				assert.Equal(t, "gpt-test", body["model"])
				assert.Equal(t, false, body["stream"])
				assert.Equal(t, float64(16), body["max_tokens"])
			},
		},
		{
			name:         "gemini generate content",
			provider:     model.ChannelMonitorProviderGemini,
			apiMode:      model.ChannelMonitorAPIModeGenerateContent,
			expectedPath: "/v1beta/models/gpt-test:generateContent",
			responseBody: `{"candidates":[{"content":{"parts":[{"text":"NEWAPI_MONITOR_OK"}]}}]}`,
			assertRequest: func(t *testing.T, request *http.Request, body map[string]any) {
				assert.Equal(t, credential, request.URL.Query().Get("key"))
				configuration, ok := body["generationConfig"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, float64(16), configuration["maxOutputTokens"])
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				assert.Equal(t, http.MethodPost, request.Method)
				assert.Equal(t, test.expectedPath, request.URL.Path)
				assert.Equal(t, "application/json", request.Header.Get("Content-Type"))
				var body map[string]any
				require.NoError(t, common.DecodeJson(request.Body, &body))
				test.assertRequest(t, request, body)
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write([]byte(test.responseBody))
			}))
			defer server.Close()

			monitor := channelMonitorProbeFixture(server.URL, test.provider, test.apiMode)
			result := ProbeChannelMonitorModel(context.Background(), monitor, "gpt-test", credential, server.Client())

			assert.Equal(t, model.ChannelMonitorStatusOperational, result.Status)
			assert.Equal(t, http.StatusOK, result.HTTPStatus)
			assert.Empty(t, result.ErrorCategory)
			assert.Empty(t, result.ErrorMessage)
			assert.NotContains(t, result.ErrorMessage, credential)
		})
	}
}

func TestProbeChannelMonitorModelClassifiesHTTPFailures(t *testing.T) {
	const credential = "monitor-sensitive-key"
	tests := []struct {
		statusCode int
		category   string
	}{
		{statusCode: http.StatusUnauthorized, category: model.ChannelMonitorErrorAuth},
		{statusCode: http.StatusForbidden, category: model.ChannelMonitorErrorAuth},
		{statusCode: http.StatusTooManyRequests, category: model.ChannelMonitorErrorRateLimit},
		{statusCode: http.StatusInternalServerError, category: model.ChannelMonitorErrorUpstream},
	}

	for _, test := range tests {
		t.Run(http.StatusText(test.statusCode), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.statusCode)
				_, _ = writer.Write([]byte("upstream rejected " + credential))
			}))
			defer server.Close()

			result := ProbeChannelMonitorModel(
				context.Background(),
				channelMonitorProbeFixture(server.URL, model.ChannelMonitorProviderOpenAI, model.ChannelMonitorAPIModeChatCompletions),
				"gpt-test",
				credential,
				server.Client(),
			)

			assert.Equal(t, model.ChannelMonitorStatusFailed, result.Status)
			assert.Equal(t, test.category, result.ErrorCategory)
			assert.Equal(t, test.statusCode, result.HTTPStatus)
			assert.NotContains(t, result.ErrorMessage, credential)
			assert.LessOrEqual(t, len(result.ErrorMessage), channelMonitorProbeMaxErrorBytes)
		})
	}
}

func TestProbeChannelMonitorModelRedactsURLEncodedCredentialFromErrors(t *testing.T) {
	const credential = "gemini/key+secret="
	const encodedCredential = "gemini%2Fkey%2Bsecret%3D"
	result := ProbeChannelMonitorModel(
		context.Background(),
		channelMonitorProbeFixture("https://example.com", model.ChannelMonitorProviderGemini, model.ChannelMonitorAPIModeGenerateContent),
		"gemini-test",
		credential,
		channelMonitorStaticResponseClient(http.StatusInternalServerError, "upstream rejected key="+encodedCredential),
	)

	assert.Equal(t, model.ChannelMonitorStatusFailed, result.Status)
	assert.Equal(t, model.ChannelMonitorErrorUpstream, result.ErrorCategory)
	assert.NotContains(t, result.ErrorMessage, credential)
	assert.NotContains(t, result.ErrorMessage, encodedCredential)
}

func TestProbeChannelMonitorModelClassifiesTransportFailures(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		category string
	}{
		{name: "timeout", err: context.DeadlineExceeded, category: model.ChannelMonitorErrorTimeout},
		{name: "tls", err: tls.RecordHeaderError{Msg: "bad TLS record"}, category: model.ChannelMonitorErrorTLS},
		{name: "network", err: errors.New("connection refused"), category: model.ChannelMonitorErrorNetwork},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: channelMonitorRoundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, test.err
			})}
			result := ProbeChannelMonitorModel(
				context.Background(),
				channelMonitorProbeFixture("https://example.com", model.ChannelMonitorProviderOpenAI, model.ChannelMonitorAPIModeResponses),
				"gpt-test",
				"transport-secret",
				client,
			)
			assert.Equal(t, model.ChannelMonitorStatusFailed, result.Status)
			assert.Equal(t, test.category, result.ErrorCategory)
			assert.Zero(t, result.HTTPStatus)
			assert.NotContains(t, result.ErrorMessage, "transport-secret")
		})
	}
}

func TestProbeChannelMonitorModelRejectsInvalidResponses(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed", body: "{"},
		{name: "empty", body: `{"choices":[]}`},
		{name: "challenge mismatch", body: `{"choices":[{"message":{"content":"something else"}}]}`},
		{name: "oversized", body: strings.Repeat("x", channelMonitorProbeMaxResponseBytes+1)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := channelMonitorStaticResponseClient(http.StatusOK, test.body)
			result := ProbeChannelMonitorModel(
				context.Background(),
				channelMonitorProbeFixture("https://example.com", model.ChannelMonitorProviderOpenAI, model.ChannelMonitorAPIModeChatCompletions),
				"gpt-test",
				"invalid-response-secret",
				client,
			)
			assert.Equal(t, model.ChannelMonitorStatusFailed, result.Status)
			assert.Equal(t, model.ChannelMonitorErrorInvalidResponse, result.ErrorCategory)
			assert.Equal(t, http.StatusOK, result.HTTPStatus)
			assert.NotContains(t, result.ErrorMessage, "invalid-response-secret")
		})
	}
}

func TestProbeChannelMonitorModelMarksSlowValidResponseDegraded(t *testing.T) {
	previousNow := channelMonitorProbeNow
	start := time.Unix(100, 0)
	times := []time.Time{start, start.Add(7 * time.Second)}
	channelMonitorProbeNow = func() time.Time {
		value := times[0]
		times = times[1:]
		return value
	}
	t.Cleanup(func() { channelMonitorProbeNow = previousNow })

	monitor := channelMonitorProbeFixture("https://example.com", model.ChannelMonitorProviderOpenAI, model.ChannelMonitorAPIModeResponses)
	monitor.DegradedThresholdMs = 6000
	result := ProbeChannelMonitorModel(
		context.Background(),
		monitor,
		"gpt-test",
		"degraded-secret",
		channelMonitorStaticResponseClient(http.StatusOK, `{"output_text":"NEWAPI_MONITOR_OK"}`),
	)

	assert.Equal(t, model.ChannelMonitorStatusDegraded, result.Status)
	assert.Equal(t, int64(7000), result.LatencyMs)
}

func channelMonitorProbeFixture(endpoint string, provider string, apiMode string) *model.ChannelMonitor {
	return &model.ChannelMonitor{
		Provider:            provider,
		APIMode:             apiMode,
		Endpoint:            endpoint,
		DegradedThresholdMs: 6000,
		TimeoutSeconds:      30,
	}
}

type channelMonitorRoundTripFunc func(*http.Request) (*http.Response, error)

func (function channelMonitorRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func channelMonitorStaticResponseClient(statusCode int, body string) *http.Client {
	return &http.Client{Transport: channelMonitorRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: statusCode,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})}
}
