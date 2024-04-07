/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package rsmcommon

import (
	"encoding/base64"
	"encoding/json"

	"github.com/klauspost/compress/zstd"
	"k8s.io/apimachinery/pkg/util/runtime"
)

const revisionsZSTDKey = "zstd"

var (
	reader *zstd.Decoder
	writer *zstd.Encoder
)

func init() {
	var err error
	reader, err = zstd.NewReader(nil)
	runtime.Must(err)
	writer, err = zstd.NewWriter(nil)
	runtime.Must(err)
}

func GetUpdateRevisions(revisions map[string]string) (map[string]string, error) {
	if revisions == nil {
		return nil, nil
	}
	revisionsStr, ok := revisions[revisionsZSTDKey]
	if !ok {
		return revisions, nil
	}
	revisionsData, err := base64.StdEncoding.DecodeString(revisionsStr)
	if err != nil {
		return nil, err
	}
	revisionsJSON, err := reader.DecodeAll(revisionsData, nil)
	if err != nil {
		return nil, err
	}
	updateRevisions := make(map[string]string)
	if err = json.Unmarshal(revisionsJSON, &updateRevisions); err != nil {
		return nil, err
	}
	return updateRevisions, nil
}

func BuildUpdateRevisions(updateRevisions map[string]string) (map[string]string, error) {
	revisionsJSON, err := json.Marshal(updateRevisions)
	if err != nil {
		return nil, err
	}
	revisionsData := writer.EncodeAll(revisionsJSON, nil)
	revisionsStr := base64.StdEncoding.EncodeToString(revisionsData)
	return map[string]string{revisionsZSTDKey: revisionsStr}, nil
}
