package dbaas

import (
	"encoding/json"
	"strconv"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/leaanthony/debme"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func buildProbeContainers(reqCtx intctrlutil.RequestCtx, params createParams,
	containers []corev1.Container) ([]corev1.Container, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")

	cueTpl, err := params.getCacheCUETplValue("probe_template.cue", func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("probe_template.cue"))
	})
	if err != nil {
		return nil, err
	}
	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)
	probeContainerByte, err := cueValue.Lookup("probeContainer")
	if err != nil {
		return nil, err
	}

	probeContainers := []corev1.Container{}
	componentProbes := params.component.Probes
	reqCtx.Log.Info("probe", "settings", componentProbes)
	if componentProbes == nil {
		return probeContainers, nil
	}

	probeServiceHttpPort := viper.GetInt32("PROBE_SERVICE_PORT")
	availablePorts, err := getAvailableContainerPort(containers, []int32{probeServiceHttpPort, 50001})
	probeServiceHttpPort = availablePorts[0]
	probeServiceGrpcPort := availablePorts[1]
	if err != nil {
		reqCtx.Log.Info("get probe container port failed", "error", err)
		return nil, err
	}
	// TODO: support status and running probes

	if componentProbes.RoleChangedProbe != nil {
		container := corev1.Container{}
		if err = json.Unmarshal(probeContainerByte, &container); err != nil {
			return nil, err
		}
		container.Name = "kbprobe-rolechangedcheck"
		probe := container.ReadinessProbe
		probe.Exec.Command = []string{"curl", "-X", "POST",
			"--fail-with-body", "--silent",
			"-H", "Content-Type: application/json",
			"http://localhost:" + strconv.Itoa(int(probeServiceHttpPort)) + "/v1.0/bindings/probe",
			"-d", "{\"operation\": \"roleCheck\", \"metadata\": {\"sql\" : \"\"}}"}
		probe.PeriodSeconds = componentProbes.RoleChangedProbe.PeriodSeconds
		probe.SuccessThreshold = componentProbes.RoleChangedProbe.SuccessThreshold
		probe.FailureThreshold = componentProbes.RoleChangedProbe.FailureThreshold
		container.StartupProbe.TCPSocket.Port = intstr.FromInt(int(probeServiceHttpPort))
		probeContainers = append(probeContainers, container)
	}

	if len(probeContainers) >= 1 {
		container := &probeContainers[0]
		container.Image = viper.GetString("KUBEBLOCKS_IMAGE")
		container.ImagePullPolicy = corev1.PullPolicy(viper.GetString("KUBEBLOCKS_IMAGE_PULL_POLICY"))
		logLevel := viper.GetString("PROBE_SERVICE_LOG_LEVEL")
		container.Command = []string{"probe", "--app-id", "batch-sdk",
			"--dapr-http-port", strconv.Itoa(int(probeServiceHttpPort)),
			"--dapr-grpc-port", strconv.Itoa(int(probeServiceGrpcPort)),
			"--app-protocol", "http",
			"--log-level", logLevel,
			"--components-path", "/config/components"}

		// set pod name and namespace, for role label updating inside pod
		podName := corev1.EnvVar{
			Name: "MY_POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		}
		podNamespace := corev1.EnvVar{
			Name: "MY_POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		}
		container.Env = append(container.Env, podName, podNamespace)

		container.Ports = []corev1.ContainerPort{{
			ContainerPort: probeServiceHttpPort,
			Name:          "probe-port",
			Protocol:      "TCP",
		}}
	}

	reqCtx.Log.Info("probe", "containers", probeContainers)
	return probeContainers, nil
}
