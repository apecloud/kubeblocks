/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package parameters

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/parameters"
)

type resourceRenderFingerprint struct {
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

// Quantities are encoded independently of their original format (1Gi == 1024Mi).
// Missing values remain distinct from an explicitly configured zero.
func canonicalResourceQuantity(q resource.Quantity) string {
	mantissa, exponent := q.AsCanonicalBytes(nil)
	return fmt.Sprintf("%se%d", mantissa, exponent)
}

func resourceRequirementsInput(requests, limits corev1.ResourceList, names ...corev1.ResourceName) map[string]string {
	values := make(map[string]string)
	for _, name := range names {
		if q, ok := requests[name]; ok {
			values["requests/"+string(name)] = canonicalResourceQuantity(q)
		}
		if q, ok := limits[name]; ok {
			values["limits/"+string(name)] = canonicalResourceQuantity(q)
		}
	}
	return values
}

func resourceVars(cmpd *appsv1.ComponentDefinition) []appsv1.EnvVar {
	var refs []appsv1.EnvVar
	for _, v := range cmpd.Spec.Vars {
		if v.ValueFrom != nil && v.ValueFrom.ResourceVarRef != nil {
			refs = append(refs, v)
		}
	}
	return refs
}

// resourceRenderInputFingerprint tracks both built-in component resources and
// declared resource vars. It does not parse templates or change rendering rules.
func resourceRenderInputFingerprint(ctx context.Context, reader client.Reader, cmpd *appsv1.ComponentDefinition,
	comp *appsv1.Component, synth *component.SynthesizedComponent) (resourceRenderFingerprint, error) {
	input := struct {
		Resources   map[string]string            `json:"resources"`
		Volumes     map[string]map[string]string `json:"volumes"`
		Vars        map[string]string            `json:"vars,omitempty"`
		Definitions []appsv1.EnvVar              `json:"definitions,omitempty"`
	}{
		Resources:   resourceRequirementsInput(comp.Spec.Resources.Requests, comp.Spec.Resources.Limits, corev1.ResourceCPU, corev1.ResourceMemory),
		Volumes:     make(map[string]map[string]string),
		Definitions: resourceVars(cmpd),
	}
	for _, vct := range comp.Spec.VolumeClaimTemplates {
		input.Volumes[vct.Name] = resourceRequirementsInput(vct.Spec.Resources.Requests, vct.Spec.Resources.Limits, corev1.ResourceStorage)
	}
	if len(input.Definitions) > 0 {
		var err error
		if synth == nil {
			synth, err = component.BuildSynthesizedComponent(ctx, reader, cmpd, comp)
			if err != nil {
				return resourceRenderFingerprint{}, err
			}
		}
		input.Vars, err = component.ResolveResourceRenderVars(ctx, reader, synth, input.Definitions)
		if err != nil {
			return resourceRenderFingerprint{}, err
		}
	}
	data, err := json.Marshal(input)
	if err != nil {
		return resourceRenderFingerprint{}, err
	}
	digest := sha256.Sum256(data)
	return resourceRenderFingerprint{Version: "v1", Digest: hex.EncodeToString(digest[:])}, nil
}

func applyResourceRenderInputs(ctx context.Context, reader client.Reader, cmpd *appsv1.ComponentDefinition,
	comp *appsv1.Component, spec *parametersv1alpha1.ComponentParameterSpec) error {
	reader = newResourceSnapshotReader(reader, comp)
	fingerprint, err := resourceRenderInputFingerprint(ctx, reader, cmpd, comp, nil)
	if err != nil {
		return err
	}
	for i := range spec.ConfigItemDetails {
		if _, err = parameters.CheckAndPatchPayload(&spec.ConfigItemDetails[i], constant.ResourceRenderInputsPayload, fingerprint); err != nil {
			return err
		}
	}
	return nil
}

// Preserve payload owned by external controllers while replacing/removing only
// keys managed by ComponentDrivenParameterReconciler.
func mergeResourcePayload(dest, expected *parametersv1alpha1.ConfigTemplateItemDetail) {
	for _, key := range []string{constant.ComponentResourcePayload, constant.ReplicasPayload,
		constant.TLSPayload, constant.ShardingPayload, constant.ResourceRenderInputsPayload} {
		delete(dest.Payload, key)
	}
	for key, value := range expected.Payload {
		if dest.Payload == nil {
			dest.Payload = make(parametersv1alpha1.Payload)
		}
		dest.Payload[key] = slices.Clone(value)
	}
	if len(dest.Payload) == 0 {
		dest.Payload = nil
	}
}

// Reuse the same component reads for fingerprinting and rendering. This prevents
// resource vars from observing a newer component than the revision they describe.
type resourceSnapshotReader struct {
	client.Reader
	components map[client.ObjectKey]*appsv1.Component
	errors     map[client.ObjectKey]error
}

func newResourceSnapshotReader(reader client.Reader, comp *appsv1.Component) *resourceSnapshotReader {
	return &resourceSnapshotReader{Reader: reader, errors: make(map[client.ObjectKey]error), components: map[client.ObjectKey]*appsv1.Component{client.ObjectKeyFromObject(comp): comp.DeepCopy()}}
}

func (r *resourceSnapshotReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	comp, ok := obj.(*appsv1.Component)
	if !ok {
		return r.Reader.Get(ctx, key, obj, opts...)
	}
	if err, exists := r.errors[key]; exists {
		return err
	}
	if cached, exists := r.components[key]; exists {
		*comp = *cached.DeepCopy()
		return nil
	}
	if err := r.Reader.Get(ctx, key, comp, opts...); err != nil {
		r.errors[key] = err
		return err
	}
	r.components[key] = comp.DeepCopy()
	return nil
}
