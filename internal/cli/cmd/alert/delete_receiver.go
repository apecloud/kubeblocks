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
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	deleteReceiverExample = templates.Examples(`
		# delete a receiver named my-receiver, all receivers can be found by command: kbcli alert list-receivers
		kbcli alert delete-receiver my-receiver`)
)

type deleteReceiverOptions struct {
	baseOptions
	names []string
}

func newDeleteReceiverCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &deleteReceiverOptions{baseOptions: baseOptions{IOStreams: streams}}
	cmd := &cobra.Command{
		Use:     "delete-receiver NAME",
		Short:   "Delete alert receiver.",
		Example: deleteReceiverExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f))
			util.CheckErr(o.validate(args))
			util.CheckErr(o.run())
		},
	}
	return cmd
}

func (o *deleteReceiverOptions) validate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("receiver name is required")
	}
	o.names = args
	return nil
}

func (o *deleteReceiverOptions) run() error {
	// delete receiver from alter manager config
	if err := o.deleteReceiver(); err != nil {
		return err
	}

	// delete receiver from webhook config
	if err := o.deleteWebhookReceivers(); err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "Receiver %s deleted successfully\n", strings.Join(o.names, ","))
	return nil
}

func (o *deleteReceiverOptions) deleteReceiver() error {
	data, err := getConfigData(o.alterConfigMap, alertConfigFileName)
	if err != nil {
		return err
	}

	var newReceivers []interface{}
	var newRoutes []interface{}
	// build receiver route map, key is receiver name, value is route
	receiverRouteMap := make(map[string]interface{})
	routes := getRoutesFromData(data)
	for i, r := range routes {
		name := r.(map[string]interface{})["receiver"].(string)
		receiverRouteMap[name] = routes[i]
	}

	receivers := getReceiversFromData(data)
	for i, rec := range receivers {
		var found bool
		name := rec.(map[string]interface{})["name"].(string)
		for _, n := range o.names {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			newReceivers = append(newReceivers, receivers[i])
			r, ok := receiverRouteMap[name]
			if !ok {
				klog.V(1).Infof("receiver %s not found in routes\n", name)
				continue
			}
			newRoutes = append(newRoutes, r)
		}
	}

	// check if receiver exists
	if len(receivers) == len(newReceivers) {
		return fmt.Errorf("receiver %s not found", strings.Join(o.names, ","))
	}

	data["receivers"] = newReceivers
	data["route"].(map[string]interface{})["routes"] = newRoutes
	return updateConfig(o.client, o.alterConfigMap, alertConfigFileName, data)
}

func (o *deleteReceiverOptions) deleteWebhookReceivers() error {
	data, err := getConfigData(o.webhookConfigMap, webhookAdaptorFileName)
	if err != nil {
		return err
	}
	var newReceivers []interface{}
	receivers := getReceiversFromData(data)
	for i, rec := range receivers {
		var found bool
		name := rec.(map[string]interface{})["name"].(string)
		for _, n := range o.names {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			newReceivers = append(newReceivers, receivers[i])
		}
	}
	data["receivers"] = newReceivers
	return updateConfig(o.client, o.webhookConfigMap, webhookAdaptorFileName, data)
}
