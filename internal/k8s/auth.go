package k8s

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func GetConfig() (*rest.Config, error) {
	// We want to try, in order:
	// 1. The KUBECONFIG environment variable
	// 2. The default kubeconfig file
	// 3. In-cluster configuration

	// 1. The KUBECONFIG environment variable
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig != "" {
		config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from KUBECONFIG: %w", err)
		}
		return config, nil
	}

	// 2. The default kubeconfig file
	home := homedir.HomeDir()
	if home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
		if _, err := os.Stat(kubeconfig); err == nil {
			config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err != nil {
				return nil, fmt.Errorf("failed to build config from default kubeconfig: %w", err)
			}
			return config, nil
		}
	}

	// 3. In-cluster configuratio
	return rest.InClusterConfig()
}
