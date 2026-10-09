package alertmanager

import (
	"context"
	"errors"
	"testing"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager/models"
	"github.com/USA-RedDragon/metrics-actioner/internal/config"
)

const (
	alertName = "alertname"
	alertDown = "Down"
	nsLabel   = "namespace"
	firing    = string(models.AlertStatusFiring)
)

type fakeAction struct {
	err   error
	calls []map[string]string
}

func (f *fakeAction) Execute(_ context.Context, _ *models.Webhook, options map[string]string) error {
	f.calls = append(f.calls, options)
	return f.err
}

func TestFailingActionDoesNotSkipLaterRules(t *testing.T) {
	t.Parallel()
	failing := &fakeAction{err: errors.New("boom")}
	working := &fakeAction{}
	rules := []config.Action{
		{MatchCommonLabels: config.Labels{alertName: alertDown}, Action: "failing"},
		{MatchCommonLabels: config.Labels{alertName: alertDown}, Action: "working", Options: config.Options{"n": "1"}},
	}
	r := &Receiver{config: &rules, registeredActions: map[string]ActionIface{"failing": failing, "working": working}}

	err := r.ReceiveWebhook(context.Background(), &models.Webhook{
		Status:       firing,
		CommonLabels: models.Labels{alertName: alertDown},
	})
	if err == nil {
		t.Error("expected the failing action's error")
	}
	if len(failing.calls) != 1 {
		t.Errorf("failing action ran %d times, want 1", len(failing.calls))
	}
	if len(working.calls) != 1 {
		t.Errorf("second matching rule ran %d times, want 1", len(working.calls))
	}
}

func TestMatching(t *testing.T) {
	t.Parallel()
	action := &fakeAction{}
	rules := []config.Action{
		{MatchCommonLabels: config.Labels{alertName: alertDown}, MatchGroupLabels: config.Labels{nsLabel: "a"}, Action: "x"},
	}
	r := &Receiver{config: &rules, registeredActions: map[string]ActionIface{"x": action}}
	for _, tc := range []struct {
		name string
		hook models.Webhook
		want int
	}{
		{"match", models.Webhook{Status: firing, CommonLabels: models.Labels{alertName: alertDown, "extra": "y"}, GroupLabels: models.Labels{nsLabel: "a"}}, 1},
		{"resolved", models.Webhook{Status: "resolved", CommonLabels: models.Labels{alertName: alertDown}, GroupLabels: models.Labels{nsLabel: "a"}}, 0},
		{"common mismatch", models.Webhook{Status: firing, CommonLabels: models.Labels{alertName: "Up"}, GroupLabels: models.Labels{nsLabel: "a"}}, 0},
		{"group mismatch", models.Webhook{Status: firing, CommonLabels: models.Labels{alertName: alertDown}, GroupLabels: models.Labels{nsLabel: "b"}}, 0},
	} {
		action.calls = nil
		if err := r.ReceiveWebhook(context.Background(), &tc.hook); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(action.calls) != tc.want {
			t.Errorf("%s: action ran %d times, want %d", tc.name, len(action.calls), tc.want)
		}
	}
}
