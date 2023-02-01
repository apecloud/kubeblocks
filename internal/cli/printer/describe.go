package printer

import (
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/util"
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
func PrintComponentConfigMeta(cfgTplMap map[dbaasv1alpha1.ConfigTemplate]*corev1.ConfigMap, clusterName, componentName string, out io.Writer) {
	if len(cfgTplMap) == 0 {
		return
	}
	tbl := NewTablePrinter(out)
	PrintTitle("Configures Meta")
	tbl.SetHeader("CONFIGURATION-FILE", "CONFIGMAP", "COMPONENT", "CLUSTER", "TEMPLATE-NAME", "CONFIG-TEMPLATE", "CONFIG-CONSTRAINT", "NAMESPACE")
	for tpl, cm := range cfgTplMap {
		for key := range cm.Data {
			tbl.AddRow(key, cm.Name, componentName, clusterName, tpl.Name, tpl.ConfigTplRef, tpl.ConfigConstraintRef, tpl.Namespace)
		}
	}
	tbl.Print()
}
