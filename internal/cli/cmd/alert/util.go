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

	"helm.sh/helm/v3/pkg/cli/values"

	flag "github.com/spf13/pflag"
	"helm.sh/helm/v3/pkg/action"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
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

func buildHelmCfgByCmdFlags(namespace string, flags *flag.FlagSet) (*action.Configuration, error) {
	config, err := flags.GetString("kubeconfig")
	if err != nil {
		return nil, err
	}
	ctx, err := flags.GetString("context")
	if err != nil {
		return nil, err
	}
	return helm.NewActionConfig(namespace, config, helm.WithContext(ctx))
}

// updateAlterConfig updates alert configuration
func updateAlterConfig(helmCfg *action.Configuration, namespace string, set string) error {
	opts := helm.InstallOpts{
		Name:            types.KubeBlocksChartName,
		Chart:           types.KubeBlocksChartName + "/" + types.KubeBlocksChartName,
		Wait:            false,
		Namespace:       namespace,
		ValueOpts:       &values.Options{JSONValues: []string{set}},
		TryTimes:        2,
		CreateNamespace: false,
	}
	return opts.Upgrade(helmCfg)
}
