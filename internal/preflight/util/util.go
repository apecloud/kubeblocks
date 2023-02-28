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

package util

import (
	"strconv"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
)

func TitleOrDefault[T troubleshootv1beta2.HostCollectorMeta | troubleshootv1beta2.AnalyzeMeta](meta T, defaultTitle string) string {
	var title string
	iMeta := (interface{})(meta)
	switch tmp := iMeta.(type) {
	case troubleshootv1beta2.HostCollectorMeta:
		title = tmp.CollectorName
	case troubleshootv1beta2.AnalyzeMeta:
		title = tmp.CheckName
	default:
		title = ""
	}
	if title == "" {
		title = defaultTitle
	}
	return title
}

func IsExcluded(excludeVal *multitype.BoolOrString) (bool, error) {
	if excludeVal == nil {
		return false, nil
	}
	if excludeVal.Type == multitype.Bool {
		return excludeVal.BoolVal, nil
	}
	if excludeVal.StrVal == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(excludeVal.StrVal)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse bool string")
	}
	return parsed, nil
}
