/*
Copyright ApeCloud Inc.
Copyright 2016 The Kubernetes Authors.

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

package dbaas

import (
	"regexp"
	"strconv"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// ------- copy from stateful_set_utils.go ----
// statefulPodRegex is a regular expression that extracts the parent StatefulSet and ordinal from the Name of a Pod
var statefulPodRegex = regexp.MustCompile("(.*)-([0-9]+)$")

// getParentNameAndOrdinal gets the name of pod's parent StatefulSet and pod's ordinal as extracted from its Name. If
// the Pod was not created by a StatefulSet, its parent is considered to be empty string, and its ordinal is considered
// to be -1.
func getParentNameAndOrdinal(pod *corev1.Pod) (string, int) {
	parent := ""
	ordinal := -1
	subMatches := statefulPodRegex.FindStringSubmatch(pod.Name)
	if len(subMatches) < 3 {
		return parent, ordinal
	}
	parent = subMatches[1]
	if i, err := strconv.ParseInt(subMatches[2], 10, 32); err == nil {
		ordinal = int(i)
	}
	return parent, ordinal
}

// getParentName gets the name of pod's parent StatefulSet. If pod has not parent, the empty string is returned.
func getParentName(pod *corev1.Pod) string {
	parent, _ := getParentNameAndOrdinal(pod)
	return parent
}

// isMemberOf tests if pod is a member of set.
func isMemberOf(set *appsv1.StatefulSet, pod *corev1.Pod) bool {
	return getParentName(pod) == set.Name
}

// getPodRevision gets the revision of Pod by inspecting the StatefulSetRevisionLabel. If pod has no revision the empty
// string is returned.
func getPodRevision(pod *corev1.Pod) string {
	if pod.Labels == nil {
		return ""
	}
	return pod.Labels[appsv1.StatefulSetRevisionLabel]
}

// ------- end copy from stateful_set_utils.go ----

// descendingOrdinalSts is a sort.Interface that Sorts a list of StatefulSet based on the ordinals extracted
// from the StatefulSet.
type descendingOrdinalSts []*appsv1.StatefulSet

var statefulSetRegex = regexp.MustCompile("(.*)-([0-9]+)$")

func (dos descendingOrdinalSts) Len() int {
	return len(dos)
}

func (dos descendingOrdinalSts) Swap(i, j int) {
	dos[i], dos[j] = dos[j], dos[i]
}

func (dos descendingOrdinalSts) Less(i, j int) bool {
	return getOrdinalSts(dos[i]) > getOrdinalSts(dos[j])
}

// getOrdinal gets StatefulSet's ordinal. If StatefulSet has no ordinal, -1 is returned.
func getOrdinalSts(sts *appsv1.StatefulSet) int {
	_, ordinal := getParentNameAndOrdinalSts(sts)
	return ordinal
}

// getParentNameAndOrdinalSts gets the name of cluster-component and StatefulSet's ordinal as extracted from its Name. If
// the StatefulSet's Name was not match a statefulSetRegex, its parent is considered to be empty string, and its ordinal is considered
// to be -1.
func getParentNameAndOrdinalSts(sts *appsv1.StatefulSet) (string, int) {
	parent := ""
	ordinal := -1
	subMatches := statefulSetRegex.FindStringSubmatch(sts.Name)
	if len(subMatches) < 3 {
		return parent, ordinal
	}
	parent = subMatches[1]
	if i, err := strconv.ParseInt(subMatches[2], 10, 32); err == nil {
		ordinal = int(i)
	}
	return parent, ordinal
}

func checkStsIsPrimary(sts *appsv1.StatefulSet) bool {
	return sts.Labels[replicationSetRoleLabelKey] == string(dbaasv1alpha1.Primary)
}
