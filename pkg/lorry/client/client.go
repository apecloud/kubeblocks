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

package client

import (
	"context"
	"errors"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"

	. "github.com/apecloud/kubeblocks/pkg/lorry/util"
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

	_, err := rest.InClusterConfig()
	if err != nil {
		// As the service does not run as a pod in the Kubernetes cluster,
		// it is unable to call the lorry service running as a pod using the pod's IP address.
		// In this scenario, it is recommended to use an k8s exec client instead.
		execClient, err := NewK8sExecClientWithPod(nil, &pod)
		if err != nil {
			return nil, err
		}
		if execClient != nil {
			return execClient, nil
		}
		return nil, nil
	}

	httpClient, err := NewHTTPClientWithPod(&pod)
	if err != nil {
		return nil, err
	}
	if httpClient != nil {
		return httpClient, nil
	}

	// return Client as nil explicitly to indicate that Client interface is nil,
	// or Client will be a non-nil interface value even newclient returns nil.
	return nil, nil
}

type Requester interface {
	Request(ctx context.Context, operation, method string, req map[string]any) (map[string]any, error)
}
type lorryClient struct {
	requester Requester
}

func (cli *lorryClient) GetRole(ctx context.Context) (string, error) {
	resp, err := cli.Request(ctx, string(GetRoleOperation), http.MethodGet, nil)
	if err != nil {
		return "", err
	}

	role, ok := resp["role"]
	if !ok {
		return "", nil
	}

	return role.(string), nil
}

func (cli *lorryClient) CreateUser(ctx context.Context, userName, password, roleName string) error {
	parameters := map[string]any{
		"userName": userName,
		"password": password,
	}
	if roleName != "" {
		parameters["roleName"] = roleName
	}

	req := map[string]any{"parameters": parameters}
	_, err := cli.Request(ctx, string(CreateUserOp), http.MethodPost, req)
	return err
}

func (cli *lorryClient) DeleteUser(ctx context.Context, userName string) error {
	parameters := map[string]any{
		"userName": userName,
	}
	req := map[string]any{"parameters": parameters}
	_, err := cli.Request(ctx, string(DeleteUserOp), http.MethodPost, req)
	return err
}

func (cli *lorryClient) DescribeUser(ctx context.Context, userName string) (map[string]any, error) {
	parameters := map[string]any{
		"userName": userName,
	}
	req := map[string]any{"parameters": parameters}
	resp, err := cli.Request(ctx, string(DescribeUserOp), http.MethodGet, req)
	if err != nil {
		return nil, err
	}
	user, ok := resp["user"]
	if !ok {
		return nil, nil
	}

	return user.(map[string]any), nil
}

func (cli *lorryClient) GrantUserRole(ctx context.Context, userName, roleName string) error {
	parameters := map[string]any{
		"userName": userName,
		"roleName": roleName,
	}
	req := map[string]any{"parameters": parameters}
	_, err := cli.Request(ctx, string(GrantUserRoleOp), http.MethodPost, req)
	return err
}

func (cli *lorryClient) RevokeUserRole(ctx context.Context, userName, roleName string) error {
	parameters := map[string]any{
		"userName": userName,
		"roleName": roleName,
	}
	req := map[string]any{"parameters": parameters}
	_, err := cli.Request(ctx, string(RevokeUserRoleOp), http.MethodPost, req)
	return err
}

func (cli *lorryClient) Switchover(ctx context.Context, primary, candidate string, force bool) error {
	parameters := map[string]any{
		"primary":   primary,
		"candidate": candidate,
		"force":     force,
	}
	req := map[string]any{"parameters": parameters}
	_, err := cli.Request(ctx, string(SwitchoverOperation), http.MethodPost, req)
	return err
}

// ListUsers lists all normal users created
func (cli *lorryClient) ListUsers(ctx context.Context) ([]map[string]any, error) {
	resp, err := cli.Request(ctx, string(ListUsersOp), http.MethodGet, nil)
	if err != nil {
		return nil, err
	}
	users, ok := resp["users"]
	if !ok {
		return nil, nil
	}
	return convertToArrayOfMap(users)
}

// ListSystemAccounts lists all system accounts created
func (cli *lorryClient) ListSystemAccounts(ctx context.Context) ([]map[string]any, error) {
	resp, err := cli.Request(ctx, string(ListSystemAccountsOp), http.MethodGet, nil)
	if err != nil {
		return nil, err
	}
	systemAccounts, ok := resp["systemAccounts"]
	if !ok {
		return nil, nil
	}
	return convertToArrayOfMap(systemAccounts)
}

// JoinMember sends a join member operation request to Lorry, located on the target pod that is about to join.
func (cli *lorryClient) JoinMember(ctx context.Context) error {
	_, err := cli.Request(ctx, string(JoinMemberOperation), http.MethodPost, nil)
	return err
}

// LeaveMember sends a Leave member operation request to Lorry, located on the target pod that is about to leave.
func (cli *lorryClient) LeaveMember(ctx context.Context) error {
	_, err := cli.Request(ctx, string(LeaveMemberOperation), http.MethodPost, nil)
	return err
}

func (cli *lorryClient) Request(ctx context.Context, operation, method string, req map[string]any) (map[string]any, error) {
	if cli.requester == nil {
		return nil, errors.New("lorry client's requester must be set")
	}
	return cli.requester.Request(ctx, operation, method, req)
}
