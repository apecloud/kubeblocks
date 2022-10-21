/*
Copyright 2022 The KubeBlocks Authors

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

package describe

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/describe"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/cluster"
)

// Each level has 2 spaces for PrefixWriter
const (
	LEVEL_0 = iota
	LEVEL_1
	LEVEL_2
	LEVEL_3
	LEVEL_4
	LEVEL_5

	valueNone = "<none>"
)

var (
	// DescriberFn gives a way to easily override the function for unit testing if needed
	DescriberFn describe.DescriberFunc = Describer
)

// Describer returns a Describer for displaying the specified RESTMapping type or an error.
func Describer(restClientGetter genericclioptions.RESTClientGetter, mapping *meta.RESTMapping) (describe.ResourceDescriber, error) {
	clientConfig, err := restClientGetter.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	// try to get a describer
	if describer, ok := DescriberFor(mapping.GroupVersionKind.GroupKind(), clientConfig); ok {
		return describer, nil
	}
	// if this is a kind we don't have a describer for yet, go generic if possible
	if genericDescriber, ok := describe.GenericDescriberFor(mapping, clientConfig); ok {
		return genericDescriber, nil
	}
	// otherwise return an unregistered error
	return nil, fmt.Errorf("no description has been implemented for %s", mapping.GroupVersionKind.String())
}

func describerMap(clientConfig *rest.Config) (map[schema.GroupKind]describe.ResourceDescriber, error) {
	c, err := clientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	// used to fetch the resource
	dc, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	m := map[schema.GroupKind]describe.ResourceDescriber{
		types.ClusterGK(): &ClusterDescriber{client: c, dynamic: dc},
	}

	return m, nil
}

// DescriberFor returns the default describe functions for each of the standard
// Kubernetes types.
func DescriberFor(kind schema.GroupKind, clientConfig *rest.Config) (describe.ResourceDescriber, bool) {
	describers, err := describerMap(clientConfig)
	if err != nil {
		klog.V(1).Info(err)
		return nil, false
	}

	f, ok := describers[kind]
	return f, ok
}

// ClusterDescriber generates information about a cluster.
type ClusterDescriber struct {
	client  clientset.Interface
	dynamic dynamic.Interface

	describerSettings describe.DescriberSettings
	*types.ClusterObjects
}

func (d *ClusterDescriber) Describe(namespace, name string, describerSettings describe.DescriberSettings) (string, error) {
	var err error

	d.describerSettings = describerSettings
	d.ClusterObjects = cluster.NewClusterObjects()

	if err = cluster.GetAllObjects(d.client, d.dynamic, namespace, name, d.ClusterObjects); err != nil {
		return "", err
	}

	var events *corev1.EventList
	if describerSettings.ShowEvents {
		events, err = d.client.CoreV1().Events(namespace).Search(scheme.Scheme, d.ClusterObjects.Cluster)
		if err != nil {
			return "", err
		}
	}

	return d.describeCluster(events)
}

func (d *ClusterDescriber) describeCluster(events *corev1.EventList) (string, error) {
	return tabbedString(func(out io.Writer) error {
		cluster := d.ClusterObjects.Cluster
		w := describe.NewPrefixWriter(out)
		w.Write(LEVEL_0, "Name:\t%s\n", cluster.Name)
		w.Write(LEVEL_0, "Namespace:\t%s\n", cluster.Namespace)
		w.Write(LEVEL_0, "Status:\t%s\n", cluster.Status.Phase)
		w.Write(LEVEL_0, "AppVersion:\t%s\n", cluster.Spec.AppVersionRef)
		w.Write(LEVEL_0, "ClusterDefinition:\t%s\n", cluster.Spec.ClusterDefRef)
		w.Write(LEVEL_0, "TerminationPolicy:\t%s\n", cluster.Spec.TerminationPolicy)
		w.Write(LEVEL_0, "CreationTimestamp:\t%s\n", cluster.CreationTimestamp.Time.Format(time.RFC1123Z))

		// topology
		if err := d.showTopology(w); err != nil {
			return err
		}

		// components
		if err := d.showComponent(w); err != nil {
			return err
		}

		// describe secret
		d.showSecret(w)

		// describe events
		if events != nil {
			describe.DescribeEvents(events, w)
		}

		return nil
	})
}

func (d *ClusterDescriber) showTopology(w describe.PrefixWriter) error {
	w.Write(LEVEL_0, "\nTopology:\n")
	for _, compInClusterDef := range d.ClusterDef.Spec.Components {
		c := findCompInCluster(d.Cluster, compInClusterDef.TypeName)
		if c == nil {
			return fmt.Errorf("failed to find componnet in cluster")
		}
		w.Write(LEVEL_1, "%s:\n", c.Name)
		w.Write(LEVEL_2, "Type:\t%s\n", c.Type)
		w.Write(LEVEL_2, "Instances:\t%s\n", c.Type)

		// describe instance name
		pods := d.getPodsOfComponent(c.Name)
		for _, pod := range pods {
			w.Write(LEVEL_3, "%s@%s\n", pod.Name, pod.Labels[types.ConsensusSetRoleLabelKey])
		}
	}
	return nil
}

func (d *ClusterDescriber) showComponent(w describe.PrefixWriter) error {
	for _, compInClusterDef := range d.ClusterDef.Spec.Components {
		c := findCompInCluster(d.Cluster, compInClusterDef.TypeName)
		if c == nil {
			return fmt.Errorf("failed to find componnet in cluster \"%s\"", d.Cluster.Name)
		}

		replicas := c.Replicas
		if replicas == 0 {
			replicas = compInClusterDef.DefaultReplicas
		}

		pods := d.getPodsOfComponent(c.Name)
		if len(pods) == 0 {
			return fmt.Errorf("failed to find any instance belonging to component \"%s\"", c.Name)
		}
		running, waiting, succeeded, failed := getPodStatus(pods)
		w.Write(LEVEL_0, "\nComponent:\n")
		w.Write(LEVEL_1, "Type:\t%s\n", c.Type)
		w.Write(LEVEL_1, "Replicas:\t%d desired | %d total\n", replicas, len(pods))
		w.Write(LEVEL_1, "Status:\t%d Running / %d Waiting / %d Succeeded / %d Failed\n", running, waiting, succeeded, failed)
		w.Write(LEVEL_1, "Image:\t%s\n", pods[0].Spec.Containers[0].Image)

		// cpu and memory
		describeResource(c.Resources, w)

		// storage
		describeStorage(c.VolumeClaimTemplates, w)

		// show instance
		for _, pod := range pods {
			d.showInstance(pod, w)
		}
	}
	return nil
}

func describeResource(resources corev1.ResourceRequirements, w describe.PrefixWriter) {
	if len(resources.Limits) > 0 {
		w.Write(LEVEL_1, "Limits:\n")
	}
	for _, name := range describe.SortedResourceNames(resources.Limits) {
		quantity := resources.Limits[name]
		w.Write(LEVEL_2, "%s:\t%s\n", name, quantity.String())
	}

	if len(resources.Requests) > 0 {
		w.Write(LEVEL_1, "Requests:\n")
	}
	for _, name := range describe.SortedResourceNames(resources.Requests) {
		quantity := resources.Requests[name]
		w.Write(LEVEL_2, "%s:\t%s\n", name, quantity.String())
	}
}

func describeStorage(vcTmpls []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate, w describe.PrefixWriter) {
	if len(vcTmpls) > 0 {
		w.Write(LEVEL_1, "Storage:\n")
	}
	for _, vcTmpl := range vcTmpls {
		w.Write(LEVEL_2, "%s:\n", vcTmpl.Name)
		val := vcTmpl.Spec.Resources.Requests[corev1.ResourceStorage]
		if vcTmpl.Spec.StorageClassName == nil {
			w.Write(LEVEL_3, "StorageClass:\t%s\n", valueNone)
		} else {
			w.Write(LEVEL_3, "StorageClass:\t%s\n", *vcTmpl.Spec.StorageClassName)
		}
		w.Write(LEVEL_3, "Access Modes:\t%s\n", getAccessModes(vcTmpl.Spec.AccessModes))
		w.Write(LEVEL_3, "Size:\t%s\n", val.String())
	}
}

func (d *ClusterDescriber) showInstance(pod *corev1.Pod, w describe.PrefixWriter) {
	w.Write(LEVEL_1, "\n")
	w.Write(LEVEL_1, "Instance:\t\n")
	w.Write(LEVEL_2, "%s:\n", pod.Name)
	w.Write(LEVEL_3, "Role:\t%s\n", pod.Labels[types.ConsensusSetRoleLabelKey])

	// status and reason
	if pod.DeletionTimestamp != nil {
		w.Write(LEVEL_3, "Status:\tTerminating (lasts %s)\n", translateTimestampSince(*pod.DeletionTimestamp))
		w.Write(LEVEL_3, "Termination Grace Period:\t%ds\n", *pod.DeletionGracePeriodSeconds)
	} else {
		w.Write(LEVEL_3, "Status:\t%s\n", string(pod.Status.Phase))
	}
	if len(pod.Status.Reason) > 0 {
		w.Write(LEVEL_3, "Reason:\t%s\n", pod.Status.Reason)
	}

	// TODO: get AccessMode from label
	w.Write(LEVEL_3, "AccessMode:\t%s\n", "")

	// node information include its region and AZ
	if pod.Spec.NodeName == "" {
		w.Write(LEVEL_3, "Node:\t%s\n", valueNone)
	} else {
		w.Write(LEVEL_3, "Node:\t%s\n", pod.Spec.NodeName+"/"+pod.Status.HostIP)
		node := d.getNodeByName(pod.Spec.NodeName)
		if region, ok := node.Labels[types.RegionLabelKey]; ok {
			w.Write(LEVEL_3, "Region:\t%s\n", region)
		}
		if zone, ok := node.Labels[types.ZoneLabelKey]; ok {
			w.Write(LEVEL_3, "AZ:\t%s\n", zone)
		}
	}

	w.Write(LEVEL_3, "CreationTimestamp:\t%s\n", pod.CreationTimestamp.Time.Format(time.RFC1123Z))
}

func (d *ClusterDescriber) showSecret(w describe.PrefixWriter) {
	for i := range d.Secrets.Items {
		describeSecret(&d.Secrets.Items[i], w)
	}
}

func findCompInCluster(cluster *dbaasv1alpha1.Cluster, typeName string) *dbaasv1alpha1.ClusterComponent {
	for i, c := range cluster.Spec.Components {
		if c.Type == typeName {
			return &cluster.Spec.Components[i]
		}
	}
	return nil
}

func (d *ClusterDescriber) getPodsOfComponent(name string) []*corev1.Pod {
	var pods []*corev1.Pod
	for i, p := range d.Pods.Items {
		if n, ok := p.Labels[types.ComponentLabelKey]; ok && n == name {
			pods = append(pods, &d.Pods.Items[i])
		}
	}
	return pods
}

func (d *ClusterDescriber) getNodeByName(name string) *corev1.Node {
	for _, node := range d.Nodes {
		if node.Name == name {
			return node
		}
	}
	return nil
}

func describeSecret(secret *corev1.Secret, w describe.PrefixWriter) {
	w.Write(LEVEL_0, "\n")
	w.Write(LEVEL_0, "Secret:\n")
	w.Write(LEVEL_1, "Name:\t%s\n", secret.Name)
	w.Write(LEVEL_1, "Data:\n")
	for k, v := range secret.Data {
		switch {
		case k == corev1.ServiceAccountTokenKey && secret.Type == corev1.SecretTypeServiceAccountToken:
			w.Write(LEVEL_2, "%s:\t%s\n", k, string(v))
		default:
			w.Write(LEVEL_2, "%s:\t%d bytes\n", k, len(v))
		}
	}
}

func tabbedString(f func(io.Writer) error) (string, error) {
	out := new(tabwriter.Writer)
	buf := &bytes.Buffer{}
	out.Init(buf, 0, 8, 2, ' ', 0)

	err := f(out)
	if err != nil {
		return "", err
	}

	out.Flush()
	str := buf.String()
	return str, nil
}

func getPodStatus(pods []*corev1.Pod) (running, waiting, succeeded, failed int) {
	for _, pod := range pods {
		switch pod.Status.Phase {
		case corev1.PodRunning:
			running++
		case corev1.PodPending:
			waiting++
		case corev1.PodSucceeded:
			succeeded++
		case corev1.PodFailed:
			failed++
		}
	}
	return
}

func getAccessModes(modes []corev1.PersistentVolumeAccessMode) string {
	modes = removeDuplicateAccessModes(modes)
	var modesStr []string
	if containsAccessMode(modes, corev1.ReadWriteOnce) {
		modesStr = append(modesStr, "RWO")
	}
	if containsAccessMode(modes, corev1.ReadOnlyMany) {
		modesStr = append(modesStr, "ROX")
	}
	if containsAccessMode(modes, corev1.ReadWriteMany) {
		modesStr = append(modesStr, "RWX")
	}
	return strings.Join(modesStr, ",")
}

func removeDuplicateAccessModes(modes []corev1.PersistentVolumeAccessMode) []corev1.PersistentVolumeAccessMode {
	var accessModes []corev1.PersistentVolumeAccessMode
	for _, m := range modes {
		if !containsAccessMode(accessModes, m) {
			accessModes = append(accessModes, m)
		}
	}
	return accessModes
}

func containsAccessMode(modes []corev1.PersistentVolumeAccessMode, mode corev1.PersistentVolumeAccessMode) bool {
	for _, m := range modes {
		if m == mode {
			return true
		}
	}
	return false
}

// translateTimestampSince returns the elapsed time since timestamp in
// human-readable approximation.
func translateTimestampSince(timestamp metav1.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}

	return duration.HumanDuration(time.Since(timestamp.Time))
}
