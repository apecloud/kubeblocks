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
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/rand"
)

func ComputeHash(object interface{}) (string, error) {
	objString, err := json.Marshal(object)
	if err != nil {
		return "", errors.Wrap(err, "failed to compute hash.")
	}

	// hasher := sha1.New()
	hasher := fnv.New32()
	if _, err := io.Copy(hasher, bytes.NewReader(objString)); err != nil {
		return "", errors.Wrapf(err, "failed to compute hash for sha256. [%s]", objString)
	}

	sha := hasher.Sum32()
	return rand.SafeEncodeString(fmt.Sprint(sha)), nil
}
