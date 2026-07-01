/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package revisionmap

import (
	"encoding/base64"

	jsoniter "github.com/json-iterator/go"
	"github.com/klauspost/compress/zstd"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/apecloud/kubeblocks/pkg/lru"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

const (
	// MaxPlainRevisionCount specifies max number of plain revisions stored in status revision maps.
	MaxPlainRevisionCount = "MAX_PLAIN_REVISION_COUNT"

	revisionsZSTDKey = "zstd"
)

var (
	jsonIter = jsoniter.ConfigCompatibleWithStandardLibrary

	reader *zstd.Decoder
	writer *zstd.Encoder

	revisionsCache = lru.New(1024)
)

func init() {
	var err error
	reader, err = zstd.NewReader(nil)
	utilruntime.Must(err)
	writer, err = zstd.NewWriter(nil)
	utilruntime.Must(err)
}

// Decode returns the plain revision map from a status-safe revision map.
func Decode(revisions map[string]string) (map[string]string, error) {
	if revisions == nil {
		return nil, nil
	}
	revisionsStr, ok := revisions[revisionsZSTDKey]
	if !ok {
		return revisions, nil
	}
	if revisionsInCache, ok := revisionsCache.Get(revisionsStr); ok {
		return revisionsInCache.(map[string]string), nil
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
	if err = jsonIter.Unmarshal(revisionsJSON, &updateRevisions); err != nil {
		return nil, err
	}
	revisionsCache.Put(revisionsStr, updateRevisions)
	return updateRevisions, nil
}

// Encode returns a status-safe revision map, compressing it when it exceeds the plain-entry limit.
func Encode(revisions map[string]string) (map[string]string, error) {
	maxPlainRevisionCount := viper.GetInt(MaxPlainRevisionCount)
	if len(revisions) <= maxPlainRevisionCount {
		return revisions, nil
	}
	revisionsJSON, err := jsonIter.Marshal(revisions)
	if err != nil {
		return nil, err
	}
	revisionsData := writer.EncodeAll(revisionsJSON, nil)
	revisionsStr := base64.StdEncoding.EncodeToString(revisionsData)
	return map[string]string{revisionsZSTDKey: revisionsStr}, nil
}
