package alertmanager

import (
	"log/slog"
	"time"

	"github.com/USA-RedDragon/metrics-actioner/internal/config"
)

// https://prometheus.io/docs/alerting/latest/configuration/#webhook_config

type AlertStatus string

const (
	AlertStatusFiring   AlertStatus = "firing"
	AlertStatusResolved AlertStatus = "resolved"
)

type Labels map[string]string
type Annotations map[string]string

type Alert struct {
	Status       AlertStatus `json:"status"`
	Labels       Labels      `json:"labels"`
	Annotations  Annotations `json:"annotations"`
	StartsAt     time.Time   `json:"startsAt"`
	EndsAt       time.Time   `json:"endsAt"`
	GeneratorURL string      `json:"generatorURL"`
	Fingerprint  string      `json:"fingerprint"`
}

// The Alertmanager will send HTTP POST requests in the following JSON format to the configured endpoint:
type Webhook struct {
	Version           string      `json:"version"`
	GroupKey          string      `json:"groupKey"`
	TruncatedAlerts   int         `json:"truncatedAlerts"`
	Status            string      `json:"status"`
	Receiver          string      `json:"receiver"`
	GroupLabels       Labels      `json:"groupLabels"`
	CommonLabels      Labels      `json:"commonLabels"`
	CommonAnnotations Annotations `json:"commonAnnotations"`
	ExternalURL       string      `json:"externalURL"`
	Alerts            []Alert     `json:"alerts"`
}

type Receiver struct {
	config *config.Actions
}

func NewReceiver(config *config.Actions) *Receiver {
	return &Receiver{config: config}
}

func (r *Receiver) ReceiveWebhook(webhook Webhook) error {
	// Print the json to the console
	slog.Info("Received AlertManager webhook", "webhook", webhook)
	return nil
}
