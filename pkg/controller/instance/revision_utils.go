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

package instance

import (
	"encoding/json"
	"fmt"
	"hash"
	"hash/fnv"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/dump"
	"k8s.io/apimachinery/pkg/util/rand"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

const (
	// controllerRevisionHashLabel is the label used to indicate the hash value of a ControllerRevision's Data.
	controllerRevisionHashLabel = "controller.kubernetes.io/hash"
)

var (
	codecs         = serializer.NewCodecFactory(model.GetScheme())
	patchCodec     = codecs.LegacyCodec(workloads.SchemeGroupVersion)
	controllerKind = appsv1.SchemeGroupVersion.WithKind("Instance")
)

func newRevision(inst *workloads.Instance) (*appsv1.ControllerRevision, error) {
	patch, err := getPatch(inst)
	if err != nil {
		return nil, err
	}
	collision := int32(0)
	cr, err := newControllerRevision(inst,
		controllerKind,
		inst.Spec.Template.Labels,
		runtime.RawExtension{Raw: patch},
		1,
		&collision)
	if err != nil {
		return nil, err
	}
	if cr.ObjectMeta.Annotations == nil {
		cr.ObjectMeta.Annotations = make(map[string]string)
	}
	for key, value := range inst.Annotations {
		cr.ObjectMeta.Annotations[key] = value
	}
	return cr, nil
}

// getPatch returns a strategic merge patch that can be applied to restore a StatefulSet to a
// previous version. If the returned error is nil the patch is valid. The current state that we save is just the
// PodSpecTemplate. We can modify this later to encompass more state (or less) and remain compatible with previously
// recorded patches.
func getPatch(inst *workloads.Instance) ([]byte, error) {
	data, err := runtime.Encode(patchCodec, inst)
	if err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	if err != nil {
		return nil, err
	}
	objCopy := make(map[string]interface{})
	specCopy := make(map[string]interface{})
	spec := raw["spec"].(map[string]interface{})
	template := spec["template"].(map[string]interface{})
	specCopy["template"] = template
	template["$patch"] = "replace"
	objCopy["spec"] = specCopy
	patch, err := json.Marshal(objCopy)
	return patch, err
}

// newControllerRevision returns a ControllerRevision with a ControllerRef pointing to parent and indicating that
// parent is of parentKind. The ControllerRevision has labels matching template labels, contains Data equal to data, and
// has a Revision equal to revision. The collisionCount is used when creating the name of the ControllerRevision
// so the name is likely unique. If the returned error is nil, the returned ControllerRevision is valid. If the
// returned error is not nil, the returned ControllerRevision is invalid for use.
func newControllerRevision(parent metav1.Object,
	parentKind schema.GroupVersionKind,
	templateLabels map[string]string,
	data runtime.RawExtension,
	revision int64,
	collisionCount *int32) (*appsv1.ControllerRevision, error) {
	labelMap := make(map[string]string)
	for k, v := range templateLabels {
		labelMap[k] = v
	}
	cr := &appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          labelMap,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(parent, parentKind)},
		},
		Data:     data,
		Revision: revision,
	}
	hash := hashControllerRevision(cr, collisionCount)
	cr.Name = controllerRevisionName(parent.GetName(), hash)
	cr.Labels[controllerRevisionHashLabel] = hash
	return cr, nil
}

// hashControllerRevision hashes the contents of revision's Data using FNV hashing. If probe is not nil, the byte value
// of probe is added written to the hash as well. The returned hash will be a safe encoded string to avoid bad words.
func hashControllerRevision(revision *appsv1.ControllerRevision, probe *int32) string {
	hf := fnv.New32()
	if len(revision.Data.Raw) > 0 {
		hf.Write(revision.Data.Raw)
	}
	if revision.Data.Object != nil {
		deepHashObject(hf, revision.Data.Object)
	}
	if probe != nil {
		hf.Write([]byte(strconv.FormatInt(int64(*probe), 10)))
	}
	return rand.SafeEncodeString(fmt.Sprint(hf.Sum32()))
}

// deepHashObject writes specified object to hash using the spew library
// which follows pointers and prints actual values of the nested objects
// ensuring the hash does not change when a pointer changes.
func deepHashObject(hasher hash.Hash, objectToWrite interface{}) {
	hasher.Reset()
	fmt.Fprintf(hasher, "%v", dump.ForHash(objectToWrite))
}

// controllerRevisionName returns the Name for a ControllerRevision in the form prefix-hash. If the length
// of prefix is greater than 223 bytes, it is truncated to allow for a name that is no larger than 253 bytes.
func controllerRevisionName(prefix string, hash string) string {
	if len(prefix) > 223 {
		prefix = prefix[:223]
	}
	return fmt.Sprintf("%s-%s", prefix, hash)
}

// getPodRevision gets the revision of Pod by inspecting the StatefulSetRevisionLabel. If pod has no revision the empty
// string is returned.
func getPodRevision(pod *corev1.Pod) string {
	if pod.Labels == nil {
		return ""
	}
	return pod.Labels[appsv1.ControllerRevisionHashLabelKey]
}

func buildInstancePodRevision(template *corev1.PodTemplateSpec, parent *workloads.Instance) (string, error) {
	podTemplate := filterInPlaceFields(template)
	inst := builder.NewInstanceBuilder(parent.Namespace, parent.Name).
		SetUID(parent.UID).
		AddAnnotationsInMap(parent.Annotations).
		SetSelectorMatchLabels(parent.Labels).
		SetPodTemplate(*podTemplate).
		GetObject()

	cr, err := newRevision(inst)
	if err != nil {
		return "", err
	}
	return cr.Labels[controllerRevisionHashLabel], nil
}
