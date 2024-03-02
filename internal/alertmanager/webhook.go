package alertmanager

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type alertStatus string

const (
	AlertStatusFiring   alertStatus = "firing"
	AlertStatusResolved alertStatus = "resolved"
)

type labels map[string]string
type annotations map[string]string

type alert struct {
	Status       alertStatus `json:"status"`
	Labels       labels      `json:"labels"`
	Annotations  annotations `json:"annotations"`
	StartsAt     time.Time   `json:"startsAt"`
	EndsAt       time.Time   `json:"endsAt"`
	GeneratorURL string      `json:"generatorURL"`
	Fingerprint  string      `json:"fingerprint"`
}

// https://prometheus.io/docs/alerting/latest/configuration/#webhook_config
// The Alertmanager will send HTTP POST requests in the following JSON format to the configured endpoint:
//
//	{
//		"version": "4",
//		"groupKey": <string>,              // key identifying the group of alerts (e.g. to deduplicate)
//		"truncatedAlerts": <int>,          // how many alerts have been truncated due to "max_alerts"
//		"status": "<resolved|firing>",
//		"receiver": <string>,
//		"groupLabels": <object>,
//		"commonLabels": <object>,
//		"commonAnnotations": <object>,
//		"externalURL": <string>,           // backlink to the Alertmanager.
//		"alerts": [
//		  {
//			"status": "<resolved|firing>",
//			"labels": <object>,
//			"annotations": <object>,
//			"startsAt": "<rfc3339>",
//			"endsAt": "<rfc3339>",
//			"generatorURL": <string>,      // identifies the entity that caused the alert
//			"fingerprint": <string>        // fingerprint to identify the alert
//		  },
//		  ...
//		]
//	}
type webhook struct {
	Version           string      `json:"version"`
	GroupKey          string      `json:"groupKey"`
	TruncatedAlerts   int         `json:"truncatedAlerts"`
	Status            string      `json:"status"`
	Receiver          string      `json:"receiver"`
	GroupLabels       labels      `json:"groupLabels"`
	CommonLabels      labels      `json:"commonLabels"`
	CommonAnnotations annotations `json:"commonAnnotations"`
	ExternalURL       string      `json:"externalURL"`
	Alerts            []alert     `json:"alerts"`
}

func ReceiveWebhook(c *gin.Context) {
	var json webhook
	if err := c.ShouldBindJSON(&json); err != nil {
		slog.Error("Failed to bind AlertManager webhook JSON", "error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Print the json to the console
	slog.Info("Received AlertManager webhook", "json", json)
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
