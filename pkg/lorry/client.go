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

package lorry

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// HACK: for unit test only.
var mockClient Client
var mockClientError error

func SetMockClient(cli Client, err error) {
	mockClient = cli
	mockClientError = err
}

func UnsetMockClient() {
	mockClient = nil
	mockClientError = nil
}

func GetMockClient() Client {
	return mockClient
}

func NewClient(pod corev1.Pod) (Client, error) {
	if mockClient != nil || mockClientError != nil {
		return mockClient, mockClientError
	}

	return nil, fmt.Errorf("not implemented")
}
