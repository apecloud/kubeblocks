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

package organization

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/pkg/errors"
)

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
}

func NewRequest(method, path, token string, requestBody []byte) ([]byte, error) {
	client := cleanhttp.DefaultClient()
	req, err := http.NewRequest(method, path, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create request for %s", path)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to perform request for %s", path)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errorResponse ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errorResponse)
		if err != nil {
			return nil, errors.Wrapf(err, "code: %d, failed to decode error response body for %s", resp.StatusCode, path)
		}
		return nil, fmt.Errorf("request failed with status code: %d for %s\nreason: %s %s", resp.StatusCode, path, errorResponse.Reason, errorResponse.Message)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read response body for %s", path)
	}

	return responseBody, nil
}
