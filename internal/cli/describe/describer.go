/*
Copyright ApeCloud Inc.

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

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

// Each level has 2 spaces for PrefixWriter
const (
	Level0 = iota
	Level1
	Level2
	Level3
	Level4

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
	*cluster.ClusterObjects
}

func (d *ClusterDescriber) Describe(namespace, name string, describerSettings describe.DescriberSettings) (string, error) {
	var err error
	d.describerSettings = describerSettings
	clusterGetter := cluster.ObjectsGetter{
		ClientSet:      d.client,
		DynamicClient:  d.dynamic,
		Name:           name,
		Namespace:      namespace,
		WithClusterDef: true,
		WithPVC:        true,
		WithService:    true,
		WithSecret:     true,
		WithPod:        true,
	}
	if d.ClusterObjects, err = clusterGetter.Get(); err != nil {
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
		c := d.ClusterObjects.Cluster
		w := describe.NewPrefixWriter(out)
		w.Write(Level0, "Name:\t%s\n", c.Name)
		w.Write(Level0, "Namespace:\t%s\n", c.Namespace)
		w.Write(Level0, "Status:\t%s\n", c.Status.Phase)
		w.Write(Level0, "AppVersion:\t%s\n", c.Spec.AppVersionRef)
		w.Write(Level0, "ClusterDefinition:\t%s\n", c.Spec.ClusterDefRef)
		w.Write(Level0, "TerminationPolicy:\t%s\n", c.Spec.TerminationPolicy)
		w.Write(Level0, "CreationTimestamp:\t%s\n", c.CreationTimestamp.Time.Format(time.RFC1123Z))

		// consider first component as primary component, use it's endpoints as cluster endpoints
		primaryComponent := cluster.FindCompInCluster(d.Cluster, d.ClusterDef.Spec.Components[0].TypeName)
		describeNetwork(Level0, d.Services, primaryComponent, w)

		// topology
		if err := d.describeTopology(w); err != nil {
			return err
		}

		// components
		if err := d.describeComponent(w); err != nil {
			return err
		}

		// describe secret
		describeSecret(d.Secrets, w)

		// describe events
		if events != nil {
			w.Write(Level0, "\n")
			describe.DescribeEvents(events, w)
		}

		return nil
	})
}

func (d *ClusterDescriber) describeTopology(w describe.PrefixWriter) error {
	w.Write(Level0, "\nTopology:\n")
	for _, compInClusterDef := range d.ClusterDef.Spec.Components {
		c := cluster.FindCompInCluster(d.Cluster, compInClusterDef.TypeName)
		if c == nil {
			return fmt.Errorf("failed to find componnet in cluster")
		}

		w.Write(Level1, "%s:\n", c.Name)
		w.Write(Level2, "Type:\t%s\n", c.Type)
		w.Write(Level2, "Instances:\n")

		// describe instance name
		pods := d.getPodsOfComponent(c.Name)
		for _, pod := range pods {
			instance := pod.Name
			if role, ok := pod.Labels[types.ConsensusSetRoleLabelKey]; ok {
				instance = fmt.Sprintf("%s@%s", instance, role)
			}
			w.Write(Level3, "%s\n", instance)
		}
	}
	return nil
}

func (d *ClusterDescriber) describeComponent(w describe.PrefixWriter) error {
	for _, compInClusterDef := range d.ClusterDef.Spec.Components {
		c := cluster.FindCompInCluster(d.Cluster, compInClusterDef.TypeName)
		if c == nil {
			return fmt.Errorf("failed to find component in cluster \"%s\"", d.Cluster.Name)
		}

		if c.Replicas == nil {
			r := compInClusterDef.DefaultReplicas
			c.Replicas = &r
		}
		pods := d.getPodsOfComponent(c.Name)
		if len(pods) == 0 {
			return fmt.Errorf("failed to find any instance belonging to component \"%s\"", c.Name)
		}
		running, waiting, succeeded, failed := util.GetPodStatus(pods)
		w.Write(Level0, "\nComponent:\n")
		w.Write(Level1, "%s\n", c.Name)
		w.Write(Level2, "Type:\t%s\n", c.Type)
		w.Write(Level2, "Replicas:\t%d desired | %d total\n", *c.Replicas, len(pods))
		w.Write(Level2, "Status:\t%d Running / %d Waiting / %d Succeeded / %d Failed\n", running, waiting, succeeded, failed)
		w.Write(Level2, "Image:\t%s\n", pods[0].Spec.Containers[0].Image)

		// CPU and memory
		describeResource(&c.Resources, w)

		// storage
		sc := d.Cluster.Annotations[types.StorageClassAnnotationKey]
		describeStorage(c.VolumeClaimTemplates, sc, w)

		// network
		describeNetwork(Level2, d.Services, c, w)

		// instance
		if len(pods) > 0 {
			w.Write(Level2, "\n")
			w.Write(Level2, "Instance:\t\n")
		}
		for _, pod := range pods {
			d.describeInstance(Level2, pod, w)
		}
	}
	return nil
}

func describeResource(resources *corev1.ResourceRequirements, w describe.PrefixWriter) {
	names := []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory}
	for _, name := range names {
		limit := resources.Limits[name]
		request := resources.Requests[name]
		resName := cases.Title(language.Und, cases.NoLower).String(name.String())

		if util.ResourceIsEmpty(&limit) && util.ResourceIsEmpty(&request) {
			w.Write(Level2, "%s:\t%s\n", resName, valueNone)
		} else {
			w.Write(Level2, "%s:\t%s / %s (request / limit)\n", resName, request.String(), limit.String())
		}
	}
}

func describeStorage(vcTmpls []dbaasv1alpha1.ClusterComponentVolumeClaimTemplate, sc string, w describe.PrefixWriter) {
	if len(vcTmpls) > 0 {
		w.Write(Level2, "Storage:\n")
	}
	for _, vcTmpl := range vcTmpls {
		w.Write(Level3, "%s:\n", vcTmpl.Name)
		val := vcTmpl.Spec.Resources.Requests[corev1.ResourceStorage]
		scName := vcTmpl.Spec.StorageClassName

		switch {
		case scName != nil && len(*scName) > 0:
			w.Write(Level4, "StorageClass:\t%s\n", *vcTmpl.Spec.StorageClassName)
		case sc != "":
			w.Write(Level4, "StorageClass:\t%s\n", sc)
		default:
			w.Write(Level4, "StorageClass:\t%s\n", valueNone)
		}

		w.Write(Level4, "Access Modes:\t%s\n", getAccessModes(vcTmpl.Spec.AccessModes))
		w.Write(Level4, "Size:\t%s\n", val.String())
	}
}

func (d *ClusterDescriber) describeInstance(level int, pod *corev1.Pod, w describe.PrefixWriter) {
	w.Write(level+1, "%s:\n", pod.Name)
	role := pod.Labels[types.ConsensusSetRoleLabelKey]
	if len(role) == 0 {
		role = valueNone
	}
	w.Write(level+2, "Role:\t%s\n", role)

	// status and reason
	if pod.DeletionTimestamp != nil {
		w.Write(level+2, "Status:\tTerminating (lasts %s)\n", translateTimestampSince(*pod.DeletionTimestamp))
		w.Write(level+2, "Termination Grace Period:\t%ds\n", *pod.DeletionGracePeriodSeconds)
	} else {
		w.Write(level+2, "Status:\t%s\n", string(pod.Status.Phase))
	}
	if len(pod.Status.Reason) > 0 {
		w.Write(level+2, "Reason:\t%s\n", pod.Status.Reason)
	}

	accessMode := pod.Labels[types.ConsensusSetAccessModeLabelKey]
	if len(accessMode) == 0 {
		accessMode = valueNone
	}
	w.Write(level+2, "AccessMode:\t%s\n", accessMode)

	// describe node information
	describeNode(level+2, d.Nodes, pod, w)

	w.Write(level+2, "CreationTimestamp:\t%s\n", pod.CreationTimestamp.Time.Format(time.RFC1123Z))
}

// describeNode describe node information include its region and AZ
func describeNode(level int, nodes []*corev1.Node, pod *corev1.Pod, w describe.PrefixWriter) {
	var node *corev1.Node

	if pod.Spec.NodeName == "" {
		w.Write(level, "Node:\t%s\n", valueNone)
	} else {
		w.Write(level, "Node:\t%s\n", pod.Spec.NodeName+"/"+pod.Status.HostIP)
		node = util.GetNodeByName(nodes, pod.Spec.NodeName)
	}

	if node == nil {
		return
	}

	if region, ok := node.Labels[types.RegionLabelKey]; ok {
		w.Write(level, "Region:\t%s\n", region)
	}
	if zone, ok := node.Labels[types.ZoneLabelKey]; ok {
		w.Write(level, "AZ:\t%s\n", zone)
	}
}

func describeSecret(secrets *corev1.SecretList, w describe.PrefixWriter) {
	for _, s := range secrets.Items {
		w.Write(Level0, "\n")
		w.Write(Level0, "Secret:\n")
		w.Write(Level1, "Name:\t%s\n", s.Name)
		w.Write(Level1, "Data:\n")
		for k, v := range s.Data {
			switch {
			case k == corev1.ServiceAccountTokenKey && s.Type == corev1.SecretTypeServiceAccountToken:
				w.Write(Level2, "%s:\t%s\n", k, string(v))
			default:
				w.Write(Level2, "%s:\t%d bytes\n", k, len(v))
			}
		}
	}
}

func describeNetwork(baseLevel int, svcList *corev1.ServiceList, c *dbaasv1alpha1.ClusterComponent, w describe.PrefixWriter) {
	internalEndpoints, externalEndpoints := cluster.GetClusterEndpoints(svcList, c)
	if len(internalEndpoints) == 0 && len(externalEndpoints) == 0 {
		return
	}

	w.Write(baseLevel, "Endpoints:\n")
	w.Write(baseLevel+1, "ReadWrite:\n")

	if len(internalEndpoints) > 0 {
		w.Write(baseLevel+2, fmt.Sprintf("Internal:\t%s\n", strings.Join(internalEndpoints, ",")))
	}

	if len(externalEndpoints) > 0 {
		w.Write(baseLevel+2, fmt.Sprintf("External:\t%s\n", strings.Join(externalEndpoints, ",")))
	}
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
