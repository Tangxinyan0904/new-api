package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// The infinite canvas runs as its own container next to new-api. new-api never
// talks to Docker: an updater sidecar (a Watchtower fork with its HTTP API on,
// limited to this container) pulls and recreates it when asked.
const (
	CanvasContainerName = "infinite-canvas"

	canvasCheckTimeout   = 60 * time.Second
	canvasUpdateTimeout  = 10 * time.Minute
	canvasReleaseTimeout = 15 * time.Second
	canvasCheckCacheTTL  = time.Minute
	// GitHub allows 60 unauthenticated requests per hour per IP.
	canvasReleaseCacheTTL      = 30 * time.Minute
	canvasReleaseErrorCacheTTL = 5 * time.Minute
	canvasMaxBodyBytes         = 1 << 20
)

// canvasReleaseURL is a variable so tests can point it at a fake GitHub.
var canvasReleaseURL = "https://api.github.com/repos/basketikun/infinite-canvas/releases/latest"

var (
	ErrCanvasUpdaterNotConfigured = common.NewMessage("The canvas updater is not configured")
	ErrCanvasUpdaterToken         = common.NewMessage("The canvas updater rejected the token")
	ErrCanvasUpdaterBusy          = common.NewMessage("The canvas updater is busy. Try again later.")
	ErrCanvasUpdaterAnswer        = common.NewMessage("The canvas updater answer could not be read")
	ErrCanvasContainerNotManaged  = common.NewMessage("The canvas updater does not manage the container {{name}}", map[string]any{"name": CanvasContainerName})
	ErrCanvasUpdateFailed         = common.NewMessage("The canvas updater could not update the container. Check its logs.")
)

// The updater is on the internal Docker network, so its client skips any
// outbound proxy.
var canvasUpdaterClient = func() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{Transport: transport}
}()

// canvasUpdaterConfig reads the updater address and token from
// CANVAS_UPDATER_URL and CANVAS_UPDATER_TOKEN.
func canvasUpdaterConfig() (baseURL string, token string, ok bool) {
	baseURL = strings.TrimRight(strings.TrimSpace(common.GetEnvOrDefaultString("CANVAS_UPDATER_URL", "")), "/")
	token = strings.TrimSpace(common.GetEnvOrDefaultString("CANVAS_UPDATER_TOKEN", ""))
	return baseURL, token, baseURL != "" && token != ""
}

// callCanvasUpdater posts to the updater's HTTP API and decodes its JSON answer
// into out.
func callCanvasUpdater(ctx context.Context, path string, out any) error {
	baseURL, token, ok := canvasUpdaterConfig()
	if !ok {
		return ErrCanvasUpdaterNotConfigured
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, nil)
	if err != nil {
		return common.NewMessage("Canvas updater unreachable: {{error}}", map[string]any{"error": err.Error()})
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := canvasUpdaterClient.Do(request)
	if err != nil {
		return common.NewMessage("Canvas updater unreachable: {{error}}", map[string]any{"error": err.Error()})
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, canvasMaxBodyBytes))
	if err != nil {
		return common.NewMessage("Canvas updater unreachable: {{error}}", map[string]any{"error": err.Error()})
	}
	switch {
	case response.StatusCode == http.StatusUnauthorized:
		return ErrCanvasUpdaterToken
	case response.StatusCode == http.StatusTooManyRequests:
		return ErrCanvasUpdaterBusy
	case response.StatusCode < 200 || response.StatusCode > 299:
		var failure struct {
			Error string `json:"error"`
		}
		detail := strings.TrimSpace(string(body))
		if common.Unmarshal(body, &failure) == nil && failure.Error != "" {
			detail = failure.Error
		}
		if detail == "" {
			detail = http.StatusText(response.StatusCode)
		}
		return common.NewMessage("The canvas updater answered {{status}}: {{error}}", map[string]any{"status": response.StatusCode, "error": truncateUpstreamMessage(detail)})
	}
	if err := common.Unmarshal(body, out); err != nil {
		return ErrCanvasUpdaterAnswer
	}
	return nil
}

type canvasCheck struct {
	CurrentVersion  string
	UpdateAvailable bool
}

// checkCanvas asks the updater which version the canvas container runs and
// whether the registry has a newer image. Nothing is pulled.
func checkCanvas(ctx context.Context) (canvasCheck, error) {
	ctx, cancel := context.WithTimeout(ctx, canvasCheckTimeout)
	defer cancel()
	var response struct {
		Containers []struct {
			Name                string `json:"name"`
			UpdateAvailable     bool   `json:"update_available"`
			CurrentImageVersion string `json:"current_image_version"`
			Error               string `json:"error"`
		} `json:"containers"`
	}
	path := "/v1/check?container=" + url.QueryEscape(CanvasContainerName) + "&timeout=" + canvasCheckTimeout.String()
	if err := callCanvasUpdater(ctx, path, &response); err != nil {
		return canvasCheck{}, err
	}
	for _, container := range response.Containers {
		if strings.TrimPrefix(container.Name, "/") != CanvasContainerName {
			continue
		}
		if container.Error != "" {
			return canvasCheck{}, common.NewMessage("The canvas update check failed: {{error}}", map[string]any{"error": truncateUpstreamMessage(container.Error)})
		}
		return canvasCheck{CurrentVersion: container.CurrentImageVersion, UpdateAvailable: container.UpdateAvailable}, nil
	}
	return canvasCheck{}, ErrCanvasContainerNotManaged
}

// CanvasRelease is the newest release of the canvas on GitHub.
type CanvasRelease struct {
	Version     string `json:"version"`
	PublishedAt string `json:"published_at"`
	URL         string `json:"url"`
	Notes       string `json:"notes"`
}

func fetchLatestCanvasRelease(ctx context.Context) (*CanvasRelease, error) {
	ctx, cancel := context.WithTimeout(ctx, canvasReleaseTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, canvasReleaseURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, common.NewMessage("GitHub could not be reached: {{error}}", map[string]any{"error": err.Error()})
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusForbidden || response.StatusCode == http.StatusTooManyRequests {
		return nil, common.NewMessage("GitHub rate limit reached. Try again later.")
	}
	if response.StatusCode != http.StatusOK {
		return nil, common.NewMessage("GitHub answered {{status}}", map[string]any{"status": response.StatusCode})
	}
	var release struct {
		TagName     string `json:"tag_name"`
		PublishedAt string `json:"published_at"`
		HTMLURL     string `json:"html_url"`
		Body        string `json:"body"`
	}
	if err := common.DecodeJson(io.LimitReader(response.Body, canvasMaxBodyBytes), &release); err != nil || release.TagName == "" {
		return nil, common.NewMessage("Unexpected release payload")
	}
	return &CanvasRelease{Version: release.TagName, PublishedAt: release.PublishedAt, URL: release.HTMLURL, Notes: release.Body}, nil
}

var canvasCache struct {
	sync.Mutex
	check          canvasCheck
	checkErr       error
	checkedAt      int64
	checkExpires   time.Time
	release        *CanvasRelease
	releaseErr     error
	releaseExpires time.Time
}

// latestCanvasRelease keeps GitHub's answer for half an hour, and a failure for
// five minutes.
func latestCanvasRelease(ctx context.Context) (*CanvasRelease, error) {
	canvasCache.Lock()
	if time.Now().Before(canvasCache.releaseExpires) {
		release, err := canvasCache.release, canvasCache.releaseErr
		canvasCache.Unlock()
		return release, err
	}
	canvasCache.Unlock()
	release, err := fetchLatestCanvasRelease(ctx)
	canvasCache.Lock()
	defer canvasCache.Unlock()
	ttl := canvasReleaseCacheTTL
	if err != nil {
		ttl = canvasReleaseErrorCacheTTL
		// Keep showing the last release that was read.
		release = canvasCache.release
	}
	canvasCache.release, canvasCache.releaseErr, canvasCache.releaseExpires = release, err, time.Now().Add(ttl)
	return release, err
}

func cachedCanvasCheck(ctx context.Context, refresh bool) (canvasCheck, int64, error) {
	canvasCache.Lock()
	if !refresh && time.Now().Before(canvasCache.checkExpires) {
		check, checkedAt, err := canvasCache.check, canvasCache.checkedAt, canvasCache.checkErr
		canvasCache.Unlock()
		return check, checkedAt, err
	}
	canvasCache.Unlock()
	check, err := checkCanvas(ctx)
	checkedAt := common.GetTimestamp()
	canvasCache.Lock()
	canvasCache.check, canvasCache.checkErr, canvasCache.checkedAt, canvasCache.checkExpires = check, err, checkedAt, time.Now().Add(canvasCheckCacheTTL)
	canvasCache.Unlock()
	return check, checkedAt, err
}

func forgetCanvasCheck() {
	canvasCache.Lock()
	canvasCache.checkExpires = time.Time{}
	canvasCache.Unlock()
}

// CanvasUpdateResult is what a canvas update task records when it ends.
type CanvasUpdateResult struct {
	FromVersion string          `json:"from_version"`
	ToVersion   string          `json:"to_version"`
	Updated     int             `json:"updated"`
	Error       *common.Message `json:"error,omitempty"`
}

// CanvasStatus is what the console shows about the canvas.
type CanvasStatus struct {
	Configured      bool                      `json:"configured"`
	Container       string                    `json:"container"`
	CurrentVersion  string                    `json:"current_version"`
	UpdateAvailable bool                      `json:"update_available"`
	CheckedAt       int64                     `json:"checked_at"`
	CheckError      *common.Message           `json:"check_error"`
	Release         *CanvasRelease            `json:"release"`
	ReleaseError    *common.Message           `json:"release_error"`
	Task            *model.SystemTaskResponse `json:"task"`
}

// canvasMessage turns err into a message the console can translate.
func canvasMessage(err error) *common.Message {
	if err == nil {
		return nil
	}
	var message *common.Message
	if errors.As(err, &message) {
		return message
	}
	return common.NewMessage("Canvas updater unreachable: {{error}}", map[string]any{"error": err.Error()})
}

// GetCanvasStatus reports the running and newest canvas versions and the
// latest update task. While an update runs the updater is not asked, since the
// container is being replaced. refresh skips the one-minute cache of the check.
func GetCanvasStatus(ctx context.Context, refresh bool) (CanvasStatus, error) {
	status := CanvasStatus{Container: CanvasContainerName}
	if _, _, ok := canvasUpdaterConfig(); !ok {
		return status, nil
	}
	status.Configured = true
	task, err := model.GetLatestSystemTask(model.SystemTaskTypeCanvasUpdate)
	if err != nil {
		return status, err
	}
	if task != nil {
		response := task.ToResponse()
		status.Task = &response
	}
	release, releaseErr := latestCanvasRelease(ctx)
	status.Release, status.ReleaseError = release, canvasMessage(releaseErr)
	if task != nil && (task.Status == model.SystemTaskStatusPending || task.Status == model.SystemTaskStatusRunning) {
		return status, nil
	}
	check, checkedAt, checkErr := cachedCanvasCheck(ctx, refresh)
	status.CurrentVersion, status.UpdateAvailable, status.CheckedAt = check.CurrentVersion, check.UpdateAvailable, checkedAt
	status.CheckError = canvasMessage(checkErr)
	return status, nil
}

// StartCanvasUpdate queues an update of the canvas container. created is false
// when an update is already queued or running; that task is returned instead.
func StartCanvasUpdate() (*model.SystemTask, bool, error) {
	if _, _, ok := canvasUpdaterConfig(); !ok {
		return nil, false, ErrCanvasUpdaterNotConfigured
	}
	return EnqueueSystemTask(model.SystemTaskTypeCanvasUpdate, nil)
}

type canvasUpdateHandler struct{}

func (canvasUpdateHandler) Type() string { return model.SystemTaskTypeCanvasUpdate }

// Run asks the updater to pull the newest canvas image and recreate the
// container with its current settings, then records the versions before and
// after.
func (canvasUpdateHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	ctx, cancel := context.WithTimeout(ctx, canvasUpdateTimeout+time.Minute)
	defer cancel()
	defer forgetCanvasCheck()
	result := CanvasUpdateResult{}
	if before, err := checkCanvas(ctx); err == nil {
		result.FromVersion = before.CurrentVersion
	}
	err := runCanvasUpdate(ctx, &result)
	status := model.SystemTaskStatusSucceeded
	errorText := ""
	if err != nil {
		status = model.SystemTaskStatusFailed
		result.Error = canvasMessage(err)
		errorText = err.Error()
	}
	if finishErr := model.FinishSystemTask(task.TaskID, runnerID, status, result, errorText); finishErr != nil {
		common.SysLog(common.LogText("system task %s failed to persist result: %v", task.TaskID, finishErr))
	}
}

func runCanvasUpdate(ctx context.Context, result *CanvasUpdateResult) error {
	var response struct {
		Summary struct {
			Scanned int `json:"scanned"`
			Updated int `json:"updated"`
			Failed  int `json:"failed"`
		} `json:"summary"`
	}
	path := "/v1/update?container=" + url.QueryEscape(CanvasContainerName) + "&timeout=" + canvasUpdateTimeout.String()
	if err := callCanvasUpdater(ctx, path, &response); err != nil {
		return err
	}
	result.Updated = response.Summary.Updated
	if response.Summary.Scanned == 0 {
		return ErrCanvasContainerNotManaged
	}
	if response.Summary.Failed > 0 {
		return ErrCanvasUpdateFailed
	}
	after, err := checkCanvas(ctx)
	if err != nil {
		return err
	}
	result.ToVersion = after.CurrentVersion
	return nil
}

func init() {
	RegisterSystemTaskHandler(canvasUpdateHandler{})
}
