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
	troubleshoot "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
)

// TitleOrDefault extracts titleName from metaInfo, and returns default if metaInfo is unhelpful
func TitleOrDefault[T troubleshoot.HostCollectorMeta | troubleshoot.AnalyzeMeta](meta T, defaultTitle string) string {
	var title string
	iMeta := (interface{})(meta)
	switch tmp := iMeta.(type) {
	case troubleshoot.HostCollectorMeta:
		title = tmp.CollectorName
	case troubleshoot.AnalyzeMeta:
		title = tmp.CheckName
	default:
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
