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

package kubeblocks

import (
	"context"
	"fmt"
	"golang.org/x/exp/maps"
	"sort"
	"strconv"
	"strings"

	"github.com/containerd/stargz-snapshotter/estargz/errorutil"
	"github.com/spf13/cobra"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"

	tablePrinter "github.com/jedib0t/go-pretty/v6/table"
	text "github.com/jedib0t/go-pretty/v6/text"
)

var (
	infoExample = templates.Examples(`
	# list workloads owned by KubeBlocks
	kbcli kubeblocks status

	# list all resources owned by KubeBlocks, such as workloads, cluster definitions, backup template.
	kbcli kubeblocks status --all`)
)

var (
	kubeBlocksWorkloads = []schema.GroupVersionResource{
		types.DeployGVR(),
		types.StatefulSetGVR(),
		types.DaemonSetGVR(),
		types.JobGVR(),
		types.CronJobGVR(),
	}

	kubeBlocksGlobalCustomResources = []schema.GroupVersionResource{
		types.BackupToolGVR(),
		types.ClusterDefGVR(),
		types.ClusterVersionGVR(),
		types.ConfigConstraintGVR(),
	}

	kubeBlocksConfigurations = []schema.GroupVersionResource{
		types.ConfigmapGVR(),
		types.SecretGVR(),
		types.ServiceGVR(),
	}

	kubeBlocksClusterRBAC = []schema.GroupVersionResource{
		types.ClusterRoleGVR(),
		types.ClusterRoleBindingGVR(),
	}

	kubeBlocksNamespacedRBAC = []schema.GroupVersionResource{
		types.RoleGVR(),
		types.RoleBindingGVR(),
		types.ServiceAccountGVR(),
	}

	kubeBlocksStorages = []schema.GroupVersionResource{
		types.PVCGVR(),
	}

	helmConfigurations = []schema.GroupVersionResource{
		types.ConfigmapGVR(),
		types.SecretGVR(),
	}
	notAvailable = "N/A"
)

type statusOptions struct {
	genericclioptions.IOStreams
	client       kubernetes.Interface
	dynamic      dynamic.Interface
	mc           metrics.Interface
	showAll      bool
	ns           string
	addons       []*extensionsv1alpha1.Addon
	selectorList []metav1.ListOptions
}

func newStatusCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := statusOptions{IOStreams: streams}
	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Show list of resource KubeBlocks uses or owns.",
		Args:    cobra.NoArgs,
		Example: infoExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f))
			util.CheckErr(o.run())
		},
	}
	cmd.Flags().BoolVarP(&o.showAll, "all", "A", false, "Show all resources, including configurations, storages, etc")
	return cmd
}

func (o *statusOptions) complete(f cmdutil.Factory) error {
	var err error

	o.dynamic, err = f.DynamicClient()
	if err != nil {
		return err
	}

	o.client, err = f.KubernetesClientSet()
	if err != nil {
		return err
	}

	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	o.mc, err = metrics.NewForConfig(config)
	if err != nil {
		return err
	}

	o.ns, _ = util.GetKubeBlocksNamespace(o.client)
	if o.ns == "" {
		printer.Warning(o.Out, "Failed to find deployed KubeBlocks in any namespace\n")
		printer.Warning(o.Out, "Will check all namespaces for KubeBlocks resources left behind\n")
	}

	o.selectorList = []metav1.ListOptions{
		{LabelSelector: fmt.Sprintf("%s=%s", constant.AppManagedByLabelKey, constant.AppName)}, // app.kubernetes.io/managed-by=kubeblocks
	}

	return nil
}

func (o *statusOptions) run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	allErrs := make([]error, 0)
	o.buildSelectorList(ctx, &allErrs)
	o.showK8sClusterInfos(ctx, &allErrs)
	o.showWorkloads(ctx, &allErrs)
	o.showAddons()

	if o.showAll {
		o.showKubeBlocksResources(ctx, &allErrs)
		o.showKubeBlocksConfig(ctx, &allErrs)
		o.showKubeBlocksRBAC(ctx, &allErrs)
		o.showKubeBlocksStorage(ctx, &allErrs)
		o.showHelmResources(ctx, &allErrs)
	}
	return errorutil.Aggregate(allErrs)
}

func (o *statusOptions) buildSelectorList(ctx context.Context, allErrs *[]error) {
	addons := make([]*extensionsv1alpha1.Addon, 0)
	objs, err := o.dynamic.Resource(types.AddonGVR()).List(ctx, metav1.ListOptions{})
	appendErrIgnoreNotFound(allErrs, err)
	if objs != nil {
		for _, obj := range objs.Items {
			addon := &extensionsv1alpha1.Addon{}
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, addon); err != nil {
				appendErrIgnoreNotFound(allErrs, err)
				continue
			}
			addons = append(addons, addon)
		}
	}
	// build addon instance selector
	o.addons = addons
	for _, selector := range buildResourceLabelSelectors(addons) {
		o.selectorList = append(o.selectorList, metav1.ListOptions{LabelSelector: selector})
	}
}

func (o *statusOptions) showAddons() {
	fmt.Fprintln(o.Out, "\nKubeBlocks Addons:")
	tbl := printer.NewTablePrinter(o.Out)

	tbl.Tbl.SetColumnConfigs([]tablePrinter.ColumnConfig{
		{
			Name: "STATUS",
			Transformer: func(val interface{}) string {
				var ok bool
				var addonPhase extensionsv1alpha1.AddonPhase
				if addonPhase, ok = val.(extensionsv1alpha1.AddonPhase); !ok {
					return fmt.Sprint(val)
				}
				var color text.Color
				switch addonPhase {
				case extensionsv1alpha1.AddonEnabled:
					color = text.FgGreen
				case extensionsv1alpha1.AddonFailed:
					color = text.FgRed
				case extensionsv1alpha1.AddonDisabled:
					color = text.Faint
				case extensionsv1alpha1.AddonEnabling, extensionsv1alpha1.AddonDisabling:
					color = text.FgCyan
				default:
					return fmt.Sprint(addonPhase)
				}
				return color.Sprint(addonPhase)
			},
		},
	},
	)

	tbl.SetHeader("NAME", "STATUS", "TYPE", "PROVIDER")

	var provider string
	var ok bool
	for _, addon := range o.addons {
		if addon.Labels == nil {
			provider = notAvailable
		} else if provider, ok = addon.Labels[constant.AddonProviderLableKey]; !ok {
			provider = notAvailable
		}
		tbl.AddRow(addon.Name, addon.Status.Phase, addon.Spec.Type, provider)
	}
	tbl.Print()
}

func (o *statusOptions) showKubeBlocksResources(ctx context.Context, allErrs *[]error) {
	fmt.Fprintln(o.Out, "\nKubeBlocks Global Custom Resources:")
	tblPrinter := printer.NewTablePrinter(o.Out)
	tblPrinter.SetHeader("KIND", "NAME")

	unstructuredList := listResourceByGVR(ctx, o.dynamic, metav1.NamespaceAll, kubeBlocksGlobalCustomResources, o.selectorList, allErrs)
	for _, resourceList := range unstructuredList {
		for _, resource := range resourceList.Items {
			tblPrinter.AddRow(resource.GetKind(), resource.GetName())
		}
	}
	tblPrinter.Print()
}

func (o *statusOptions) showKubeBlocksConfig(ctx context.Context, allErrs *[]error) {
	fmt.Fprintln(o.Out, "\nKubeBlocks Configurations:")
	tblPrinter := printer.NewTablePrinter(o.Out)
	tblPrinter.SetHeader("NAMESPACE", "KIND", "NAME")
	unstructuredList := listResourceByGVR(ctx, o.dynamic, o.ns, kubeBlocksConfigurations, o.selectorList, allErrs)
	for _, resourceList := range unstructuredList {
		for _, resource := range resourceList.Items {
			tblPrinter.AddRow(resource.GetNamespace(), resource.GetKind(), resource.GetName())
		}
	}
	tblPrinter.Print()
}

func (o *statusOptions) showKubeBlocksRBAC(ctx context.Context, allErrs *[]error) {
	fmt.Fprintln(o.Out, "\nKubeBlocks Global RBAC:")
	tblPrinter := printer.NewTablePrinter(o.Out)
	tblPrinter.SetHeader("KIND", "NAME")
	unstructuredList := listResourceByGVR(ctx, o.dynamic, metav1.NamespaceAll, kubeBlocksClusterRBAC, o.selectorList, allErrs)
	for _, resourceList := range unstructuredList {
		for _, resource := range resourceList.Items {
			tblPrinter.AddRow(resource.GetKind(), resource.GetName())
		}
	}

	tblPrinter.Print()

	fmt.Fprintln(o.Out, "\nKubeBlocks Namespaced RBAC:")
	tblPrinter = printer.NewTablePrinter(o.Out)
	tblPrinter.SetHeader("NAMESPACE", "KIND", "NAME")
	unstructuredList = listResourceByGVR(ctx, o.dynamic, o.ns, kubeBlocksNamespacedRBAC, o.selectorList, allErrs)
	for _, resourceList := range unstructuredList {
		for _, resource := range resourceList.Items {
			tblPrinter.AddRow(resource.GetNamespace(), resource.GetKind(), resource.GetName())
		}
	}

	tblPrinter.Print()
}

func (o *statusOptions) showKubeBlocksStorage(ctx context.Context, allErrs *[]error) {
	fmt.Fprintln(o.Out, "\nKubeBlocks Storage:")
	tblPrinter := printer.NewTablePrinter(o.Out)
	tblPrinter.SetHeader("NAMESPACE", "KIND", "NAME", "CAPACITY")

	renderPVC := func(raw *unstructured.Unstructured) {
		pvc := &corev1.PersistentVolumeClaim{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw.Object, pvc)
		if err != nil {
			appendErrIgnoreNotFound(allErrs, err)
			return
		}
		tblPrinter.AddRow(pvc.GetNamespace(), pvc.Kind, pvc.GetName(), pvc.Status.Capacity.Storage())
	}

	unstructuredList := listResourceByGVR(ctx, o.dynamic, o.ns, kubeBlocksStorages, o.selectorList, allErrs)
	for _, resourceList := range unstructuredList {
		for _, resource := range resourceList.Items {
			switch resource.GetKind() {
			case constant.PersistentVolumeClaimKind:
				renderPVC(&resource)
			default:
				err := fmt.Errorf("unsupported resources: %s", resource.GetKind())
				appendErrIgnoreNotFound(allErrs, err)
			}
		}
	}
	tblPrinter.Print()
}

func (o *statusOptions) showHelmResources(ctx context.Context, allErrs *[]error) {
	fmt.Fprintln(o.Out, "\nHelm Resources:")
	tblPrinter := printer.NewTablePrinter(o.Out)
	tblPrinter.SetHeader("NAMESPACE", "KIND", "NAME", "STATUS")

	helmLabel := func(name []string) string {
		return fmt.Sprintf("%s in (%s),%s=%s", "name", strings.Join(name, ","), "owner", "helm")
	}
	// init helm release list with 'kubeblocks'
	helmReleaseList := []string{types.KubeBlocksChartName}
	// add add one names name = $kubeblocks-addons$
	for _, addon := range o.addons {
		helmReleaseList = append(helmReleaseList, util.BuildAddonReleaseName(addon.Name))
	}
	// label selector 'owner=helm,name in (kubeblocks,kb-addon-mongodb,kb-addon-redis...)'
	selectors := []metav1.ListOptions{{LabelSelector: helmLabel(helmReleaseList)}}
	unstructuredList := listResourceByGVR(ctx, o.dynamic, o.ns, helmConfigurations, selectors, allErrs)
	for _, resourceList := range unstructuredList {
		for _, resource := range resourceList.Items {
			deployedStatus := resource.GetLabels()["status"]
			tblPrinter.AddRow(resource.GetNamespace(), resource.GetKind(), resource.GetName(), deployedStatus)
		}
	}
	tblPrinter.Print()
}

func (o *statusOptions) showWorkloads(ctx context.Context, allErrs *[]error) {
	fmt.Fprintln(o.Out, "\nKubeBlocks Workloads:")
	tblPrinter := printer.NewTablePrinter(o.Out)
	tblPrinter.Tbl.SetColumnConfigs([]tablePrinter.ColumnConfig{
		{
			Name: "READY PODS",
			Transformer: func(val interface{}) (valStr string) {
				var ok bool
				if valStr, ok = val.(string); !ok {
					return fmt.Sprint(val)
				}
				if valStr == notAvailable || len(valStr) == 0 {
					return valStr
				}
				// split string by '/'
				podsInfo := strings.Split(valStr, "/")
				if len(podsInfo) != 2 {
					return valStr
				}
				readyPods, totalPods := int(0), int(0)
				readyPods, _ = strconv.Atoi(podsInfo[0])
				totalPods, _ = strconv.Atoi(podsInfo[1])

				var color text.Color
				if readyPods != totalPods {
					color = text.FgRed
				} else {
					color = text.FgGreen
				}
				return color.Sprint(valStr)
			},
		},
	},
	)

	tblPrinter.SetHeader("NAMESPACE", "KIND", "NAME", "READY PODS", "CPU(cores)", "MEMORY(bytes)", "CREATED-AT")

	unstructuredList := listResourceByGVR(ctx, o.dynamic, o.ns, kubeBlocksWorkloads, o.selectorList, allErrs)

	cpuMap, memMap, readyMap := computeMetricByWorkloads(ctx, o.ns, unstructuredList, o.mc, allErrs)

	for _, workload := range unstructuredList {
		for _, resource := range workload.Items {
			createdAt := resource.GetCreationTimestamp()
			name := resource.GetName()
			row := []interface{}{resource.GetNamespace(), resource.GetKind(), name, readyMap[name], cpuMap[name], memMap[name], util.TimeFormat(&createdAt)}
			tblPrinter.AddRow(row...)
		}
	}
	tblPrinter.Print()
}

func (o *statusOptions) showK8sClusterInfos(ctx context.Context, allErrs *[]error) {
	version, err := util.GetVersionInfo(o.client)
	if err != nil {
		appendErrIgnoreNotFound(allErrs, err)
	}
	if o.ns != "" {
		fmt.Fprintf(o.Out, "KubeBlocks is deployed in namespace: %s", o.ns)
	}
	if version.KubeBlocks != "" {
		fmt.Fprintf(o.Out, ",version: %s\n", version.KubeBlocks)
	} else {
		printer.PrintBlankLine(o.Out)
	}

	provider, err := util.GetK8sProvider(version.Kubernetes, o.client)
	if err != nil {
		*allErrs = append(*allErrs, fmt.Errorf("failed to get kubernetes provider: %v", err))
	}
	if !provider.IsCloud() {
		return
	}
	fmt.Fprintf(o.Out, "\nKubernetes Cluster:")
	printer.PrintBlankLine(o.Out)
	tblPrinter := printer.NewTablePrinter(o.Out)
	tblPrinter.SetHeader("VERSION", "PROVIDER", "REGION", "AVAILABLE ZONES")
	nodesList, err := o.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		appendErrIgnoreNotFound(allErrs, err)
	}
	if nodesList == nil {
		tblPrinter.AddRow(version.Kubernetes, provider, "unknown", "unknown")
		tblPrinter.Print()
		return
	}
	var region string
	availableZones := make(map[string]struct{})
	for _, node := range nodesList.Items {
		labels := node.GetLabels()
		if labels == nil {
			continue
		}
		region = labels[constant.RegionLabelKey]
		availableZones[labels[constant.ZoneLabelKey]] = struct{}{}
	}
	if region == "" {
		tblPrinter.AddRow(version.Kubernetes, provider, "unknown", "unknown")
		tblPrinter.Print()
		return
	}
	allZones := maps.Keys(availableZones)
	sort.Strings(allZones)
	tblPrinter.AddRow(version.Kubernetes, provider, region, strings.Join(allZones, ","))
	tblPrinter.Print()
}

func getNestedSelectorAsString(obj map[string]interface{}, fields ...string) (string, error) {
	val, found, err := unstructured.NestedStringMap(obj, fields...)
	if !found || err != nil {
		return "", fmt.Errorf("failed to get selector for %v, using field %s", obj, fields)
	}
	// convert it to string
	var pair []string
	for k, v := range val {
		pair = append(pair, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(pair, ","), nil
}

func getNestedInt64(obj map[string]interface{}, fields ...string) int64 {
	val, found, err := unstructured.NestedInt64(obj, fields...)
	if !found || err != nil {
		if klog.V(1).Enabled() {
			klog.Errorf("failed to get int64 for %s, using field %s", obj, fields)
		}
	}
	return val
}

func computeMetricByWorkloads(ctx context.Context, ns string, workloads []*unstructured.UnstructuredList, mc metrics.Interface, allErrs *[]error) (cpuMetricMap, memMetricMap, readyMap map[string]string) {
	cpuMetricMap = make(map[string]string)
	memMetricMap = make(map[string]string)
	readyMap = make(map[string]string)

	computeMetrics := func(namespace, name string, matchLabels string) {
		if pods, err := mc.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{LabelSelector: matchLabels}); err != nil {
			if klog.V(1).Enabled() {
				klog.Errorf("faied to get pod metrics for %s/%s, selector: , error: %v", namespace, name, matchLabels, err)
			}
		} else {
			cpuUsage, memUsage := int64(0), int64(0)
			for _, pod := range pods.Items {
				for _, container := range pod.Containers {
					cpuUsage += container.Usage.Cpu().MilliValue()
					memUsage += container.Usage.Memory().Value() / 1024 / 1024
				}
			}
			cpuMetricMap[name] = fmt.Sprintf("%dm", cpuUsage)
			memMetricMap[name] = fmt.Sprintf("%dMi", memUsage)
		}
	}

	computeWorkloadRunningMeta := func(resource *unstructured.Unstructured, getReadyRepilca func() []string, getTotalReplicas func() []string, getSelector func() []string) error {
		name := resource.GetName()

		readyMap[name] = notAvailable
		cpuMetricMap[name] = notAvailable
		memMetricMap[name] = notAvailable

		if getReadyRepilca != nil && getTotalReplicas != nil {
			readyReplicas := getNestedInt64(resource.Object, getReadyRepilca()...)
			replicas := getNestedInt64(resource.Object, getTotalReplicas()...)
			readyMap[name] = fmt.Sprintf("%d/%d", readyReplicas, replicas)
		}

		if getSelector != nil {
			if matchLabels, err := getNestedSelectorAsString(resource.Object, getSelector()...); err != nil {
				return err
			} else {
				computeMetrics(resource.GetNamespace(), name, matchLabels)
			}
		}
		return nil
	}

	readyReplicas := func() []string { return []string{"status", "readyReplicas"} }
	replicas := func() []string { return []string{"status", "replicas"} }
	matchLabels := func() []string { return []string{"spec", "selector", "matchLabels"} }
	daemonReady := func() []string { return []string{"status", "numberReady"} }
	daemonTotal := func() []string { return []string{"status", "desiredNumberScheduled"} }
	jobReady := func() []string { return []string{"status", "succeeded"} }
	jobTotal := func() []string { return []string{"spec", "completions"} }

	for _, workload := range workloads {
		for _, resource := range workload.Items {
			var err error
			switch resource.GetKind() {
			case constant.DeploymentKind, constant.StatefulSetKind:
				err = computeWorkloadRunningMeta(&resource, readyReplicas, replicas, matchLabels)
			case constant.DaemonSetKind:
				err = computeWorkloadRunningMeta(&resource, daemonReady, daemonTotal, matchLabels)
			case constant.JobKind:
				err = computeWorkloadRunningMeta(&resource, jobReady, jobTotal, matchLabels)
			case constant.CronJobKind:
				err = computeWorkloadRunningMeta(&resource, nil, nil, nil)
			default:
				err = fmt.Errorf("unsupported workload kind: %s, name: %s", resource.GetKind(), resource.GetName())
			}
			if err != nil {
				appendErrIgnoreNotFound(allErrs, err)
			}
		}
	}
	return cpuMetricMap, memMetricMap, readyMap
}

func listResourceByGVR(ctx context.Context, client dynamic.Interface, namespace string, gvrlist []schema.GroupVersionResource, selector []metav1.ListOptions, allErrs *[]error) []*unstructured.UnstructuredList {
	unstructuredList := make([]*unstructured.UnstructuredList, 0)
	for _, gvr := range gvrlist {
		for _, labelSelector := range selector {
			klog.V(1).Infof("listResourceByGVR: namespace=%s, gvrlist=%v, selector=%v", namespace, gvr, labelSelector)
			resource, err := client.Resource(gvr).Namespace(namespace).List(ctx, labelSelector)
			if err != nil {
				appendErrIgnoreNotFound(allErrs, err)
				continue
			}
			unstructuredList = append(unstructuredList, resource)
		}
	}
	return unstructuredList
}

func appendErrIgnoreNotFound(allErrs *[]error, err error) {
	if err == nil || apierrors.IsNotFound(err) {
		return
	}
	*allErrs = append(*allErrs, err)
}
