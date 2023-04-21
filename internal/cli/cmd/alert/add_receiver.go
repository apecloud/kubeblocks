/*
Copyright (C) 2022 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package alert

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/yaml"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	// alertConfigmapName is the name of alertmanager configmap
	alertConfigmapName = getConfigMapName(alertManagerAddonName)

	// webhookAdaptorConfigmapName is the name of webhook adaptor
	webhookAdaptorConfigmapName = getConfigMapName(webhookAdaptorAddonName)
)

var (
	addReceiverExample = templates.Examples(`
		# add webhook receiver without token, for example feishu
		kbcli alert add-receiver --webhook='url=https://open.feishu.cn/open-apis/bot/v2/hook/foo'

		# add webhook receiver with token, for example feishu
		kbcli alert add-receiver --webhook='url=https://open.feishu.cn/open-apis/bot/v2/hook/foo,token=XXX'

		# add email receiver
        kbcli alter add-receiver --email='a@foo.com,b@foo.com'

		# add email receiver, and only receive alert from cluster mycluster
		kbcli alter add-receiver --email='a@foo.com,b@foo.com' --cluster=mycluster

		# add email receiver, and only receive alert from cluster mycluster and alert severity is warning
		kbcli alter add-receiver --email='a@foo.com,b@foo.com' --cluster=mycluster --severity=warning

		# add slack receiver
  		kbcli alert add-receiver --slack api_url=https://hooks.slackConfig.com/services/foo,channel=monitor,username=kubeblocks-alert-bot`)
)

type baseOptions struct {
	genericclioptions.IOStreams
	alterConfigMap   *corev1.ConfigMap
	webhookConfigMap *corev1.ConfigMap
	client           kubernetes.Interface
}

type addReceiverOptions struct {
	baseOptions

	emails     []string
	webhooks   []string
	slacks     []string
	clusters   []string
	severities []string
	name       string

	receiver                *receiver
	route                   *route
	webhookAdaptorReceivers []webhookAdaptorReceiver
}

func newAddReceiverCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := addReceiverOptions{baseOptions: baseOptions{IOStreams: streams}}
	cmd := &cobra.Command{
		Use:     "add-receiver",
		Short:   "Add alert receiver, such as email, slack, webhook and so on.",
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
	cmd.Flags().StringArrayVar(&o.severities, "severity", []string{}, "Alert severity, critical, warning or info, more than one severity can be specified, such as critical,warning")

	// register completions
	util.CheckErr(cmd.RegisterFlagCompletionFunc("severity",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return severities(), cobra.ShellCompDirectiveNoFileComp
		}))

	return cmd
}

func (o *baseOptions) complete(f cmdutil.Factory) error {
	var err error
	ctx := context.Background()

	o.client, err = f.KubernetesClientSet()
	if err != nil {
		return err
	}

	namespace, err := util.GetKubeBlocksNamespace(o.client)
	if err != nil {
		return err
	}

	// get alertmanager configmap
	o.alterConfigMap, err = o.client.CoreV1().ConfigMaps(namespace).Get(ctx, alertConfigmapName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// get webhook adaptor configmap
	o.webhookConfigMap, err = o.client.CoreV1().ConfigMaps(namespace).Get(ctx, webhookAdaptorConfigmapName, metav1.GetOptions{})
	return err
}

func (o *addReceiverOptions) validate(args []string) error {
	if len(o.emails) == 0 && len(o.webhooks) == 0 && len(o.slacks) == 0 {
		return fmt.Errorf("must specify at least one receiver, such as --email, --webhook or --slack")
	}

	// if name is not specified, generate a random name
	if len(args) == 0 {
		o.name = generateReceiverName()
	} else {
		o.name = args[0]
	}

	if err := o.checkEmails(); err != nil {
		return err
	}

	if err := o.checkSeverities(); err != nil {
		return err
	}
	return nil
}

// checkSeverities check if severity is valid
func (o *addReceiverOptions) checkSeverities() error {
	if len(o.severities) == 0 {
		return nil
	}
	checkSeverity := func(severity string) error {
		ss := strings.Split(severity, ",")
		for _, s := range ss {
			if !slices.Contains(severities(), strings.ToLower(strings.TrimSpace(s))) {
				return fmt.Errorf("invalid severity: %s, must be one of %v", s, severities())
			}
		}
		return nil
	}

	for _, severity := range o.severities {
		if err := checkSeverity(severity); err != nil {
			return err
		}
	}
	return nil
}

// checkEmails check if email SMTP is configured, if not, do not allow to add email receiver
func (o *addReceiverOptions) checkEmails() error {
	if len(o.emails) == 0 {
		return nil
	}

	errMsg := "SMTP %sis not configured, if you want to add email receiver, please configure it first"
	data, err := getConfigData(o.alterConfigMap, alertConfigFileName)
	if err != nil {
		return err
	}

	if data["global"] == nil {
		return fmt.Errorf(errMsg, "")
	}

	// check smtp config in global
	checkKeys := []string{"smtp_from", "smtp_smarthost", "smtp_auth_username", "smtp_auth_password"}
	checkSMTP := func(key string) error {
		val := data["global"].(map[string]interface{})[key]
		if val == nil || fmt.Sprintf("%v", val) == "" {
			return fmt.Errorf(errMsg, key+" ")
		}
		return nil
	}

	for _, key := range checkKeys {
		if err = checkSMTP(key); err != nil {
			return err
		}
	}
	return nil
}

func (o *addReceiverOptions) run() error {
	// build receiver
	if err := o.buildReceiver(); err != nil {
		return err
	}

	// build route
	o.buildRoute()

	// add alertmanager receiver and route
	if err := o.addReceiver(); err != nil {
		return err
	}

	// add webhook receiver
	if err := o.addWebhookReceivers(); err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "Receiver %s added successfully.\n", o.receiver.Name)
	return nil
}

// buildReceiver builds receiver from receiver options
func (o *addReceiverOptions) buildReceiver() error {
	webhookConfigs, err := o.buildWebhook()
	if err != nil {
		return err
	}

	slackConfigs, err := buildSlackConfigs(o.slacks)
	if err != nil {
		return err
	}

	o.receiver = &receiver{
		Name:           o.name,
		EmailConfigs:   buildEmailConfigs(o.emails),
		WebhookConfigs: webhookConfigs,
		SlackConfigs:   slackConfigs,
	}
	return nil
}

func (o *addReceiverOptions) buildRoute() {
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
		deValues := removeDuplicateStr(values)
		switch t {
		case routeMatcherClusterType:
			return routeMatcherClusterKey + routeMatcherOperator + strings.Join(deValues, "|")
		case routeMatcherSeverityType:
			return routeMatcherSeverityKey + routeMatcherOperator + strings.Join(deValues, "|")
		default:
			return ""
		}
	}

	r.Matchers = append(r.Matchers, buildMatchers(routeMatcherClusterType, clusterArray),
		buildMatchers(routeMatcherSeverityType, severityArray))
	o.route = r
}

// addReceiver adds receiver to alertmanager config
func (o *addReceiverOptions) addReceiver() error {
	data, err := getConfigData(o.alterConfigMap, alertConfigFileName)
	if err != nil {
		return err
	}

	// add receiver
	receivers := getReceiversFromData(data)
	if receiverExists(receivers, o.name) {
		return fmt.Errorf("receiver %s already exists", o.receiver.Name)
	}
	receivers = append(receivers, o.receiver)

	// add route
	routes := getRoutesFromData(data)
	routes = append(routes, o.route)

	data["receivers"] = receivers
	data["route"].(map[string]interface{})["routes"] = routes

	// update alertmanager configmap
	return updateConfig(o.client, o.alterConfigMap, alertConfigFileName, data)
}

func (o *addReceiverOptions) addWebhookReceivers() error {
	data, err := getConfigData(o.webhookConfigMap, webhookAdaptorFileName)
	if err != nil {
		return err
	}

	receivers := getReceiversFromData(data)
	for _, r := range o.webhookAdaptorReceivers {
		receivers = append(receivers, r)
	}
	data["receivers"] = receivers

	// update webhook configmap
	return updateConfig(o.client, o.webhookConfigMap, webhookAdaptorFileName, data)
}

// buildWebhook builds webhookConfig and webhookAdaptorReceiver from webhook options
func (o *addReceiverOptions) buildWebhook() ([]*webhookConfig, error) {
	var ws []*webhookConfig
	var waReceivers []webhookAdaptorReceiver
	for _, hook := range o.webhooks {
		m := strToMap(hook)
		if len(m) == 0 {
			return nil, fmt.Errorf("invalid webhook: %s, webhook should be in the format of url=my-url,token=my-token", hook)
		}
		w := webhookConfig{
			MaxAlerts:    10,
			SendResolved: false,
		}
		waReceiver := webhookAdaptorReceiver{Name: o.name}
		for k, v := range m {
			// check webhookConfig keys
			switch webhookKey(k) {
			case webhookURL:
				if valid, err := urlIsValid(v); !valid {
					return nil, fmt.Errorf("invalid webhook url: %s, %v", v, err)
				}
				w.URL = getWebhookAdaptorURL(o.name, o.webhookConfigMap.Namespace)
				webhookType := getWebhookType(v)
				if webhookType == unknownWebhookType {
					return nil, fmt.Errorf("invalid webhook url: %s, failed to prase the webhook type", v)
				}
				waReceiver.Type = string(webhookType)
				waReceiver.Params.URL = v
			case webhookToken:
				waReceiver.Params.Secret = v
			default:
				return nil, fmt.Errorf("invalid webhook key: %s, webhook key should be one of url and token", k)
			}
		}
		ws = append(ws, &w)
		waReceivers = append(waReceivers, waReceiver)
	}
	o.webhookAdaptorReceivers = waReceivers
	return ws, nil
}

func receiverExists(receivers []interface{}, name string) bool {
	for _, r := range receivers {
		n := r.(map[string]interface{})["name"]
		if n != nil && n.(string) == name {
			return true
		}
	}
	return false
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
				if valid, err := urlIsValid(v); !valid {
					return nil, fmt.Errorf("invalid slack api_url: %s, %v", v, err)
				}
				s.APIURL = v
			case slackChannel:
				s.Channel = "#" + v
			case slackUsername:
				s.Username = v
			default:
				return nil, fmt.Errorf("invalid slack config key: %s", k)
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

func updateConfig(client kubernetes.Interface, cm *corev1.ConfigMap, key string, data map[string]interface{}) error {
	newValue, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	_, err = client.CoreV1().ConfigMaps(cm.Namespace).Patch(context.TODO(), cm.Name, apitypes.JSONPatchType,
		[]byte(fmt.Sprintf("[{\"op\": \"replace\", \"path\": \"/data/%s\", \"value\": %s }]",
			key, strconv.Quote(string(newValue)))), metav1.PatchOptions{})
	return err
}
