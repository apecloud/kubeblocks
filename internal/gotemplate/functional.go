/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package gotemplate

import (
	"fmt"
	"regexp"
	"strconv"

	"gopkg.in/yaml.v2"
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

func fromYAML(str string) (map[string]interface{}, error) {
	m := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(str), &m)
	return m, err
}

func fromYAMLArray(str string) ([]interface{}, error) {
	var a []interface{}
	err := yaml.Unmarshal([]byte(str), &a)
	return a, err
}
