package alertmanager

import (
	"context"
	"errors"
	"testing"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager/models"
	"github.com/USA-RedDragon/metrics-actioner/internal/config"
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
		{MatchCommonLabels: config.Labels{"alertname": "Down"}, Action: "failing"},
		{MatchCommonLabels: config.Labels{"alertname": "Down"}, Action: "working", Options: config.Options{"n": "1"}},
	}
	r := &Receiver{config: &rules, registeredActions: map[string]ActionIface{"failing": failing, "working": working}}

	err := r.ReceiveWebhook(context.Background(), &models.Webhook{
		Status:       string(models.AlertStatusFiring),
		CommonLabels: models.Labels{"alertname": "Down"},
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
		{MatchCommonLabels: config.Labels{"alertname": "Down"}, MatchGroupLabels: config.Labels{"namespace": "a"}, Action: "x"},
	}
	r := &Receiver{config: &rules, registeredActions: map[string]ActionIface{"x": action}}
	for _, tc := range []struct {
		name string
		hook models.Webhook
		want int
	}{
		{"match", models.Webhook{Status: "firing", CommonLabels: models.Labels{"alertname": "Down", "extra": "y"}, GroupLabels: models.Labels{"namespace": "a"}}, 1},
		{"resolved", models.Webhook{Status: "resolved", CommonLabels: models.Labels{"alertname": "Down"}, GroupLabels: models.Labels{"namespace": "a"}}, 0},
		{"common mismatch", models.Webhook{Status: "firing", CommonLabels: models.Labels{"alertname": "Up"}, GroupLabels: models.Labels{"namespace": "a"}}, 0},
		{"group mismatch", models.Webhook{Status: "firing", CommonLabels: models.Labels{"alertname": "Down"}, GroupLabels: models.Labels{"namespace": "b"}}, 0},
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
