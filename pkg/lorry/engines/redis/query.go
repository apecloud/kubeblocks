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

package redis

import (
	"context"
	"encoding/json"
)

func (mgr *Manager) Exec(ctx context.Context, cmd string) (int64, error) {
	args := tokenizeCmd2Args(cmd)
	return 0, mgr.client.Do(ctx, args...).Err()
}

func (mgr *Manager) Query(ctx context.Context, cmd string) ([]byte, error) {
	args := tokenizeCmd2Args(cmd)
	// parse result into a slice of string
	data, err := mgr.client.Do(ctx, args...).Result()
	if err != nil {
		return nil, err
	}
	// convert interface{} to []byte
	switch v := data.(type) {
	case map[interface{}]interface{}:
		strMap := make(map[string]interface{})
		for key, value := range v {
			strMap[key.(string)] = value
		}
		return json.Marshal(strMap)
	default:
		return json.Marshal(v)
	}
}
