/*
Copyright ApeCloud, Inc.

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

package component

import (
	"embed"
	"encoding/json"

	"github.com/leaanthony/debme"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

const (
	defaultMonitorPort = 9104

	// list of supported CharacterType
	kMysql = "mysql"
)

var (
	supportedCharacterTypeFunc = map[string]func(cluster *appsv1alpha1.Cluster, component *SynthesizedComponent) error{
		kMysql: setMysqlComponent,
	}
	//go:embed cue/*
	cueTemplates embed.FS
)

func buildMonitorConfig(
	cluster *appsv1alpha1.Cluster,
	clusterDef *appsv1alpha1.ClusterDefinition,
	clusterDefComp *appsv1alpha1.ClusterComponentDefinition,
	clusterComp *appsv1alpha1.ClusterComponentSpec,
	component *SynthesizedComponent) {
	monitorEnable := false
	if clusterComp != nil {
		monitorEnable = clusterComp.Monitor
	}

	monitorConfig := clusterDefComp.Monitor
	if !monitorEnable || monitorConfig == nil {
		disableMonitor(component)
		return
	}

	if !monitorConfig.BuiltIn {
		if monitorConfig.Exporter == nil {
			disableMonitor(component)
			return
		}
		component.Monitor = &MonitorConfig{
			Enable:     true,
			ScrapePath: monitorConfig.Exporter.ScrapePath,
			ScrapePort: monitorConfig.Exporter.ScrapePort,
		}
		return
	}

	characterType := clusterDefComp.CharacterType
	if !isSupportedCharacterType(characterType) {
		disableMonitor(component)
		return
	}

	if err := supportedCharacterTypeFunc[characterType](cluster, component); err != nil {
		disableMonitor(component)
	}
}

func disableMonitor(component *SynthesizedComponent) {
	component.Monitor = &MonitorConfig{
		Enable: false,
	}
}

// isSupportedCharacterType check whether the specific CharacterType supports monitoring
func isSupportedCharacterType(characterType string) bool {
	if val, ok := supportedCharacterTypeFunc[characterType]; ok && val != nil {
		return true
	}
	return false
}

// MySQL monitor implementation

type mysqlMonitorConfig struct {
	SecretName      string `json:"secretName"`
	InternalPort    int32  `json:"internalPort"`
	Image           string `json:"image"`
	ImagePullPolicy string `json:"imagePullPolicy"`
}

func buildMysqlMonitorContainer(monitor *mysqlMonitorConfig) (*corev1.Container, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue/monitor")

	cueTpl, err := intctrlutil.NewCUETplFromBytes(cueFS.ReadFile("mysql_template.cue"))
	if err != nil {
		return nil, err
	}
	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)

	mysqlMonitorStrByte, err := json.Marshal(monitor)
	if err != nil {
		return nil, err
	}

	if err = cueValue.Fill("monitor", mysqlMonitorStrByte); err != nil {
		return nil, err
	}

	containerStrByte, err := cueValue.Lookup("container")
	if err != nil {
		return nil, err
	}
	container := corev1.Container{}
	if err = json.Unmarshal(containerStrByte, &container); err != nil {
		return nil, err
	}
	return &container, nil
}

func setMysqlComponent(cluster *appsv1alpha1.Cluster, component *SynthesizedComponent) error {
	image := viper.GetString(constant.KBImage)
	imagePullPolicy := viper.GetString(constant.KBImagePullPolicy)

	// port value is checked against other containers for conflicts.
	port, err := getAvailableContainerPorts(component.PodSpec.Containers, []int32{defaultMonitorPort})
	if err != nil || len(port) != 1 {
		return err
	}

	mysqlMonitorConfig := &mysqlMonitorConfig{
		SecretName:      cluster.Name,
		InternalPort:    port[0],
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
	}

	container, err := buildMysqlMonitorContainer(mysqlMonitorConfig)
	if err != nil {
		return err
	}

	component.PodSpec.Containers = append(component.PodSpec.Containers, *container)
	component.Monitor = &MonitorConfig{
		Enable:     true,
		ScrapePath: "/metrics",
		ScrapePort: mysqlMonitorConfig.InternalPort,
	}
	return nil
}
