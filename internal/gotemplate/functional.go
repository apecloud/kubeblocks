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

package gotemplate

import (
	"fmt"
	"regexp"
	"strconv"

	corev1 "k8s.io/api/core/v1"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

const (
	GoTemplateLibraryAnnotationKey = "config.kubeblocks.io/go-template-library"
)

func isSystemFuncsCM(cm *corev1.ConfigMap) bool {
	if len(cm.Annotations) == 0 {
		return false
	}

	v, ok := cm.Annotations[GoTemplateLibraryAnnotationKey]
	if !ok {
		return false
	}
	b, err := strconv.ParseBool(v)
	return err == nil && b
}

func ConstructFunctionArgList(args ...interface{}) TplValues {
	values := TplValues{}
	for i, arg := range args {
		values[constructArgsValueKey(i)] = arg
	}
	return values
}

func constructArgsValueKey(i int) string {
	return fmt.Sprintf("arg%d", i)
}

func failed(errMessage string, args ...interface{}) (string, error) {
	return "", cfgcore.MakeError(errMessage, args...)
}

func regexStringSubmatch(regex string, s string) ([]string, error) {
	r, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}
	return r.FindStringSubmatch(s), nil
}
