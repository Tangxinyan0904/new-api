package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCanvasUpdater answers like the Watchtower HTTP API for the container
// infinite-canvas, which runs version until an update replaces it with latest.
type fakeCanvasUpdater struct {
	mu        sync.Mutex
	version   string
	latest    string
	failCount int
	checks    int
	updates   int
}

func (f *fakeCanvasUpdater) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer updater-token" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.URL.Query().Get("container") != CanvasContainerName {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	switch r.URL.Path {
	case "/v1/check":
		f.checks++
		available := "false"
		if f.version != f.latest {
			available = "true"
		}
		_, _ = w.Write([]byte(`{"containers":[{"name":"infinite-canvas","update_available":` + available + `,"current_image_version":"` + f.version + `"}],"count":1}`))
	case "/v1/update":
		f.updates++
		if f.failCount > 0 {
			_, _ = w.Write([]byte(`{"summary":{"scanned":1,"updated":0,"failed":1}}`))
			return
		}
		updated := "0"
		if f.version != f.latest {
			f.version = f.latest
			updated = "1"
		}
		_, _ = w.Write([]byte(`{"summary":{"scanned":1,"updated":` + updated + `,"failed":0}}`))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func setupCanvasUpdater(t *testing.T, updater http.Handler) (releaseRequests *int) {
	t.Helper()
	require.NoError(t, model.DB.Where("type = ?", model.SystemTaskTypeCanvasUpdate).Delete(&model.SystemTask{}).Error)
	require.NoError(t, model.DB.Where("type = ?", model.SystemTaskTypeCanvasUpdate).Delete(&model.SystemTaskLock{}).Error)
	updaterServer := httptest.NewServer(updater)
	t.Cleanup(updaterServer.Close)
	requests := 0
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{"tag_name":"v0.20.0","published_at":"2026-10-10T08:47:20Z","html_url":"https://github.com/basketikun/infinite-canvas/releases/tag/v0.20.0","body":"+ [feat] new"}`))
	}))
	t.Cleanup(github.Close)
	savedURL := canvasReleaseURL
	canvasReleaseURL = github.URL
	t.Cleanup(func() { canvasReleaseURL = savedURL })
	canvasCache.Lock()
	canvasCache.checkExpires, canvasCache.releaseExpires, canvasCache.release = time.Time{}, time.Time{}, nil
	canvasCache.Unlock()
	t.Setenv("CANVAS_UPDATER_URL", updaterServer.URL+"/")
	t.Setenv("CANVAS_UPDATER_TOKEN", "updater-token")
	return &requests
}

// runCanvasUpdateTask claims the queued update and runs it the way the system
// task runner would.
func runCanvasUpdateTask(t *testing.T, task *model.SystemTask) *model.SystemTask {
	t.Helper()
	claimed, ok, err := model.ClaimSystemTask(task.ID, task.Type, "runner-test", common.GetTimestamp()+60)
	require.NoError(t, err)
	require.True(t, ok)
	canvasUpdateHandler{}.Run(context.Background(), claimed, "runner-test")
	finished, err := model.GetSystemTaskByTaskID(task.TaskID)
	require.NoError(t, err)
	return finished
}

func TestCanvasStatusNeedsTheUpdater(t *testing.T) {
	t.Setenv("CANVAS_UPDATER_URL", "")
	t.Setenv("CANVAS_UPDATER_TOKEN", "")

	status, err := GetCanvasStatus(context.Background(), true)
	require.NoError(t, err)
	assert.False(t, status.Configured)

	_, _, err = StartCanvasUpdate()
	assert.ErrorIs(t, err, ErrCanvasUpdaterNotConfigured)
}

func TestCanvasUpdateReplacesTheRunningVersion(t *testing.T) {
	updater := &fakeCanvasUpdater{version: "v0.15.1", latest: "v0.20.0"}
	releaseRequests := setupCanvasUpdater(t, updater)

	status, err := GetCanvasStatus(context.Background(), true)
	require.NoError(t, err)
	require.True(t, status.Configured)
	require.Nil(t, status.CheckError)
	assert.Equal(t, "v0.15.1", status.CurrentVersion)
	assert.True(t, status.UpdateAvailable)
	require.NotNil(t, status.Release)
	assert.Equal(t, "v0.20.0", status.Release.Version)
	assert.Nil(t, status.Task)

	task, created, err := StartCanvasUpdate()
	require.NoError(t, err)
	require.True(t, created)
	again, createdAgain, err := StartCanvasUpdate()
	require.NoError(t, err)
	assert.False(t, createdAgain, "a second click while one update is queued starts nothing")
	assert.Equal(t, task.TaskID, again.TaskID)

	checksBefore := updater.checks
	running, err := GetCanvasStatus(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, model.SystemTaskStatusPending, running.Task.Status)
	assert.Equal(t, checksBefore, updater.checks, "the updater is not asked while the container is being replaced")

	finished := runCanvasUpdateTask(t, task)
	assert.Equal(t, model.SystemTaskStatusSucceeded, finished.Status)
	var result CanvasUpdateResult
	require.NoError(t, common.UnmarshalJsonStr(finished.Result, &result))
	assert.Equal(t, CanvasUpdateResult{FromVersion: "v0.15.1", ToVersion: "v0.20.0", Updated: 1}, result)
	assert.Equal(t, 1, updater.updates)

	after, err := GetCanvasStatus(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, "v0.20.0", after.CurrentVersion, "the update drops the cached check")
	assert.False(t, after.UpdateAvailable)
	assert.Equal(t, model.SystemTaskStatusSucceeded, after.Task.Status)
	assert.Equal(t, 1, *releaseRequests, "GitHub is asked once per half hour")
}

func TestCanvasUpdateReportsUpdaterFailures(t *testing.T) {
	updater := &fakeCanvasUpdater{version: "v0.15.1", latest: "v0.20.0", failCount: 1}
	setupCanvasUpdater(t, updater)

	task, created, err := StartCanvasUpdate()
	require.NoError(t, err)
	require.True(t, created)
	finished := runCanvasUpdateTask(t, task)
	assert.Equal(t, model.SystemTaskStatusFailed, finished.Status)
	var result CanvasUpdateResult
	require.NoError(t, common.UnmarshalJsonStr(finished.Result, &result))
	require.NotNil(t, result.Error)
	assert.Equal(t, ErrCanvasUpdateFailed.Key, result.Error.Key)
	assert.Equal(t, "v0.15.1", result.FromVersion)

	t.Setenv("CANVAS_UPDATER_TOKEN", "wrong-token")
	status, err := GetCanvasStatus(context.Background(), true)
	require.NoError(t, err)
	require.NotNil(t, status.CheckError)
	assert.Equal(t, ErrCanvasUpdaterToken.Key, status.CheckError.Key)
}
