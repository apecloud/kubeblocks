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
