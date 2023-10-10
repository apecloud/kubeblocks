package util

import (
	"context"
	"os"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctlruntime "sigs.k8s.io/controller-runtime"

	"github.com/apecloud/kubeblocks/internal/constant"
)

func SendEvent(ctx context.Context, log logr.Logger, event *corev1.Event) error {
	config, err := ctlruntime.GetConfig()
	if err != nil {
		log.Error(err, "get k8s client config failed")
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error(err, "k8s client create failed")
		return err
	}
	namespace := os.Getenv(constant.KBEnvNamespace)
	for i := 0; i < 30; i++ {
		_, err = clientset.CoreV1().Events(namespace).Create(ctx, event, metav1.CreateOptions{})
		if err == nil {
			break
		}
		log.Error(err, "send event failed")
		time.Sleep(10 * time.Second)
	}
	return err
}
