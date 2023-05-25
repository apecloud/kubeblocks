/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
				info.TPL.TemplateRef,
				info.TPL.ConfigConstraintRef,
				info.CMObj.Name,
				componentName,
				clusterName)
		}
	}
	tbl.Print()
}
