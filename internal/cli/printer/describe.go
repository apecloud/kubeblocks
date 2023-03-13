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

package printer

import (
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

const NoneString = "<none>"

func PrintAllWarningEvents(events *corev1.EventList, out io.Writer) {
	objs := util.SortEventsByLastTimestamp(events, corev1.EventTypeWarning)
	title := fmt.Sprintf("\n%s Events: ", corev1.EventTypeWarning)
	if objs == nil || len(*objs) == 0 {
		fmt.Fprintln(out, title+NoneString)
		return
	}
	tbl := NewTablePrinter(out)
	fmt.Fprintln(out, title)
	tbl.SetHeader("TIME", "TYPE", "REASON", "OBJECT", "MESSAGE")
	for _, o := range *objs {
		e := o.(*corev1.Event)
		tbl.AddRow(util.GetEventTimeStr(e), e.Type, e.Reason, util.GetEventObject(e), e.Message)
	}
	tbl.Print()

}

// PrintConditions prints the conditions of resource.
func PrintConditions(conditions []metav1.Condition, out io.Writer) {
	// if the conditions are empty, return.
	if len(conditions) == 0 {
		return
	}
	tbl := NewTablePrinter(out)
	PrintTitle("Conditions")
	tbl.SetHeader("LAST-TRANSITION-TIME", "TYPE", "REASON", "STATUS", "MESSAGE")
	for _, con := range conditions {
		tbl.AddRow(util.TimeFormat(&con.LastTransitionTime), con.Type, con.Reason, con.Status, con.Message)
	}
	tbl.Print()
}

// PrintComponentConfigMeta prints the conditions of resource.
func PrintComponentConfigMeta(tplInfos []types.ConfigTemplateInfo, clusterName, componentName string, out io.Writer) {
	if len(tplInfos) == 0 {
		return
	}
	tbl := NewTablePrinter(out)
	PrintTitle("ConfigSpecs Meta")
	enableReconfiguring := func(tpl appsv1alpha1.ComponentConfigSpec, key string) string {
		if len(tpl.ConfigConstraintRef) > 0 && cfgcore.CheckConfigTemplateReconfigureKey(tpl, key) {
			return "true"
		}
		return "false"
	}
	tbl.SetHeader("CONFIG-SPEC-NAME", "FILE", "ENABLED", "TEMPLATE", "CONSTRAINT", "RENDERED", "COMPONENT", "CLUSTER")
	for _, info := range tplInfos {
		for key := range info.CMObj.Data {
			tbl.AddRow(
				BoldYellow(info.Name),
				key,
				BoldYellow(enableReconfiguring(info.TPL, key)),
				info.TPL.ConfigTemplateRef,
				info.TPL.ConfigConstraintRef,
				info.CMObj.Name,
				componentName,
				clusterName)
		}
	}
	tbl.Print()
}
