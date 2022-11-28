/*
Copyright ApeCloud Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dbaas

import (
	"encoding/json"
	"strconv"

	"github.com/leaanthony/debme"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
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
	container := corev1.Container{}
	if err = json.Unmarshal(probeContainerByte, &container); err != nil {
		return nil, err
	}

	probeContainers := []corev1.Container{}
	componentProbes := params.component.Probes
	reqCtx.Log.Info("probe", "settings", componentProbes)
	if componentProbes == nil {
		return probeContainers, nil
	}

	probeServiceHTTPPort := viper.GetInt32("PROBE_SERVICE_PORT")
	availablePorts, err := getAvailableContainerPorts(containers, []int32{probeServiceHTTPPort, 50001})
	probeServiceHTTPPort = availablePorts[0]
	probeServiceGrpcPort := availablePorts[1]
	if err != nil {
		reqCtx.Log.Info("get probe container port failed", "error", err)
		return nil, err
	}

	if componentProbes.RoleChangedProbe != nil {
		roleChangedContainer := container.DeepCopy()
		buildRoleChangedProbeContainer(roleChangedContainer, componentProbes.RoleChangedProbe, int(probeServiceHTTPPort))
		probeContainers = append(probeContainers, *roleChangedContainer)
	}

	if componentProbes.StatusProbe != nil {
		statusProbeContainer := container.DeepCopy()
		buildStatusProbeContainer(statusProbeContainer, componentProbes.StatusProbe, int(probeServiceHTTPPort))
		probeContainers = append(probeContainers, *statusProbeContainer)
	}

	if componentProbes.RunningProbe != nil {
		runningProbeContainer := container.DeepCopy()
		buildRunningProbeContainer(runningProbeContainer, componentProbes.RunningProbe, int(probeServiceHTTPPort))
		probeContainers = append(probeContainers, *runningProbeContainer)
	}

	if len(probeContainers) >= 1 {
		container := &probeContainers[0]
		buildProbeServiceContainer(params.component, container, int(probeServiceHTTPPort), int(probeServiceGrpcPort))
	}

	reqCtx.Log.Info("probe", "containers", probeContainers)
	return probeContainers, nil
}

func buildProbeServiceContainer(component *Component, container *corev1.Container, probeServiceHTTPPort int, probeServiceGrpcPort int) {
	container.Image = viper.GetString("KUBEBLOCKS_IMAGE")
	container.ImagePullPolicy = corev1.PullPolicy(viper.GetString("KUBEBLOCKS_IMAGE_PULL_POLICY"))
	logLevel := viper.GetString("PROBE_SERVICE_LOG_LEVEL")
	container.Command = []string{"probe", "--app-id", "batch-sdk",
		"--dapr-http-port", strconv.Itoa(probeServiceHTTPPort),
		"--dapr-grpc-port", strconv.Itoa(probeServiceGrpcPort),
		"--app-protocol", "http",
		"--log-level", logLevel,
		"--config", "/config/dapr/config.yaml",
		"--components-path", "/config/dapr/components"}

	if len(component.Service.Ports) > 0 {
		port := component.Service.Ports[0]
		dbPort := port.TargetPort.IntValue()
		if dbPort == 0 {
			dbPort = int(port.Port)
		}
		container.Env = append(container.Env, corev1.EnvVar{
			Name:      dbaasPrefix + "_DB_PORT",
			Value:     strconv.Itoa(dbPort),
			ValueFrom: nil,
		})
	}

	container.Env = append(container.Env, corev1.EnvVar{
		Name:      dbaasPrefix + "_DB_CHARACTER_TYPE",
		Value:     component.CharacterType,
		ValueFrom: nil,
	})

	container.Ports = []corev1.ContainerPort{{
		ContainerPort: int32(probeServiceHTTPPort),
		Name:          "probe-port",
		Protocol:      "TCP",
	}}
}

func buildRoleChangedProbeContainer(roleChangedContainer *corev1.Container,
	probeSetting *dbaasv1alpha1.ClusterDefinitionProbe, probeServiceHTTPPort int) {
	roleChangedContainer.Name = "kb-rolechangedcheck"
	probe := roleChangedContainer.ReadinessProbe
	probe.Exec.Command = []string{"curl", "-X", "POST",
		"--fail-with-body", "--silent",
		"-H", "Content-Type: application/json",
		"http://localhost:" + strconv.Itoa(probeServiceHTTPPort) + "/v1.0/bindings/probe",
		"-d", "{\"operation\": \"roleCheck\", \"metadata\": {\"sql\" : \"\"}}"}
	probe.PeriodSeconds = probeSetting.PeriodSeconds
	roleChangedContainer.StartupProbe.TCPSocket.Port = intstr.FromInt(probeServiceHTTPPort)
}

func buildStatusProbeContainer(statusProbeContainer *corev1.Container,
	probeSetting *dbaasv1alpha1.ClusterDefinitionProbe, probeServiceHTTPPort int) {
	statusProbeContainer.Name = "kb-statuscheck"
	probe := statusProbeContainer.ReadinessProbe
	httpGet := &corev1.HTTPGetAction{}
	httpGet.Path = "/v1.0/bindings/probe?operation=statusCheck"
	httpGet.Port = intstr.FromInt(probeServiceHTTPPort)
	probe.Exec = nil
	probe.HTTPGet = httpGet
	probe.PeriodSeconds = probeSetting.PeriodSeconds
	statusProbeContainer.StartupProbe.TCPSocket.Port = intstr.FromInt(probeServiceHTTPPort)
}

func buildRunningProbeContainer(runningProbeContainer *corev1.Container,
	probeSetting *dbaasv1alpha1.ClusterDefinitionProbe, probeServiceHTTPPort int) {
	runningProbeContainer.Name = "kb-runningcheck"
	probe := runningProbeContainer.ReadinessProbe
	httpGet := &corev1.HTTPGetAction{}
	httpGet.Path = "/v1.0/bindings/probe?operation=runningCheck"
	httpGet.Port = intstr.FromInt(probeServiceHTTPPort)
	probe.Exec = nil
	probe.HTTPGet = httpGet
	probe.PeriodSeconds = probeSetting.PeriodSeconds
	runningProbeContainer.StartupProbe.TCPSocket.Port = intstr.FromInt(probeServiceHTTPPort)
}
