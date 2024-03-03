package actions

//nolint:golint,revive
import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/USA-RedDragon/metrics-actioner/internal/alertmanager/models"
	"github.com/USA-RedDragon/metrics-actioner/internal/k8s"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

type RolloutRestartDeployment struct {
}

type Options struct {
	Namespace  string
	Deployment string
}

func (r *RolloutRestartDeployment) Execute(webhook *models.Webhook, options map[string]string) error {
	slog.Info("RolloutRestartDeployment action executed")
	var opts Options
	// Get the options
	for k, v := range options {
		switch k {
		case "namespace":
			opts.Namespace = v
		case "deployment":
			opts.Deployment = v
		default:
			slog.Warn("Unknown option", "option", k)
		}
	}
	// Validate the options
	if opts.Deployment == "" {
		return fmt.Errorf("missing deployment option")
	}
	if opts.Namespace == "" {
		// Default to the namespace of the alert
		var ok bool
		opts.Namespace, ok = webhook.CommonLabels["namespace"]
		if !ok {
			return fmt.Errorf("missing namespace option")
		}
	}

	return r.restart(opts)
}

func (r *RolloutRestartDeployment) restart(opts Options) error {
	// Now we essentially run `kubectl -n <namespace> rollout restart deployment <deployment>`
	slog.Info("Restarting deployment", "namespace", opts.Namespace, "deployment", opts.Deployment)

	kubeconfig, err := k8s.GetConfig()
	if err != nil {
		return err
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return err
	}

	deploymentsClient := clientset.AppsV1().Deployments(opts.Namespace)
	data := fmt.Sprintf(`{"spec": {"template": {"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": "%s"}}}}}`, time.Now().Format("20060102150405"))
	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()
	_, err = deploymentsClient.Patch(ctx, opts.Deployment, types.StrategicMergePatchType, []byte(data), v1.PatchOptions{})
	if err != nil {
		return err
	}

	return nil
}
