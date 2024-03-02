package server

import (
	"net/http"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager"
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
	group.POST("/webhooks/alertmanager", alertmanager.ReceiveWebhook)
}
