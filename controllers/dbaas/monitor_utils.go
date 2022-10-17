/*
Copyright 2022 The KubeBlocks Authors

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

	"github.com/apecloud/kubeblocks/internal/controllerutil"

	"github.com/leaanthony/debme"
	corev1 "k8s.io/api/core/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
)

// ClusterDefinition Type Const Define
const (
	kStateMysql  = "state.mysql"
	kStateMysql8 = "state.mysql-8"
)

// ClusterDefinitionComponent CharacterType Const Define
const (
	KMysql = "mysql"
	KEmpty = ""
)

var (
	kWellKnownTypeMaps = map[string]string{
		kStateMysql:  KMysql,
		kStateMysql8: KMysql,
	}
	WellKnownCharacterTypeFunc = map[string]func(cluster *dbaasv1alpha1.Cluster, component *Component) error{
		KMysql: setMysqlComponent,
	}
)

//===================================
//   CharacterType == MySQL
//===================================

type MysqlMonitor struct {
	SecretName   string `json:"secretName,omitempty"`
	InternalPort int    `json:"internalPort,omitempty"`
}

func buildMysqlContainer(key string, monitor *MysqlMonitor) (*corev1.Container, error) {
	cueFS, _ := debme.FS(controllerutil.CueTemplates, "cue/monitor")

	cueTpl, err := controllerutil.GetCacheCUETplValue("mysql_template.cue", func() (*controllerutil.CUETpl, error) {
		return controllerutil.NewCUETplFromBytes(cueFS.ReadFile("mysql_template.cue"))
	})
	if err != nil {
		return nil, err
	}
	cueValue := controllerutil.NewCUEBuilder(*cueTpl)

	mysqlMonitorStrByte, err := controllerutil.GetCacheBytesValue(key, func() ([]byte, error) {
		return json.Marshal(monitor)
	})
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

func setMysqlComponent(cluster *dbaasv1alpha1.Cluster, component *Component) error {
	mysqlMonitor := &MysqlMonitor{
		SecretName:   cluster.Name,
		InternalPort: 9104,
	}

	container, err := buildMysqlContainer(cluster.Name, mysqlMonitor)
	if err != nil {
		return err
	}

	component.PodSpec.Containers = append(component.PodSpec.Containers, *container)
	component.Monitor = MonitorConfig{
		Enable:     true,
		ScrapePath: "/metrics",
		ScrapePort: mysqlMonitor.InternalPort,
	}
	return nil
}

//===================================
//   CharacterType Logic
//===================================

// CalcCharacterType calc wellknown CharacterType, if not wellknown return empty string
func CalcCharacterType(clusterType string) string {
	if v, ok := kWellKnownTypeMaps[clusterType]; !ok {
		return KEmpty
	} else {
		return v
	}
}

// IsWellKnownCharacterType check CharacterType is wellknown
func IsWellKnownCharacterType(characterType string) bool {
	return isWellKnowCharacterType(characterType, WellKnownCharacterTypeFunc)
}

func isWellKnowCharacterType(characterType string,
	wellKnownCharacterTypeFunc map[string]func(cluster *dbaasv1alpha1.Cluster, component *Component) error) bool {
	val, ok := wellKnownCharacterTypeFunc[characterType]
	if ok && val != nil {
		return true
	}
	return false
}
