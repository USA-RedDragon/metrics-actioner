package alertmanager

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager/models"
	"github.com/USA-RedDragon/metrics-actioner/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

const (
	alertName = "alertname"
	alertDown = "Down"
	nsLabel   = "namespace"
	firing    = string(models.AlertStatusFiring)
	slowName  = "slow"
)

// fakeAction records calls. With block set, it waits for release or for
// its context to end.
type fakeAction struct {
	err     error
	block   bool
	release chan struct{}
	started chan struct{}

	mu        sync.Mutex
	calls     int
	cancelled int
}

func newFake(block bool, err error) *fakeAction {
	return &fakeAction{err: err, block: block, release: make(chan struct{}), started: make(chan struct{}, 16)}
}

func (f *fakeAction) Execute(ctx context.Context, _ *models.Webhook, _ map[string]string) error {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	f.started <- struct{}{}
	if f.block {
		select {
		case <-f.release:
		case <-ctx.Done():
			f.mu.Lock()
			f.cancelled++
			f.mu.Unlock()
			return ctx.Err()
		}
	}
	return f.err
}

func (f *fakeAction) counts() (calls, cancelled int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls, f.cancelled
}

func (f *fakeAction) waitStarted(t *testing.T) {
	t.Helper()
	select {
	case <-f.started:
	case <-time.After(5 * time.Second):
		t.Fatal("action never started")
	}
}

func newTestReceiver(t *testing.T, rules []config.Action, fakes map[string]*fakeAction) (*Receiver, *bytes.Buffer) {
	t.Helper()
	r, err := NewReceiver(&rules, prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	r.logger = slog.New(slog.NewTextHandler(&lockedWriter{w: &logs}, nil))
	for name, f := range fakes {
		r.RegisterAction(name, f)
	}
	return r, &logs
}

type lockedWriter struct {
	mu sync.Mutex
	w  *bytes.Buffer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func downHook(groupKey string) *models.Webhook {
	return &models.Webhook{Status: firing, GroupKey: groupKey, CommonLabels: models.Labels{alertName: alertDown}}
}

func rule(action string) config.Action {
	return config.Action{MatchCommonLabels: config.Labels{alertName: alertDown}, Action: action}
}

func TestFailingActionIsLoggedCountedAndDoesNotSkipLaterRules(t *testing.T) {
	t.Parallel()
	failing := newFake(false, errors.New("boom"))
	working := newFake(false, nil)
	r, logs := newTestReceiver(t, []config.Action{rule("failing"), rule("working")},
		map[string]*fakeAction{"failing": failing, "working": working})

	if !r.Dispatch(downHook("g")) {
		t.Fatal("Dispatch did not start the actions")
	}
	if err := r.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls, _ := failing.counts(); calls != 1 {
		t.Errorf("failing action ran %d times, want 1", calls)
	}
	if calls, _ := working.counts(); calls != 1 {
		t.Errorf("second matching rule ran %d times, want 1", calls)
	}
	if got := testutil.ToFloat64(r.runs.WithLabelValues("failing")); got != 1 {
		t.Errorf("runs{failing} = %v, want 1", got)
	}
	if got := testutil.ToFloat64(r.failures.WithLabelValues("failing")); got != 1 {
		t.Errorf("failures{failing} = %v, want 1", got)
	}
	if got := testutil.ToFloat64(r.failures.WithLabelValues("working")); got != 0 {
		t.Errorf("failures{working} = %v, want 0", got)
	}
	out := logs.String()
	for _, want := range []string{"Action failed", "action=failing", "rule=0", "group_key=g", "error=boom"} {
		if !strings.Contains(out, want) {
			t.Errorf("failure log missing %q:\n%s", want, out)
		}
	}
}

func TestActionTimeoutIsAFailure(t *testing.T) {
	t.Parallel()
	slow := newFake(true, nil)
	r, _ := newTestReceiver(t, []config.Action{rule(slowName)}, map[string]*fakeAction{slowName: slow})
	r.actionTimeout = 50 * time.Millisecond

	r.Dispatch(downHook("g"))
	if err := r.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, cancelled := slow.counts(); cancelled != 1 {
		t.Errorf("action was not stopped by its timeout")
	}
	if got := testutil.ToFloat64(r.failures.WithLabelValues(slowName)); got != 1 {
		t.Errorf("failures{slow} = %v, want 1", got)
	}
}

func TestDedupesInFlightGroup(t *testing.T) {
	t.Parallel()
	slow := newFake(true, nil)
	r, logs := newTestReceiver(t, []config.Action{rule(slowName)}, map[string]*fakeAction{slowName: slow})

	if !r.Dispatch(downHook("g1")) {
		t.Fatal("first webhook did not start")
	}
	slow.waitStarted(t)
	if r.Dispatch(downHook("g1")) {
		t.Error("resend of an in-flight group started its actions again")
	}
	if !strings.Contains(logs.String(), "still running") {
		t.Errorf("skip not logged:\n%s", logs.String())
	}
	if !r.Dispatch(downHook("g2")) {
		t.Error("a different group was skipped")
	}
	slow.waitStarted(t)

	close(slow.release)
	deadline := time.Now().Add(5 * time.Second)
	for !r.Dispatch(downHook("g1")) {
		if time.Now().After(deadline) {
			t.Fatal("group g1 stayed in flight after its actions finished")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := r.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls, _ := slow.counts(); calls != 3 {
		t.Errorf("action ran %d times, want 3 (g1, g2, g1 again)", calls)
	}
}

func TestShutdownWaitsForRunningActions(t *testing.T) {
	t.Parallel()
	slow := newFake(true, nil)
	r, _ := newTestReceiver(t, []config.Action{rule(slowName)}, map[string]*fakeAction{slowName: slow})
	r.Dispatch(downHook("g"))
	slow.waitStarted(t)

	done := make(chan error, 1)
	go func() { done <- r.Shutdown(context.Background()) }()
	select {
	case <-done:
		t.Fatal("Shutdown returned while an action was still running")
	case <-time.After(100 * time.Millisecond):
	}
	close(slow.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, cancelled := slow.counts(); cancelled != 0 {
		t.Error("an action that finished in time was cancelled")
	}
	if r.Dispatch(downHook("g2")) {
		t.Error("Dispatch started actions after Shutdown")
	}
}

func TestShutdownCancelsActionsAfterBound(t *testing.T) {
	t.Parallel()
	slow := newFake(true, nil)
	r, _ := newTestReceiver(t, []config.Action{rule(slowName)}, map[string]*fakeAction{slowName: slow})
	r.Dispatch(downHook("g"))
	slow.waitStarted(t)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := r.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("Shutdown took %s", elapsed)
	}
	if _, cancelled := slow.counts(); cancelled != 1 {
		t.Error("running action was not cancelled")
	}
}

func TestMatching(t *testing.T) {
	t.Parallel()
	r, _ := newTestReceiver(t, []config.Action{
		{MatchCommonLabels: config.Labels{alertName: alertDown}, MatchGroupLabels: config.Labels{nsLabel: "a"}, Action: "x"},
	}, nil)
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
		if got := len(r.matchingRules(&tc.hook)); got != tc.want {
			t.Errorf("%s: matched %d rules, want %d", tc.name, got, tc.want)
		}
	}
}
