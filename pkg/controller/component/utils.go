/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package component

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func inDataContext() *multicluster.ClientOption {
	return multicluster.InDataContext() // TODO
}

func ValidateDefNameRegexp(defNamePattern string) error {
	_, err := regexp.Compile(defNamePattern)
	return err
}

func PrefixOrRegexMatched(defName, defNamePattern string) bool {
	if strings.HasPrefix(defName, defNamePattern) {
		return true
	}

	isRegexpPattern := func(pattern string) bool {
		escapedPattern := regexp.QuoteMeta(pattern)
		return escapedPattern != pattern
	}

	isRegex := false
	regex, err := regexp.Compile(defNamePattern)
	if err == nil {
		// distinguishing between regular expressions and ordinary strings.
		if isRegexpPattern(defNamePattern) {
			isRegex = true
		}
	}
	if !isRegex {
		return false
	}
	return regex.MatchString(defName)
}

func DefNameMatched(defName, defNamePattern string) bool {
	if defName == defNamePattern {
		return true
	}

	isRegexpPattern := func(pattern string) bool {
		escapedPattern := regexp.QuoteMeta(pattern)
		return escapedPattern != pattern
	}

	isRegex := false
	regex, err := regexp.Compile(defNamePattern)
	if err == nil {
		// distinguishing between regular expressions and ordinary strings.
		if isRegexpPattern(defNamePattern) {
			isRegex = true
		}
	}
	if !isRegex {
		return false
	}
	return regex.MatchString(defName)
}

func IsHostNetworkEnabled(synthesizedComp *SynthesizedComponent) bool {
	if !hasHostNetworkCapability(synthesizedComp, nil) {
		return false
	}
	return hasHostNetworkEnabled(synthesizedComp, nil, synthesizedComp.Annotations, synthesizedComp.Name)
}

func isHostNetworkEnabled(ctx context.Context, cli client.Reader, synthesizedComp *SynthesizedComponent, compName string) (bool, error) {
	// fast path: refer to self
	if compName == synthesizedComp.Name {
		return IsHostNetworkEnabled(synthesizedComp), nil
	}

	// check the component object that whether the host-network is enabled
	compKey := types.NamespacedName{
		Namespace: synthesizedComp.Namespace,
		Name:      constant.GenerateClusterComponentName(synthesizedComp.ClusterName, compName),
	}
	comp := &appsv1.Component{}
	if err := cli.Get(ctx, compKey, comp); err != nil {
		return false, err
	}
	if !hasHostNetworkEnabled(nil, comp, comp.Annotations, compName) {
		return false, nil
	}

	// check the component definition that whether it has the host-network capability
	if len(comp.Spec.CompDef) > 0 {
		compDef := &appsv1.ComponentDefinition{}
		if err := cli.Get(ctx, types.NamespacedName{Name: comp.Spec.CompDef}, compDef); err != nil {
			return false, err
		}
		if hasHostNetworkCapability(nil, compDef) {
			return true, nil
		}
	}
	return false, nil
}

func hasHostNetworkCapability(synthesizedComp *SynthesizedComponent, compDef *appsv1.ComponentDefinition) bool {
	switch {
	case synthesizedComp != nil:
		return synthesizedComp.HostNetwork != nil
	case compDef != nil:
		return compDef.Spec.HostNetwork != nil
	}
	return false
}

func hasHostNetworkEnabled(synthesizedComp *SynthesizedComponent,
	comp *appsv1.Component, annotations map[string]string, compName string) bool {
	if synthesizedComp != nil && synthesizedComp.PodSpec.HostNetwork {
		return true
	}
	if comp != nil && comp.Spec.Network != nil && comp.Spec.Network.HostNetwork {
		return true
	}
	if annotations == nil {
		return false
	}
	comps, ok := annotations[constant.HostNetworkAnnotationKey]
	if !ok {
		return false
	}
	return slices.Index(strings.Split(comps, ","), compName) >= 0
}

func getHostNetworkPort(ctx context.Context, _ client.Reader, clusterName, compName, cName, pName string) (int32, error) {
	key := intctrlutil.BuildHostPortName(clusterName, compName, cName, pName)
	if v, ok := ctx.Value(mockHostNetworkPortManagerKey{}).(map[string]int32); ok {
		if p, okk := v[key]; okk {
			return p, nil
		}
		return 0, nil
	}
	pm := intctrlutil.GetPortManager()
	if pm == nil {
		return 0, nil
	}
	return pm.GetPort(key)
}

func mockHostNetworkPort(ctx context.Context, _ client.Reader, clusterName, compName, cName, pName string, port int32) context.Context {
	key := intctrlutil.BuildHostPortName(clusterName, compName, cName, pName)
	mockHostNetworkPortManager[key] = port
	return context.WithValue(ctx, mockHostNetworkPortManagerKey{}, mockHostNetworkPortManager)
}

var (
	mockHostNetworkPortManager = map[string]int32{}
)

type mockHostNetworkPortManagerKey struct{}

func UDFReconfigureActionName(tpl SynthesizedFileTemplate) string {
	return fmt.Sprintf("reconfigure-%s", tpl.Name)
}

func ConfigTemplates(synthesizedComp *SynthesizedComponent) []appsv1.ComponentFileTemplate {
	if synthesizedComp.FileTemplates == nil {
		return nil
	}
	templates := make([]appsv1.ComponentFileTemplate, 0)
	for i, tpl := range synthesizedComp.FileTemplates {
		if tpl.Config {
			templates = append(templates, synthesizedComp.FileTemplates[i].ComponentFileTemplate)
		}
	}
	return templates
}

func AddInstanceAssistantObject(synthesizedComp *SynthesizedComponent, object client.Object) {
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: synthesizedComp.Annotations,
		},
		Spec: workloads.InstanceSetSpec{
			EnableInstanceAPI: synthesizedComp.EnableInstanceAPI,
		},
	}
	if instanceAPINAssistantObjectsEnabled(its) {
		synthesizedComp.InstanceAssistantObjects = addInstanceAssistantObjects(synthesizedComp.InstanceAssistantObjects, object)
	}
}

func AddInstanceAssistantObjectsToITS(its *workloads.InstanceSet, objs ...client.Object) {
	if instanceAPINAssistantObjectsEnabled(its) {
		its.Spec.InstanceAssistantObjects = addInstanceAssistantObjects(its.Spec.InstanceAssistantObjects, objs...)
	}
}

func instanceAPINAssistantObjectsEnabled(its *workloads.InstanceSet) bool {
	if !ptr.Deref(its.Spec.EnableInstanceAPI, false) {
		return false
	}
	return multicluster.Enabled4Object(its)
}

func addInstanceAssistantObjects(objectRefs []corev1.ObjectReference, objs ...client.Object) []corev1.ObjectReference {
	if objectRefs == nil {
		objectRefs = []corev1.ObjectReference{}
	}
	for _, obj := range objs {
		if obj != nil && !reflect.ValueOf(obj).IsNil() {
			gvk, _ := model.GetGVKName(obj)
			objectRefs = append(objectRefs, corev1.ObjectReference{
				Kind:      gvk.Kind,
				Namespace: gvk.Namespace,
				Name:      gvk.Name,
			})
		}
	}
	slices.SortFunc(objectRefs, func(a, b corev1.ObjectReference) int {
		if a.Kind != b.Kind {
			return strings.Compare(a.Kind, b.Kind)
		}
		if a.Namespace != b.Namespace {
			return strings.Compare(a.Namespace, b.Namespace)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return objectRefs
}
