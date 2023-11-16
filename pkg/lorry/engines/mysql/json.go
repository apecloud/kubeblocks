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
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"reflect"
)

func jsonify(rows *sql.Rows) ([]byte, error) {
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	var ret []interface{}
	for rows.Next() {
		values := prepareValues(columnTypes)
		err = rows.Scan(values...)
		if err != nil {
			return nil, err
		}
		r, err := convert(columnTypes, values)
		if err != nil {
			return nil, err
		}
		ret = append(ret, r)
	}
	return json.Marshal(ret)
}

func prepareValues(columnTypes []*sql.ColumnType) []interface{} {
	types := make([]reflect.Type, len(columnTypes))
	for i, tp := range columnTypes {
		types[i] = tp.ScanType()
	}
	values := make([]interface{}, len(columnTypes))
	for i := range values {
		switch types[i].Kind() {
		case reflect.String, reflect.Interface:
			values[i] = &sql.NullString{}
		case reflect.Bool:
			values[i] = &sql.NullBool{}
		case reflect.Float64:
			values[i] = &sql.NullFloat64{}
		case reflect.Int16, reflect.Uint16:
			values[i] = &sql.NullInt16{}
		case reflect.Int32, reflect.Uint32:
			values[i] = &sql.NullInt32{}
		case reflect.Int64, reflect.Uint64:
			values[i] = &sql.NullInt64{}
		default:
			values[i] = reflect.New(types[i]).Interface()
		}
	}
	return values
}

func convert(columnTypes []*sql.ColumnType, values []interface{}) (map[string]interface{}, error) {
	r := map[string]interface{}{}
	for i, ct := range columnTypes {
		value := values[i]
		switch v := values[i].(type) {
		case driver.Valuer:
			if vv, err := v.Value(); err != nil {
				return nil, err
			} else {
				value = interface{}(vv)
			}
		case *sql.RawBytes:
			// special case for sql.RawBytes, see https://github.com/go-sql-driver/mysql/blob/master/fields.go#L178
			switch ct.DatabaseTypeName() {
			case "VARCHAR", "CHAR", "TEXT", "LONGTEXT":
				value = string(*v)
			}
		}
		if value != nil {
			r[ct.Name()] = value
		}
	}
	return r, nil
}
