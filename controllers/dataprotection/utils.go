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
package dataprotection

import (
	"context"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func checkResourceExists(
	ctx context.Context,
	client client.Client,
	key client.ObjectKey,
	obj client.Object) (bool, error) {

	if err := client.Get(ctx, key, obj); err != nil {
		// if err is NOT "not found", that means unknown error.
		if !strings.Contains(err.Error(), "not found") {
			return false, err
		}
		// if not found, return false
		return false, nil
	}
	// if found, return true
	return true, nil
}
