/*
Copyright ApeCloud, Inc.

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

package apps

import (
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getEnvReplacementMapForAccount(name, passwd string) map[string]string {
	return map[string]string{
		"$(USERNAME)": name,
		"$(PASSWD)":   passwd,
	}
}

func createK8sObject(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1alpha1.Cluster, obj client.Object) error {
	reqCtx.Log.V(1).Info("create or update object", obj)
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	if err := intctrlutil.SetOwnership(cluster, obj, scheme, dbClusterFinalizerName); err != nil {
		return err
	}
	return cli.Create(reqCtx.Ctx, obj)
}
