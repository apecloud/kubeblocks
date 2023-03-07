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
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

const (
	// alertConfigFileName is the name of alertmanager config file
	alertConfigFileName = "alertmanager.yml"
)

var (
	// alertConfigmapName is the name of alertmanager configmap
	alertConfigmapName = fmt.Sprintf("%s-alertmanager-config", types.KubeBlocksReleaseName)
)

var (
	addReceiverExample = templates.Examples(`
		# add webhookConfig receiver, for example feishu
		kbcli alert add-receiver --webhook='url=https://open.feishu.cn/open-apis/bot/v2/hook/foo,token=xxxxx'

		# add emailConfig receiver
        kbcli alter add-receiver --email='a@foo.com,b@foo.com'

		# add emailConfig receiver, and only receive alert from cluster mycluster
		kbcli alter add-receiver --email='a@foo.com,b@foo.com' --cluster=mycluster

		# add emailConfig receiver, and only receive alert from cluster mycluster and alert severity is warning
		kbcli alter add-receiver --email='a@foo.com,b@foo.com' --cluster=mycluster --severity=warning

		# add slackConfig receiver
  		kbcli alert add-receiver --slack api_url=https://hooks.slackConfig.com/services/foo,channel=monitor,username=kubeblocks-alert-bot`)
)

type baseOptions struct {
	genericclioptions.IOStreams
	alterConfigMap *corev1.ConfigMap
	client         kubernetes.Interface
}

type addReceiverOptions struct {
	baseOptions

	emails     []string
	webhooks   []string
	slacks     []string
	clusters   []string
	severities []string
	name       string
}

func newAddReceiverCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := addReceiverOptions{baseOptions: baseOptions{IOStreams: streams}}
	cmd := &cobra.Command{
		Use:     "add-receiver",
		Short:   "Add alert receiver, such as emailConfig, slackConfig, webhookConfig and so on",
		Example: addReceiverExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f))
			util.CheckErr(o.validate(args))
			util.CheckErr(o.run())
		},
	}

	cmd.Flags().StringArrayVar(&o.emails, "email", []string{}, "Add email address, such as bar@foo.com, more than one emailConfig can be specified separated by comma")
	cmd.Flags().StringArrayVar(&o.webhooks, "webhook", []string{}, "Add webhook receiver, such as url=https://open.feishu.cn/open-apis/bot/v2/hook/foo,token=xxxxx")
	cmd.Flags().StringArrayVar(&o.slacks, "slack", []string{}, "Add slack receiver, such as api_url=https://hooks.slackConfig.com/services/foo,channel=monitor,username=kubeblocks-alert-bot")
	cmd.Flags().StringArrayVar(&o.clusters, "cluster", []string{}, "Cluster name, such as mycluster, more than one cluster can be specified, such as mycluster,mycluster2")
	cmd.Flags().StringArrayVar(&o.severities, "severity", []string{}, "Alert severity, such as critical, warning, info, more than one severity can be specified, such as critical,warning")

	// register completions
	util.CheckErr(cmd.RegisterFlagCompletionFunc("severity",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return severities(), cobra.ShellCompDirectiveNoFileComp
		}))

	return cmd
}

func (o *baseOptions) complete(f cmdutil.Factory) error {
	var err error

	o.client, err = f.KubernetesClientSet()
	if err != nil {
		return err
	}

	o.alterConfigMap, err = getAlertConfigmap(o.client)
	return err
}

func (o *addReceiverOptions) validate(args []string) error {
	if len(o.emails) == 0 && len(o.webhooks) == 0 && len(o.slacks) == 0 {
		return fmt.Errorf("must specify at least one receiver, such as --emailConfig, --webhookConfig or --slackConfig")
	}

	// if name is not specified, generate a random name
	if len(args) == 0 {
		o.name = generateReceiverName()
	} else {
		o.name = args[0]
	}

	return nil
}

func (o *addReceiverOptions) run() error {
	// build receiver
	receiver, err := o.buildReceiver()
	if err != nil {
		return err
	}

	// build route
	route, err := o.buildRoute()
	if err != nil {
		return err
	}

	if err = o.addReceiver(receiver, route); err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "receiver %s added successfully", receiver.Name)
	return nil
}

// buildReceiver builds receiver from receiver options
func (o *addReceiverOptions) buildReceiver() (*receiver, error) {
	webhookConfigs, err := buildWebhookConfigs(o.webhooks)
	if err != nil {
		return nil, err
	}

	slackConfigs, err := buildSlackConfigs(o.slacks)
	if err != nil {
		return nil, err
	}

	r := &receiver{
		Name:           o.name,
		EmailConfigs:   buildEmailConfigs(o.emails),
		WebhookConfigs: webhookConfigs,
		SlackConfigs:   slackConfigs,
	}
	return r, nil
}

func (o *addReceiverOptions) buildRoute() (*route, error) {
	r := &route{
		Receiver: o.name,
		Continue: true,
	}

	var clusterArray []string
	var severityArray []string

	splitStr := func(strArray []string, target *[]string) {
		for _, s := range strArray {
			ss := strings.Split(s, ",")
			*target = append(*target, ss...)
		}
	}

	// parse clusters and severities
	splitStr(o.clusters, &clusterArray)
	splitStr(o.severities, &severityArray)

	// build matchers
	buildMatchers := func(t string, values []string) string {
		if len(values) == 0 {
			return ""
		}
		switch t {
		case routeMatcherClusterType:
			return routeMatcherClusterKey + routeMatcherOperator + strings.Join(values, "|")
		case routeMatcherSeverityType:
			return routeMatcherSeverityKey + routeMatcherOperator + strings.Join(values, "|")
		default:
			return ""
		}
	}

	r.Matchers = append(r.Matchers, buildMatchers(routeMatcherClusterType, clusterArray),
		buildMatchers(routeMatcherSeverityType, severityArray))
	return r, nil
}

// addReceiver adds receiver to alertmanager config
func (o *addReceiverOptions) addReceiver(receiver *receiver, route *route) error {
	data, err := getAlertConfigData(o.alterConfigMap)
	if err != nil {
		return err
	}

	// add receiver
	receivers := getReceiversFromData(data)
	receivers = append(receivers, receiver)

	// add route
	routes := getRoutesFromData(data)
	routes = append(routes, route)

	data["receivers"] = receivers
	data["route"].(map[string]interface{})["routes"] = routes

	// update alertmanager configmap
	return updateAlertConfig(o.client, o.alterConfigMap.Namespace, data)
}

// buildWebhookConfigs builds webhookConfig from webhook options
func buildWebhookConfigs(webhooks []string) ([]*webhookConfig, error) {
	var ws []*webhookConfig
	for _, hook := range webhooks {
		m := strToMap(hook)
		if len(m) == 0 {
			return nil, fmt.Errorf("invalid webhook: %s, webhook should be in the format of url=my-url,tolen=my-token", hook)
		}
		w := webhookConfig{
			MaxAlerts:    10,
			SendResolved: false,
		}
		for k, v := range m {
			// check webhookConfig keys
			switch webhookKey(k) {
			case webhookURL:
				w.URL = v
			case webhookToken:
				w.Token = v
			default:
				return nil, fmt.Errorf("invalid webhookConfig key: %s", k)
			}
		}
		ws = append(ws, &w)
	}
	return ws, nil
}

// buildSlackConfigs builds slackConfig from slack options
func buildSlackConfigs(slacks []string) ([]*slackConfig, error) {
	var ss []*slackConfig
	for _, slackStr := range slacks {
		m := strToMap(slackStr)
		if len(m) == 0 {
			return nil, fmt.Errorf("invalid slack: %s, slack config should be in the format of api_url=my-api-url,channel=my-channel,username=my-username", slackStr)
		}
		s := slackConfig{}
		for k, v := range m {
			// check slackConfig keys
			switch slackKey(k) {
			case slackAPIURL:
				s.APIURL = v
			case slackChannel:
				s.Channel = v
			case slackUsername:
				s.Username = v
			default:
				return nil, fmt.Errorf("invalid slackConfig key: %s", k)
			}
		}
		ss = append(ss, &s)
	}
	return ss, nil
}

// buildEmailConfigs builds emailConfig from email options
func buildEmailConfigs(emails []string) []*emailConfig {
	var es []*emailConfig
	for _, email := range emails {
		strs := strings.Split(email, ",")
		for _, str := range strs {
			es = append(es, &emailConfig{To: str})
		}
	}
	return es
}

func updateAlertConfig(client kubernetes.Interface, namespace string, data map[string]interface{}) error {
	newValue, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = client.CoreV1().ConfigMaps(namespace).Patch(context.TODO(), alertConfigmapName, apitypes.JSONPatchType,
		[]byte(fmt.Sprintf("[{\"op\": \"replace\", \"path\": \"/data/%s\", \"value\": %s }]",
			alertConfigFileName, strconv.Quote(string(newValue)))), metav1.PatchOptions{})
	return err
}
