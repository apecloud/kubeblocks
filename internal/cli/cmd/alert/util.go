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

package alert

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

// strToMap parses string to map, string format is key1=value1,key2=value2
func strToMap(set string) map[string]string {
	m := make(map[string]string)
	for _, s := range strings.Split(set, ",") {
		pair := strings.Split(s, "=")
		if len(pair) == 2 {
			m[pair[0]] = pair[1]
		}
	}
	return m
}

func severities() []string {
	return []string{string(severityCritical), string(severityWarning), string(severityInfo)}
}

func generateReceiverName() string {
	return fmt.Sprintf("receiver-%s", rand.String(5))
}

func getAlertConfigmap(client kubernetes.Interface) (*corev1.ConfigMap, error) {
	namespace, err := util.GetKubeBlocksNamespace(client)
	if err != nil {
		return nil, err
	}

	return client.CoreV1().ConfigMaps(namespace).Get(context.Background(), alertConfigmapName, metav1.GetOptions{})
}

func getAlertConfigData(alterConfigMap *corev1.ConfigMap) (map[string]interface{}, error) {
	dataStr, ok := alterConfigMap.Data[alertConfigFileName]
	if !ok {
		return nil, fmt.Errorf("alertmanager configmap has no data named alertmanager.yaml")
	}

	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(dataStr), &data); err != nil {
		return nil, err
	}
	return data, nil
}

func getReceiversFromData(data map[string]interface{}) []interface{} {
	// add receiver
	receivers, ok := data["receivers"].([]interface{})
	if !ok {
		receivers = []interface{}{} // init receivers
	}
	return receivers
}

func getRoutesFromData(data map[string]interface{}) []interface{} {
	routes, ok := data["route"].(map[string]interface{})["routes"]
	if !ok {
		routes = []interface{}{} // init routes
	}
	return routes.([]interface{})
}
