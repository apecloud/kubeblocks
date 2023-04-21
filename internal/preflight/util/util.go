/*
Copyright (C) 2022 ApeCloud Co., Ltd

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
