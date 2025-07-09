package controller

import (
	"ginK8s/pkg/services"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

type PodLogEventController struct {
	LogService *services.PodLogEventService
}

func NewPodLogEventController(logService *services.PodLogEventService) *PodLogEventController {
	return &PodLogEventController{
		LogService: logService,
	}
}

func (p *PodLogEventController) GetPodLog(c *gin.Context) {
	podName := c.DefaultQuery("podName", "")
	ns := c.DefaultQuery("namespace", "default")

	k8sReq := p.LogService.GetPodLog(podName, ns)
	rc, err := k8sReq.Stream(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rc.Close()

	logData, err := io.ReadAll(rc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": string(logData)})
}

func (p *PodLogEventController) GetPodEventList(c *gin.Context) {
	podName := c.DefaultQuery("podName", "")
	ns := c.DefaultQuery("namespace", "default")

	events, err := p.LogService.GetEventList(podName, ns)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": events})
}