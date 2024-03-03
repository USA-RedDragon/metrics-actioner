package alertmanager

import (
	"fmt"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager/actions"
	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager/models"
)

type ActionIface interface {
	Execute(webhook *models.Webhook, options map[string]string) error
}

func (r *Receiver) FindAction(action string) (ActionIface, error) {
	if action, ok := r.registeredActions[action]; ok {
		return action, nil
	}
	return nil, fmt.Errorf("action not found: %s", action)
}

func findActions() map[string]ActionIface {
	foundActions := make(map[string]ActionIface)
	foundActions["rollout-restart-deployment"] = &actions.RolloutRestartDeployment{}
	return foundActions
}
