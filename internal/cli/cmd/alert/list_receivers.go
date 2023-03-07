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
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
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
		Short:   "List all alert receivers",
		Example: listReceiversExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f))
			util.CheckErr(o.run())
		},
	}
	return cmd
}

func (o *listReceiversOptions) run() error {
	data, err := getAlertConfigData(o.alterConfigMap)
	if err != nil {
		return err
	}

	receivers := getReceiversFromData(data)
	if len(receivers) == 0 {
		return nil
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
	tbl.SetHeader("NAME", "EMAIL", "WEBHOOK", "SLACK", "CLUSTER", "SEVERITY")
	for _, rec := range receivers {
		recMap := rec.(map[string]interface{})
		name := recMap["name"].(string)
		routeInfo := getRouteInfo(receiverRouteMap[name])
		tbl.AddRow(name, joinConfigs(recMap, "email_configs"),
			joinConfigs(recMap, "webhook_configs"),
			joinConfigs(recMap, "slack_configs"),
			strings.Join(routeInfo[routeMatcherClusterType], ","),
			strings.Join(routeInfo[routeMatcherSeverityType], ","))
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
		matcher := strings.Split(m, t)
		if len(matcher) != 2 {
			return
		}
		infos := strings.Split(matcher[1], routeMatcherOperator)
		if len(infos) != 2 {
			return
		}
		info := removeDuplicateStr(strings.Split(infos[1], "|"))
		routeInfoMap[t] = append(routeInfoMap[t], info...)
	}

	for _, m := range route.Matchers {
		fetchInfo(m, routeMatcherClusterKey)
		fetchInfo(m, routeMatcherSeverityKey)
	}
	return routeInfoMap
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
	case "webhook_configs":
		for _, c := range cfg.([]interface{}) {
			var webhook webhookConfig
			_ = mapstructure.Decode(c, &webhook)
			result = append(result, webhook.string())
		}
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

func removeDuplicateStr(strArray []string) []string {
	var result []string
	for _, s := range strArray {
		if !slices.Contains(result, s) {
			result = append(result, s)
		}
	}
	return result
}
