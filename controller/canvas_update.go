package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// GetCanvasStatus reports the running and newest infinite canvas versions and
// the latest update. refresh=true asks the updater again instead of using the
// last minute's answer.
func GetCanvasStatus(c *gin.Context) {
	status, err := service.GetCanvasStatus(c.Request.Context(), c.Query("refresh") == "true")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}

// StartCanvasUpdate queues an update of the infinite canvas container; the
// console follows it through GetCanvasStatus.
func StartCanvasUpdate(c *gin.Context) {
	task, created, err := service.StartCanvasUpdate()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !created {
		common.ApiErrorStatus(c, http.StatusConflict, common.NewMessage("A canvas update is already running"), gin.H{"data": task.ToResponse()})
		return
	}
	recordManageAudit(c, "canvas.update", map[string]any{"task_id": task.TaskID})
	common.ApiSuccess(c, task.ToResponse())
}
