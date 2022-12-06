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
	"strings"

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

	probeSvcHTTPPort := viper.GetInt32("PROBE_SERVICE_PORT")
	availablePorts, err := getAvailableContainerPorts(containers, []int32{probeSvcHTTPPort, 50001})
	probeSvcHTTPPort = availablePorts[0]
	probeServiceGrpcPort := availablePorts[1]
	if err != nil {
		reqCtx.Log.Info("get probe container port failed", "error", err)
		return nil, err
	}

	if componentProbes.RoleChangedProbe != nil {
		roleChangedContainer := container.DeepCopy()
		buildRoleChangedProbeContainer(roleChangedContainer, componentProbes.RoleChangedProbe, int(probeSvcHTTPPort))
		probeContainers = append(probeContainers, *roleChangedContainer)
	}

	if componentProbes.StatusProbe != nil {
		statusProbeContainer := container.DeepCopy()
		buildStatusProbeContainer(statusProbeContainer, componentProbes.StatusProbe, int(probeSvcHTTPPort))
		probeContainers = append(probeContainers, *statusProbeContainer)
	}

	if componentProbes.RunningProbe != nil {
		runningProbeContainer := container.DeepCopy()
		buildRunningProbeContainer(runningProbeContainer, componentProbes.RunningProbe, int(probeSvcHTTPPort))
		probeContainers = append(probeContainers, *runningProbeContainer)
	}

	if len(probeContainers) >= 1 {
		container := &probeContainers[0]
		buildProbeServiceContainer(params.component, container, int(probeSvcHTTPPort), int(probeServiceGrpcPort))
	}

	reqCtx.Log.Info("probe", "containers", probeContainers)
	return probeContainers, nil
}

func buildProbeServiceContainer(component *Component, container *corev1.Container, probeSvcHTTPPort int, probeServiceGrpcPort int) {
	container.Image = viper.GetString("KUBEBLOCKS_IMAGE")
	container.ImagePullPolicy = corev1.PullPolicy(viper.GetString("KUBEBLOCKS_IMAGE_PULL_POLICY"))
	logLevel := viper.GetString("PROBE_SERVICE_LOG_LEVEL")
	container.Command = []string{"probe", "--app-id", "batch-sdk",
		"--dapr-http-port", strconv.Itoa(probeSvcHTTPPort),
		"--dapr-grpc-port", strconv.Itoa(probeServiceGrpcPort),
		"--app-protocol", "http",
		"--log-level", logLevel,
		"--config", "/config/probe/config.yaml",
		"--components-path", "/config/probe/components"}

	if component.Service != nil && len(component.Service.Ports) > 0 {
		port := component.Service.Ports[0]
		dbPort := port.TargetPort.IntValue()
		if dbPort == 0 {
			dbPort = int(port.Port)
		}
		container.Env = append(container.Env, corev1.EnvVar{
			Name:      dbaasPrefix + "_SERVICE_PORT",
			Value:     strconv.Itoa(dbPort),
			ValueFrom: nil,
		})
	}

	roles := getComponentRoles(component)
	rolesJSON, _ := json.Marshal(roles)
	container.Env = append(container.Env, corev1.EnvVar{
		Name:      dbaasPrefix + "_SERVICE_ROLES",
		Value:     string(rolesJSON),
		ValueFrom: nil,
	})

	container.Env = append(container.Env, corev1.EnvVar{
		Name:      dbaasPrefix + "_SERVICE_CHARACTER_TYPE",
		Value:     component.CharacterType,
		ValueFrom: nil,
	})

	container.Ports = []corev1.ContainerPort{{
		ContainerPort: int32(probeSvcHTTPPort),
		Name:          "probe-port",
		Protocol:      "TCP",
	}}

	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      rootSecretVolumeName,
		MountPath: "/etc/credential",
	})
}

func getComponentRoles(component *Component) map[string]string {
	var roles = map[string]string{}
	if component.ConsensusSpec == nil {
		return roles
	}

	consensus := component.ConsensusSpec
	roles[strings.ToLower(consensus.Leader.Name)] = string(consensus.Leader.AccessMode)
	for _, follower := range consensus.Followers {
		roles[strings.ToLower(follower.Name)] = string(follower.AccessMode)
	}
	if consensus.Learner != nil {
		roles[strings.ToLower(consensus.Learner.Name)] = string(consensus.Learner.AccessMode)
	}
	return roles
}

func buildRoleChangedProbeContainer(roleChangedContainer *corev1.Container,
	probeSetting *dbaasv1alpha1.ClusterDefinitionProbe, probeSvcHTTPPort int) {
	roleChangedContainer.Name = "kb-rolechangedcheck"
	probe := roleChangedContainer.ReadinessProbe
	probe.Exec.Command = []string{"curl", "-X", "POST",
		"--fail-with-body", "--silent",
		"-H", "Content-Type: application/json",
		"http://localhost:" + strconv.Itoa(probeSvcHTTPPort) + "/v1.0/bindings/probe",
		"-d", "{\"operation\": \"roleCheck\", \"metadata\": {\"sql\" : \"\"}}"}
	probe.PeriodSeconds = probeSetting.PeriodSeconds
	roleChangedContainer.StartupProbe.TCPSocket.Port = intstr.FromInt(probeSvcHTTPPort)
}

func buildStatusProbeContainer(statusProbeContainer *corev1.Container,
	probeSetting *dbaasv1alpha1.ClusterDefinitionProbe, probeSvcHTTPPort int) {
	statusProbeContainer.Name = "kb-statuscheck"
	probe := statusProbeContainer.ReadinessProbe
	httpGet := &corev1.HTTPGetAction{}
	httpGet.Path = "/v1.0/bindings/probe?operation=statusCheck"
	httpGet.Port = intstr.FromInt(probeSvcHTTPPort)
	probe.Exec = nil
	probe.HTTPGet = httpGet
	probe.PeriodSeconds = probeSetting.PeriodSeconds
	statusProbeContainer.StartupProbe.TCPSocket.Port = intstr.FromInt(probeSvcHTTPPort)
}

func buildRunningProbeContainer(runningProbeContainer *corev1.Container,
	probeSetting *dbaasv1alpha1.ClusterDefinitionProbe, probeSvcHTTPPort int) {
	runningProbeContainer.Name = "kb-runningcheck"
	probe := runningProbeContainer.ReadinessProbe
	httpGet := &corev1.HTTPGetAction{}
	httpGet.Path = "/v1.0/bindings/probe?operation=runningCheck"
	httpGet.Port = intstr.FromInt(probeSvcHTTPPort)
	probe.Exec = nil
	probe.HTTPGet = httpGet
	probe.PeriodSeconds = probeSetting.PeriodSeconds
	runningProbeContainer.StartupProbe.TCPSocket.Port = intstr.FromInt(probeSvcHTTPPort)
}
