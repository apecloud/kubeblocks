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

package v1

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextvalidation "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	apps "github.com/apecloud/kubeblocks/apis/apps/v1"
)

func TestLifecycleActionsAPIRoundTrip(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	codecs := serializer.NewCodecFactory(scheme)
	codec := codecs.LegacyCodec(GroupVersion)
	action := func(command string) *Action {
		return &Action{Exec: &apps.ExecAction{Command: []string{"/bin/sh", "-ec", command}, Env: []corev1.EnvVar{{Name: "ACTION_ENV", Value: "original"}}}}
	}
	actions := &LifecycleActions{
		TemplateVars: map[string]string{"DATA_PATH": "/data/payload"},
		MemberJoin:   action("join"), MemberLeave: action("leave"),
		DataDump: action("cat /data/payload"), DataLoad: action("cat > /data/payload"),
		DataVolume: "data",
		Worker: &corev1.Container{Name: "loader", Image: "tools:current", Command: []string{"/bin/kbagent"},
			Env:             []corev1.EnvVar{{Name: "WORKER_ENV", Value: "original"}},
			VolumeMounts:    []corev1.VolumeMount{{Name: "data", MountPath: "/data"}},
			SecurityContext: &corev1.SecurityContext{RunAsUser: ptr.To(int64(65532))}},
	}
	for _, withActions := range []bool{false, true} {
		for _, result := range []*bool{nil, ptr.To(false), ptr.To(true)} {
			object := &InstanceSet{Status: InstanceSetStatus{InstanceStatus: []InstanceStatus{{PodName: "replica-1", DataLoaded: result, MemberJoined: result}}}}
			if withActions {
				object.Spec.LifecycleActions = actions.DeepCopy()
			}
			data, err := runtime.Encode(codec, object)
			if err != nil {
				t.Fatal(err)
			}
			decoded, _, err := codecs.UniversalDeserializer().Decode(data, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := decoded.(*InstanceSet)
			if !ok || !reflect.DeepEqual(got.Spec, object.Spec) || !reflect.DeepEqual(got.Status, object.Status) {
				t.Fatalf("API fields lost for actions=%v result=%v: %#v", withActions, result, got)
			}
			if !withActions && bytes.Contains(data, []byte(`"lifecycleActions"`)) {
				t.Fatal("absent actions serialized as enabled")
			}
			if result == nil && (bytes.Contains(data, []byte(`"dataLoaded"`)) || bytes.Contains(data, []byte(`"memberJoined"`))) {
				t.Fatal("unknown result serialized as tracked")
			}
		}
	}
	copy := actions.DeepCopy()
	copy.TemplateVars["DATA_PATH"] = "changed"
	copy.MemberJoin.Exec.Command[0] = "changed"
	copy.MemberLeave.Exec.Env[0].Value = "changed"
	copy.DataDump.Exec.Command[0] = "changed"
	copy.DataLoad.Exec.Env[0].Value = "changed"
	copy.Worker.Env[0].Value = "changed"
	copy.Worker.VolumeMounts[0].MountPath = "changed"
	*copy.Worker.SecurityContext.RunAsUser = 1
	if actions.TemplateVars["DATA_PATH"] != "/data/payload" || actions.MemberJoin.Exec.Command[0] != "/bin/sh" || actions.MemberLeave.Exec.Env[0].Value != "original" || actions.DataDump.Exec.Command[0] != "/bin/sh" || actions.DataLoad.Exec.Env[0].Value != "original" || actions.Worker.Env[0].Value != "original" || actions.Worker.VolumeMounts[0].MountPath != "/data" || *actions.Worker.SecurityContext.RunAsUser != 65532 {
		t.Fatal("deepcopy aliases lifecycle inputs")
	}
}

func TestDataCopyExampleUsesGeneratedSchema(t *testing.T) {
	readCRD := func(path string) *apiextensionsv1.CustomResourceDefinition {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var crd apiextensionsv1.CustomResourceDefinition
		if err := yaml.Unmarshal(data, &crd); err != nil {
			t.Fatal(err)
		}
		return &crd
	}
	crd := readCRD("../../../config/crd/bases/workloads.kubeblocks.io_instancesets.yaml")
	helm := readCRD("../../../deploy/helm/crds/workloads.kubeblocks.io_instancesets.yaml")
	if !reflect.DeepEqual(crd.Spec, helm.Spec) {
		t.Fatal("Helm InstanceSet CRD differs from base schema")
	}
	schema := crd.Spec.Versions[0].Schema.OpenAPIV3Schema
	lifecycle := schema.Properties["spec"].Properties["lifecycleActions"]
	for _, name := range []string{"memberJoin", "memberLeave", "dataDump", "dataLoad", "dataVolume", "worker"} {
		if _, ok := lifecycle.Properties[name]; !ok {
			t.Fatalf("missing lifecycle schema: %s", name)
		}
	}
	if !slices.Contains(lifecycle.Properties["worker"].Required, "name") {
		t.Fatal("worker name is not required")
	}
	var internal apiextensions.JSONSchemaProps
	if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(schema, &internal, nil); err != nil {
		t.Fatal(err)
	}
	validator, _, err := apiextvalidation.NewSchemaValidator(&internal)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../../examples/workloads/data-copy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]interface{}
	if err := yaml.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	if errs := apiextvalidation.ValidateCustomResource(field.NewPath(""), object, validator); len(errs) > 0 {
		t.Fatalf("example violates CRD: %v", errs)
	}
	jsonData, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	var its InstanceSet
	if err := json.Unmarshal(jsonData, &its); err != nil {
		t.Fatal(err)
	}
	if its.Spec.LifecycleActions.Worker.Image == "" || its.Spec.LifecycleActions.DataDump == nil || its.Spec.LifecycleActions.DataLoad == nil || its.Spec.Template.Spec.ServiceAccountName != "workloads-data-loader" || !ptr.Deref(its.Spec.Template.Spec.AutomountServiceAccountToken, false) {
		t.Fatal("example is missing data worker inputs or explicit API access")
	}
	// A Worker without its required Container name must fail CRD validation.
	delete(object["spec"].(map[string]interface{})["lifecycleActions"].(map[string]interface{})["worker"].(map[string]interface{}), "name")
	if errs := apiextvalidation.ValidateCustomResource(field.NewPath(""), object, validator); len(errs) == 0 {
		t.Fatal("schema accepted worker without name")
	}
}
