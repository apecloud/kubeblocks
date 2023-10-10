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

package mysql

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
)

func (mgr *Manager) Query(ctx context.Context, sql string) ([]byte, error) {
	mgr.Logger.Info(fmt.Sprintf("query: %s", sql))
	rows, err := mgr.DB.QueryContext(ctx, sql)
	if err != nil {
		return nil, errors.Wrapf(err, "error executing %s", sql)
	}
	defer func() {
		_ = rows.Close()
		_ = rows.Err()
	}()
	result, err := jsonify(rows)
	if err != nil {
		return nil, errors.Wrapf(err, "error marshalling query result for %s", sql)
	}
	return result, nil
}

func (mgr *Manager) Exec(ctx context.Context, sql string) (int64, error) {
	mgr.Logger.Info(fmt.Sprintf("exec: %s", sql))
	res, err := mgr.DB.ExecContext(ctx, sql)
	if err != nil {
		return 0, errors.Wrapf(err, "error executing %s", sql)
	}
	return res.RowsAffected()
}
