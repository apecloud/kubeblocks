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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

const (
	// alertConfigmapName is the name of alertmanager configmap
	alertConfigmapName = "kubeblocks-prometheus-alertmanager"
)

var (
	addReceiverExample = templates.Examples(`
		# add webhookConfig receiver, for example feishu
		kbcli alert add-receiver --webhookConfig='url=https://open.feishu.cn/open-apis/bot/v2/hook/foo,token=xxxxx'

		# add emailConfig receiver
        kbcli alter add-receiver --emailConfig='a@foo.com,b@foo.com'

		# add emailConfig receiver, and only receive alert from cluster mycluster
		kbcli alter add-receiver --emailConfig='a@foo.com,b@foo.com' --cluster=mycluster

		# add emailConfig receiver, and only receive alert from cluster mycluster and alert severity is warning
		kbcli alter add-receiver --emailConfig='a@foo.com,b@foo.com' --cluster=mycluster --severity=warning

		# add slackConfig receiver
  		kbcli alert add-receiver --slackConfig api_url=https://hooks.slackConfig.com/services/foo,channel=monitor,username=kubeblocks-alert-bot`)
)

type addReceiverOptions struct {
	genericclioptions.IOStreams
	emails     []string
	webhooks   []string
	slacks     []string
	clusters   []string
	severities []string
	name       string

	alterConfigMap *corev1.ConfigMap
	helmCfg        *action.Configuration
}

func newAddReceiverCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := addReceiverOptions{IOStreams: streams}
	cmd := &cobra.Command{
		Use:     "add-receiver",
		Short:   "Add alert receiver, such as emailConfig, slackConfig, webhookConfig and so on",
		Example: addReceiverExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f, cmd))
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

func (o *addReceiverOptions) complete(f cmdutil.Factory, cmd *cobra.Command) error {
	var err error

	client, err := f.KubernetesClientSet()
	if err != nil {
		return err
	}

	o.alterConfigMap, err = getAlertConfigmap(client)
	if err != nil {
		return err
	}

	// build helm config to upgrade alertmanager configmap later
	o.helmCfg, err = buildHelmCfgByCmdFlags(o.alterConfigMap.Namespace, cmd.Flags())
	return err
}

func (o *addReceiverOptions) validate(args []string) error {
	if len(o.emails) == 0 && len(o.webhooks) == 0 && len(o.slacks) == 0 {
		return fmt.Errorf("must specify at least one receiver, such as --emailConfig, --webhookConfig or --slackConfig")
	}

	// if name is not specified, generate a random name
	if len(args) == 0 {
		o.name = generateReceiverName()
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

	return o.addReceiver(receiver, route)
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
	}

	splitStr := func(strArray []string, target *[]string) {
		for _, s := range strArray {
			ss := strings.Split(s, ",")
			*target = append(*target, ss...)
		}
	}

	// parse clusters and severities
	splitStr(o.clusters, &o.clusters)
	splitStr(o.severities, &o.severities)

	// build matchers
	buildMatchers := func(name string, values []string) string {
		if len(values) == 0 {
			return ""
		}
		switch name {
		case "cluster":
			return fmt.Sprintf("app_kubernetes_io_instance=~%s", strings.Join(values, "|"))
		case "severity":
			return fmt.Sprintf("severity=~%s", strings.Join(values, "|"))
		default:
			return ""
		}
	}

	r.Matchers = append(r.Matchers, buildMatchers("cluster", o.clusters),
		buildMatchers("severity", o.severities))
	return r, nil
}

// addReceiver adds receiver to alertmanager config
func (o *addReceiverOptions) addReceiver(receiver *receiver, route *route) error {
	dataStr, ok := o.alterConfigMap.Data["alertmanager.yml"]
	if !ok {
		return fmt.Errorf("alertmanager configmap has no data named alertmanager.yaml")
	}

	// convert string to json
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(dataStr), &data); err != nil {
		return err
	}

	// add receiver
	receivers, ok := data["receivers"]
	if !ok {
		receivers = []interface{}{} // init receivers
	}
	receivers = append(receivers.([]interface{}), receiver)

	// add route
	routes, ok := data["route"].(map[string]interface{})["routes"]
	if !ok {
		routes = []interface{}{} // init routes
	}
	routes = append(routes.([]interface{}), route)

	data["receivers"] = receivers
	data["route"].(map[string]interface{})["routes"] = routes

	// convert struct to json
	newValue, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// update alertmanager configmap
	return updateAlterConfig(o.helmCfg, o.alterConfigMap.Namespace,
		fmt.Sprintf("%s=%s", alertmanagerYmlJSONPath, string(newValue)))
}

// buildWebhookConfigs builds webhookConfig from webhook options
func buildWebhookConfigs(webhooks []string) ([]*webhookConfig, error) {
	var ws []*webhookConfig
	for _, hook := range webhooks {
		m := strToMap(hook)
		for k, v := range m {
			w := webhookConfig{}
			// check webhookConfig keys
			switch webhookKey(k) {
			case webhookURL:
				w.URL = v
			case webhookToken:
				w.Token = v
			default:
				return nil, fmt.Errorf("invalid webhookConfig key: %s", k)
			}
			ws = append(ws, &w)
		}
	}
	return ws, nil
}

// buildSlackConfigs builds slackConfig from slack options
func buildSlackConfigs(slacks []string) ([]*slackConfig, error) {
	var ss []*slackConfig
	for _, slackStr := range slacks {
		m := strToMap(slackStr)
		for k, v := range m {
			s := slackConfig{}
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
			ss = append(ss, &s)
		}
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
