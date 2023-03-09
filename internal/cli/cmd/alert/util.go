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
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/apecloud/kubeblocks/internal/cli/types"
)

// strToMap parses string to map, string format is key1=value1,key2=value2
func strToMap(set string) map[string]string {
	m := make(map[string]string)
	for _, s := range strings.Split(set, ",") {
		pair := strings.Split(s, "=")
		if len(pair) >= 2 {
			m[pair[0]] = strings.Join(pair[1:], "=")
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

func getConfigData(cm *corev1.ConfigMap, key string) (map[string]interface{}, error) {
	dataStr, ok := cm.Data[key]
	if !ok {
		return nil, fmt.Errorf("configmap %s has no data named %s", cm.Name, key)
	}

	data := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(dataStr), &data); err != nil {
		return nil, err
	}
	return data, nil
}

func getReceiversFromData(data map[string]interface{}) []interface{} {
	receivers, ok := data["receivers"]
	if !ok || receivers == nil {
		receivers = []interface{}{} // init receivers
	}
	return receivers.([]interface{})
}

func getRoutesFromData(data map[string]interface{}) []interface{} {
	route, ok := data["route"]
	if !ok || route == nil {
		data["route"] = map[string]interface{}{"routes": []interface{}{}}
	}
	routes, ok := data["route"].(map[string]interface{})["routes"]
	if !ok || routes == nil {
		routes = []interface{}{}
	}
	return routes.([]interface{})
}

func getWebhookType(url string) webhookType {
	if strings.Contains(url, "oapi.dingtalk.com") {
		return dingtalkWebhookType
	}
	if strings.Contains(url, "qyapi.weixin.qq.com") {
		return wechatWebhookType
	}
	if strings.Contains(url, "open.feishu.cn") {
		return feishuWebhookType
	}
	return unknownWebhookType
}

func getWebhookURL(name string, namespace string) string {
	return fmt.Sprintf("http://%s-%s.%s:5001/api/v1/notify/%s", types.KubeBlocksReleaseName, "alertmanager-webhook-adaptor", namespace, name)
}

func removeDuplicateStr(strArray []string) []string {
	var result []string
	for _, s := range strArray {
		if !slices.Contains(result, s) {
			result = append(result, s)
		}
	}
	return result
}

func urlIsValid(urlStr string) (bool, error) {
	_, err := url.ParseRequestURI(urlStr)
	if err != nil {
		return false, err
	}
	return true, nil
}
