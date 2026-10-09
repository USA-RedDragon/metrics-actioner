package alertmanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager/models"
	"github.com/USA-RedDragon/metrics-actioner/internal/config"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// DefaultActionTimeout caps how long a single action may run.
	DefaultActionTimeout = 5 * time.Minute
	// cancelGrace is how long Shutdown waits for actions to return after
	// cancelling them.
	cancelGrace = 5 * time.Second
)

// ErrActionsDidNotStop is returned by Shutdown when cancelled actions do not
// return within the grace period.
var ErrActionsDidNotStop = errors.New("actions did not stop after cancellation")

// Receiver matches AlertManager webhooks against the configured rules and
// runs the matching actions in the background.
type Receiver struct {
	config            *[]config.Action
	registeredActions map[string]ActionIface
	actionTimeout     time.Duration
	logger            *slog.Logger

	ctx    context.Context //nolint:containedctx // server-lifetime context the background actions run on
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu       sync.Mutex
	closed   bool
	inFlight map[string]struct{}

	runs     *prometheus.CounterVec
	failures *prometheus.CounterVec
}

// NewReceiver creates a Receiver and registers its metrics with reg.
func NewReceiver(rules *[]config.Action, reg prometheus.Registerer) (*Receiver, error) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &Receiver{
		config:            rules,
		registeredActions: findActions(),
		actionTimeout:     DefaultActionTimeout,
		logger:            slog.Default(),
		ctx:               ctx,
		cancel:            cancel,
		inFlight:          make(map[string]struct{}),
		runs: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "metrics_actioner_action_runs_total",
			Help: "Actions started, by action.",
		}, []string{"action"}),
		failures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "metrics_actioner_action_failures_total",
			Help: "Actions that returned an error, by action.",
		}, []string{"action"}),
	}
	for _, c := range []prometheus.Collector{r.runs, r.failures} {
		if err := reg.Register(c); err != nil {
			cancel()
			return nil, fmt.Errorf("failed to register metrics: %w", err)
		}
	}
	for name := range r.registeredActions {
		r.runs.WithLabelValues(name)
		r.failures.WithLabelValues(name)
	}
	return r, nil
}

// RegisterAction adds or replaces the action run for rules naming it.
func (r *Receiver) RegisterAction(name string, action ActionIface) {
	r.registeredActions[name] = action
}

func matchLabels(webhookLabels models.Labels, ruleLabels config.Labels) bool {
	for key, value := range ruleLabels {
		if webhookLabels[key] != value {
			return false
		}
	}
	return true
}

// matchingRules returns the indexes of the rules a firing webhook matches.
func (r *Receiver) matchingRules(webhook *models.Webhook) []int {
	if webhook.Status != string(models.AlertStatusFiring) {
		return nil
	}
	var matched []int
	for i, rule := range *r.config {
		if !matchLabels(webhook.CommonLabels, rule.MatchCommonLabels) {
			continue
		}
		if !matchLabels(webhook.GroupLabels, rule.MatchGroupLabels) {
			continue
		}
		matched = append(matched, i)
	}
	return matched
}

// Dispatch starts the actions of every rule the webhook matches in the
// background and returns without waiting for them. It returns false when
// nothing was started: no rule matched, the receiver is shutting down, or
// the actions for the same alert group are still running.
func (r *Receiver) Dispatch(webhook *models.Webhook) bool {
	key := webhook.GroupKey
	r.logger.Info("Received AlertManager webhook", "group_key", key, "status", webhook.Status, "alerts", len(webhook.Alerts))

	rules := r.matchingRules(webhook)
	if len(rules) == 0 {
		return false
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		r.logger.Warn("Shutting down, not running actions", "group_key", key)
		return false
	}
	if key != "" {
		if _, busy := r.inFlight[key]; busy {
			r.mu.Unlock()
			r.logger.Info("Actions for this alert group are still running, skipping", "group_key", key)
			return false
		}
		r.inFlight[key] = struct{}{}
	}
	r.wg.Add(1)
	r.mu.Unlock()

	go func() {
		defer r.wg.Done()
		defer func() {
			if key != "" {
				r.mu.Lock()
				delete(r.inFlight, key)
				r.mu.Unlock()
			}
		}()
		r.run(webhook, rules)
	}()
	return true
}

func (r *Receiver) run(webhook *models.Webhook, rules []int) {
	for _, i := range rules {
		rule := (*r.config)[i]
		if r.ctx.Err() != nil {
			r.logger.Warn("Shutting down, skipping remaining actions", "action", rule.Action, "rule", i, "group_key", webhook.GroupKey)
			return
		}
		r.logger.Info("Running action", "action", rule.Action, "rule", i, "group_key", webhook.GroupKey)
		r.runs.WithLabelValues(rule.Action).Inc()
		if err := r.execute(webhook, rule); err != nil {
			r.failures.WithLabelValues(rule.Action).Inc()
			r.logger.Error("Action failed", "action", rule.Action, "rule", i, "group_key", webhook.GroupKey, "error", err.Error())
			continue
		}
		r.logger.Info("Action succeeded", "action", rule.Action, "rule", i, "group_key", webhook.GroupKey)
	}
}

func (r *Receiver) execute(webhook *models.Webhook, rule config.Action) error {
	action, err := r.FindAction(rule.Action)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(r.ctx, r.actionTimeout)
	defer cancel()
	return action.Execute(ctx, webhook, rule.Options)
}

// Shutdown stops accepting webhooks and waits for running actions. When ctx
// ends first, it cancels them and waits up to a short grace period more.
func (r *Receiver) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()

	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		r.cancel()
		return nil
	case <-ctx.Done():
	}

	r.logger.Warn("Cancelling running actions")
	r.cancel()
	select {
	case <-done:
		return nil
	case <-time.After(cancelGrace):
		return ErrActionsDidNotStop
	}
}
