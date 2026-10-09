package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager"
	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager/models"
	"github.com/USA-RedDragon/metrics-actioner/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

const webhookPath = "/api/v1/webhooks/alertmanager"

func receiverFor(t *testing.T, rules []config.Action) *alertmanager.Receiver {
	t.Helper()
	r, err := alertmanager.NewReceiver(&rules, prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func router(cfg *config.HTTP, receiver *alertmanager.Receiver) *gin.Engine {
	r := gin.New()
	applyMiddleware(r, cfg, "api", receiver)
	applyRoutes(r)
	return r
}

func post(r http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, webhookPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type blockingAction struct {
	started chan struct{}
	release chan struct{}
}

func (b *blockingAction) Execute(ctx context.Context, _ *models.Webhook, _ map[string]string) error {
	close(b.started)
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestPProfWriteTimeoutOnBothStacks(t *testing.T) {
	t.Parallel()
	cfg := &config.HTTP{PProf: config.PProf{Enabled: true}}
	s := NewServer(cfg, receiverFor(t, nil))
	if s.ipv4Server.WriteTimeout != pprofWriteTimeout {
		t.Errorf("IPv4 write timeout = %s, want %s", s.ipv4Server.WriteTimeout, pprofWriteTimeout)
	}
	if s.ipv6Server.WriteTimeout != pprofWriteTimeout {
		t.Errorf("IPv6 write timeout = %s, want %s", s.ipv6Server.WriteTimeout, pprofWriteTimeout)
	}
}

func TestWebhookRepliesWhileActionRuns(t *testing.T) {
	t.Parallel()
	receiver := receiverFor(t, []config.Action{{Action: "slow", MatchCommonLabels: config.Labels{"alertname": "Down"}}})
	slow := &blockingAction{started: make(chan struct{}), release: make(chan struct{})}
	receiver.RegisterAction("slow", slow)
	r := router(&config.HTTP{}, receiver)

	start := time.Now()
	w := post(r, `{"status":"firing","groupKey":"g1","commonLabels":{"alertname":"Down"}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("handler took %s while the action was still running", elapsed)
	}

	select {
	case <-slow.started:
	case <-time.After(5 * time.Second):
		t.Fatal("action never started")
	}
	close(slow.release)
	if err := receiver.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestWebhookRejectsBadPayload(t *testing.T) {
	t.Parallel()
	r := router(&config.HTTP{}, receiverFor(t, nil))
	for _, body := range []string{`{`, `{"status":"bogus"}`, `{"status":5}`} {
		if w := post(r, body); w.Code != http.StatusBadRequest {
			t.Errorf("body %s: status = %d, want 400", body, w.Code)
		}
	}
	if w := post(r, `{"status":"resolved"}`); w.Code != http.StatusOK {
		t.Errorf("resolved webhook: status = %d, want 200", w.Code)
	}
}
