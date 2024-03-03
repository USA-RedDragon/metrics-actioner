package server

import (
	"log/slog"
	"net/http"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager"
	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager/models"
	"github.com/gin-gonic/gin"
)

func applyRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	apiV1 := r.Group("/api/v1")
	v1(apiV1)
}

func v1(group *gin.RouterGroup) {
	group.POST("/webhooks/alertmanager", v1ReceiveWebhook)
}

func v1ReceiveWebhook(c *gin.Context) {
	var json models.Webhook
	receiver, ok := c.MustGet("AlertManagerReceiver").(*alertmanager.Receiver)
	if !ok {
		slog.Error("Failed to get AlertManager receiver from context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if err := c.ShouldBindJSON(&json); err != nil {
		slog.Error("Failed to bind AlertManager webhook JSON", "error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := receiver.ReceiveWebhook(&json); err != nil {
		slog.Error("Failed to process AlertManager webhook", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
