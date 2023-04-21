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
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	listReceiversExample = templates.Examples(`
		# list all alter receivers
		kbcli alert list-receivers`)
)

type listReceiversOptions struct {
	baseOptions
}

func newListReceiversCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &listReceiversOptions{baseOptions: baseOptions{IOStreams: streams}}
	cmd := &cobra.Command{
		Use:     "list-receivers",
		Short:   "List all alert receivers.",
		Example: listReceiversExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f))
			util.CheckErr(o.run())
		},
	}
	return cmd
}

func (o *listReceiversOptions) run() error {
	data, err := getConfigData(o.alterConfigMap, alertConfigFileName)
	if err != nil {
		return err
	}

	webhookData, err := getConfigData(o.webhookConfigMap, webhookAdaptorFileName)
	if err != nil {
		return err
	}

	receivers := getReceiversFromData(data)
	if len(receivers) == 0 {
		fmt.Fprintf(o.Out, "No receivers found in alertmanager config %s\n", alertConfigmapName)
		return nil
	}
	webhookReceivers := getReceiversFromData(webhookData)
	if len(receivers) == 0 {
		fmt.Fprintf(o.Out, "No receivers found in webhook adaptor config %s\n", webhookAdaptorConfigmapName)
		return nil
	}

	// build receiver webhook map, key is receiver name, value is webhook config that with
	// the real webhook url
	receiverWebhookMap := make(map[string][]webhookConfig)
	for _, r := range receivers {
		var cfgs []webhookConfig
		name := r.(map[string]interface{})["name"].(string)
		for _, w := range webhookReceivers {
			obj := w.(map[string]interface{})
			if obj["name"] == name {
				cfg := webhookConfig{}
				params := obj["params"].(map[string]interface{})
				cfg.URL = params["url"].(string)
				cfgs = append(cfgs, cfg)
			}
		}
		receiverWebhookMap[name] = cfgs
	}

	// build receiver route map, key is receiver name, value is route
	receiverRouteMap := make(map[string]*route)
	routes := getRoutesFromData(data)
	for _, r := range routes {
		res := &route{}
		if err = mapstructure.Decode(r, &res); err != nil {
			return err
		}
		receiverRouteMap[res.Receiver] = res
	}

	tbl := printer.NewTablePrinter(o.Out)
	tbl.SetHeader("NAME", "WEBHOOK", "EMAIL", "SLACK", "CLUSTER", "SEVERITY")
	for _, rec := range receivers {
		recMap := rec.(map[string]interface{})
		name := recMap["name"].(string)
		routeInfo := getRouteInfo(receiverRouteMap[name])
		webhookCfgs := receiverWebhookMap[name]
		tbl.AddRow(name, joinWebhookConfigs(webhookCfgs),
			joinConfigs(recMap, "email_configs"),
			joinConfigs(recMap, "slack_configs"),
			strings.Join(routeInfo[routeMatcherClusterKey], ","),
			strings.Join(routeInfo[routeMatcherSeverityKey], ","))
	}
	tbl.Print()
	return nil
}

// getRouteInfo get route clusters and severity
func getRouteInfo(route *route) map[string][]string {
	routeInfoMap := map[string][]string{
		routeMatcherClusterKey:  {},
		routeMatcherSeverityKey: {},
	}
	if route == nil {
		return routeInfoMap
	}

	fetchInfo := func(m, t string) {
		if !strings.Contains(m, t) {
			return
		}
		matcher := strings.Split(m, routeMatcherOperator)
		if len(matcher) != 2 {
			return
		}
		info := removeDuplicateStr(strings.Split(matcher[1], "|"))
		routeInfoMap[t] = append(routeInfoMap[t], info...)
	}

	for _, m := range route.Matchers {
		fetchInfo(m, routeMatcherClusterKey)
		fetchInfo(m, routeMatcherSeverityKey)
	}
	return routeInfoMap
}

func joinWebhookConfigs(cfgs []webhookConfig) string {
	var result []string
	for _, c := range cfgs {
		result = append(result, c.string())
	}
	return strings.Join(result, "\n")
}

func joinConfigs(rec map[string]interface{}, key string) string {
	var result []string
	if rec == nil {
		return ""
	}

	cfg, ok := rec[key]
	if !ok {
		return ""
	}

	switch key {
	case "slack_configs":
		for _, c := range cfg.([]interface{}) {
			var slack slackConfig
			_ = mapstructure.Decode(c, &slack)
			result = append(result, slack.string())
		}
	case "email_configs":
		for _, c := range cfg.([]interface{}) {
			var email emailConfig
			_ = mapstructure.Decode(c, &email)
			result = append(result, email.string())
		}
	}
	return strings.Join(result, "\n")
}
