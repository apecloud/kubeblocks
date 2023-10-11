package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/template"
	"time"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	ctlruntime "sigs.k8s.io/controller-runtime"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/scheme"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var logger = ctlruntime.Log.WithName("event")

func SentEventForProbe(ctx context.Context, data map[string]any) error {
	logger.Info(fmt.Sprintf("send event: %v", data))
	roleUpdateMechanism := workloads.NoneUpdate
	if viper.IsSet(constant.KBEnvRsmRoleUpdateMechanism) {
		roleUpdateMechanism = workloads.RoleUpdateMechanism(viper.GetString(constant.KBEnvRsmRoleUpdateMechanism))
	}

	switch roleUpdateMechanism {
	case workloads.ReadinessProbeEventUpdate:
		return NewProbeError("not sending event directly, use readiness probe instand")
	case workloads.DirectAPIServerEventUpdate:
		event, err := createEvent(data)
		if err != nil {
			logger.Error(err, "generate event failed")
			return err
		}

		return SendEvent(ctx, event)
	default:
		logger.Info(fmt.Sprintf("no event sent, RoleUpdateMechanism: %s", roleUpdateMechanism))
	}

	return nil
}

func createEvent(data map[string]any) (*corev1.Event, error) {
	eventTmpl := `
apiVersion: v1
kind: Event
metadata:
  name: {{ .PodName }}.{{ .EventSeq }}
  namespace: {{ .Namespace }}
involvedObject:
  apiVersion: v1
  fieldPath: spec.containers{sqlchannel}
  kind: Pod
  name: {{ .PodName }}
  namespace: {{ .Namespace }}
reason: RoleChanged
type: Normal
source:
  component: sqlchannel
`

	// get pod object
	podName := os.Getenv(constant.KBEnvPodName)
	podUID := os.Getenv(constant.KBEnvPodUID)
	nodeName := os.Getenv(constant.KBEnvNodeName)
	namespace := os.Getenv(constant.KBEnvNamespace)
	msg, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	seq := rand.String(16)
	roleValue := map[string]string{
		"PodName":   podName,
		"Namespace": namespace,
		"EventSeq":  seq,
	}
	tmpl, err := template.New("event-tmpl").Parse(eventTmpl)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, roleValue)
	if err != nil {
		return nil, err
	}

	event := &corev1.Event{}
	_, _, err = scheme.Codecs.UniversalDeserializer().Decode(buf.Bytes(), nil, event)
	if err != nil {
		return nil, err
	}
	event.Message = string(msg)
	event.InvolvedObject.UID = types.UID(podUID)
	event.Source.Host = nodeName
	event.Reason = string(data["operation"].(OperationKind))
	event.FirstTimestamp = metav1.Now()
	event.LastTimestamp = metav1.Now()
	event.EventTime = metav1.NowMicro()
	event.ReportingController = "lorry"
	event.ReportingInstance = podName
	event.Action = string(data["operation"].(OperationKind))

	return event, nil
}

func SendEvent(ctx context.Context, event *corev1.Event) error {
	config, err := ctlruntime.GetConfig()
	if err != nil {
		logger.Error(err, "get k8s client config failed")
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Error(err, "k8s client create failed")
		return err
	}
	namespace := os.Getenv(constant.KBEnvNamespace)
	for i := 0; i < 30; i++ {
		_, err = clientset.CoreV1().Events(namespace).Create(ctx, event, metav1.CreateOptions{})
		if err == nil {
			break
		}
		logger.Error(err, "send event failed")
		time.Sleep(10 * time.Second)
	}
	return err
}
