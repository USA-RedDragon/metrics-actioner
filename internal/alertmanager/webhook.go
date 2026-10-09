package alertmanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager/models"
	"github.com/USA-RedDragon/metrics-actioner/internal/config"
)

type Receiver struct {
	config            *[]config.Action
	registeredActions map[string]ActionIface
}

func NewReceiver(config *[]config.Action) *Receiver {
	return &Receiver{
		config:            config,
		registeredActions: findActions(),
	}
}

func matchLabels(webhookLabels models.Labels, ruleLabels config.Labels) bool {
	for key, value := range ruleLabels {
		if webhookLabels[key] != value {
			return false
		}
	}
	return true
}

func (r *Receiver) ReceiveWebhook(ctx context.Context, webhook *models.Webhook) error {
	// Print the json to the console
	slog.Info("Received AlertManager webhook")

	var errs []error
	// For each defined action in the config
	for _, alertRule := range *r.config {
		if len(alertRule.MatchCommonLabels) > 0 {
			// Check if the common labels match
			if !matchLabels(webhook.CommonLabels, alertRule.MatchCommonLabels) {
				// If the common labels don't match, skip this action
				continue
			}
		}
		if len(alertRule.MatchGroupLabels) > 0 {
			// Check if the group labels match
			if !matchLabels(webhook.GroupLabels, alertRule.MatchGroupLabels) {
				// If the group labels don't match, skip this action
				continue
			}
		}
		// If the alert is not firing, skip this action
		if webhook.Status != string(models.AlertStatusFiring) {
			continue
		}
		slog.Info("Matched alert rule with webhook", "rule", alertRule, "webhook", webhook)
		// We match so far, so we execute the action
		action, err := r.FindAction(alertRule.Action)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		err = action.Execute(ctx, webhook, alertRule.Options)
		if err != nil {
			errs = append(errs, fmt.Errorf("action %s: %w", alertRule.Action, err))
		}
	}
	for _, alert := range webhook.Alerts {
		slog.Info("Received AlertManager alert", "alert", alert)
	}
	return errors.Join(errs...)
}
