/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package instanceset

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"hash/fnv"
	"strconv"

	jsoniter "github.com/json-iterator/go"
	apps "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/dump"
	"k8s.io/apimachinery/pkg/util/rand"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/lru"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// ControllerRevisionHashLabel is the label used to indicate the hash value of a ControllerRevision's Data.
const ControllerRevisionHashLabel = "controller.kubernetes.io/hash"

var Codecs = serializer.NewCodecFactory(model.GetScheme())
var patchCodec = Codecs.LegacyCodec(workloads.SchemeGroupVersion)
var controllerKind = apps.SchemeGroupVersion.WithKind("StatefulSet")

var jsonIter = jsoniter.ConfigCompatibleWithStandardLibrary

func NewRevision(its *workloads.InstanceSet) (*apps.ControllerRevision, error) {
	patch, err := getPatch(its)
	if err != nil {
		return nil, err
	}
	collision := int32(0)
	cr, err := NewControllerRevision(its,
		controllerKind,
		its.Spec.Template.Labels,
		runtime.RawExtension{Raw: patch},
		1,
		&collision)
	if err != nil {
		return nil, err
	}
	if cr.ObjectMeta.Annotations == nil {
		cr.ObjectMeta.Annotations = make(map[string]string)
	}
	for key, value := range its.Annotations {
		cr.ObjectMeta.Annotations[key] = value
	}
	return cr, nil
}

// getPatch returns a strategic merge patch that can be applied to restore a StatefulSet to a
// previous version. If the returned error is nil the patch is valid. The current state that we save is just the
// PodSpecTemplate. We can modify this later to encompass more state (or less) and remain compatible with previously
// recorded patches.
func getPatch(its *workloads.InstanceSet) ([]byte, error) {
	data, err := runtime.Encode(patchCodec, its)
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

// ControllerRevisionName returns the Name for a ControllerRevision in the form prefix-hash. If the length
// of prefix is greater than 223 bytes, it is truncated to allow for a name that is no larger than 253 bytes.
func ControllerRevisionName(prefix string, hash string) string {
	if len(prefix) > 223 {
		prefix = prefix[:223]
	}

	return fmt.Sprintf("%s-%s", prefix, hash)
}

// NewControllerRevision returns a ControllerRevision with a ControllerRef pointing to parent and indicating that
// parent is of parentKind. The ControllerRevision has labels matching template labels, contains Data equal to data, and
// has a Revision equal to revision. The collisionCount is used when creating the name of the ControllerRevision
// so the name is likely unique. If the returned error is nil, the returned ControllerRevision is valid. If the
// returned error is not nil, the returned ControllerRevision is invalid for use.
func NewControllerRevision(parent metav1.Object,
	parentKind schema.GroupVersionKind,
	templateLabels map[string]string,
	data runtime.RawExtension,
	revision int64,
	collisionCount *int32) (*apps.ControllerRevision, error) {
	labelMap := make(map[string]string)
	for k, v := range templateLabels {
		labelMap[k] = v
	}
	cr := &apps.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          labelMap,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(parent, parentKind)},
		},
		Data:     data,
		Revision: revision,
	}
	hash := HashControllerRevision(cr, collisionCount)
	cr.Name = ControllerRevisionName(parent.GetName(), hash)
	cr.Labels[ControllerRevisionHashLabel] = hash
	return cr, nil
}

// HashControllerRevision hashes the contents of revision's Data using FNV hashing. If probe is not nil, the byte value
// of probe is added written to the hash as well. The returned hash will be a safe encoded string to avoid bad words.
func HashControllerRevision(revision *apps.ControllerRevision, probe *int32) string {
	hf := fnv.New32()
	if len(revision.Data.Raw) > 0 {
		hf.Write(revision.Data.Raw)
	}
	if revision.Data.Object != nil {
		DeepHashObject(hf, revision.Data.Object)
	}
	if probe != nil {
		hf.Write([]byte(strconv.FormatInt(int64(*probe), 10)))
	}
	return rand.SafeEncodeString(fmt.Sprint(hf.Sum32()))
}

// DeepHashObject writes specified object to hash using the spew library
// which follows pointers and prints actual values of the nested objects
// ensuring the hash does not change when a pointer changes.
func DeepHashObject(hasher hash.Hash, objectToWrite interface{}) {
	hasher.Reset()
	fmt.Fprintf(hasher, "%v", dump.ForHash(objectToWrite))
}

var revisionsCache = lru.New(1024)

func GetRevisions(revisions map[string]string) (map[string]string, error) {
	if revisions == nil {
		return nil, nil
	}
	revisionsStr, ok := revisions[revisionsZSTDKey]
	if !ok {
		return revisions, nil
	}
	if revisionsInCache, ok := revisionsCache.Get(revisionsStr); ok {
		return revisionsInCache.(map[string]string), nil
	}
	revisionsData, err := base64.StdEncoding.DecodeString(revisionsStr)
	if err != nil {
		return nil, err
	}
	revisionsJSON, err := reader.DecodeAll(revisionsData, nil)
	if err != nil {
		return nil, err
	}
	updateRevisions := make(map[string]string)

	if err = jsonIter.Unmarshal(revisionsJSON, &updateRevisions); err != nil {
		return nil, err
	}
	revisionsCache.Put(revisionsStr, updateRevisions)
	return updateRevisions, nil
}

func buildRevisions(updateRevisions map[string]string) (map[string]string, error) {
	maxPlainRevisionCount := viper.GetInt(MaxPlainRevisionCount)
	if len(updateRevisions) <= maxPlainRevisionCount {
		return updateRevisions, nil
	}
	revisionsJSON, err := jsonIter.Marshal(updateRevisions)
	if err != nil {
		return nil, err
	}
	revisionsData := writer.EncodeAll(revisionsJSON, nil)
	revisionsStr := base64.StdEncoding.EncodeToString(revisionsData)
	return map[string]string{revisionsZSTDKey: revisionsStr}, nil
}
