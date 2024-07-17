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

package lifecycle

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	servicePortVar        = "KB_SERVICE_PORT"
	serviceUserVar        = "KB_SERVICE_USER"
	servicePasswordVar    = "KB_SERVICE_PASSWORD"
	primaryPodFQDNVar     = "KB_PRIMARY_POD_FQDN"
	membersAddressVar     = "KB_MEMBER_ADDRESSES"
	newMemberPodNameVar   = "KB_NEW_MEMBER_POD_NAME"
	newMemberPodIPVar     = "KB_NEW_MEMBER_POD_IP"
	leaveMemberPodNameVar = "KB_LEAVE_MEMBER_POD_NAME"
	leaveMemberPodIPVar   = "KB_LEAVE_MEMBER_POD_IP"
)

type memberJoin struct {
	namespace   string
	clusterName string
	compName    string
	podName     string
	podIP       string
}

var _ lifecycleAction = &memberJoin{}

func (a *memberJoin) name() string {
	return "memberJoin"
}

func (a *memberJoin) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	m, err := parameters4Member(ctx, cli, a.namespace, a.clusterName, a.compName)
	if err != nil {
		return nil, err
	}

	// - KB_NEW_MEMBER_POD_NAME: The pod name of the replica being added to the group.
	// - KB_NEW_MEMBER_POD_IP: The IP address of the replica being added to the group.
	m[newMemberPodNameVar] = a.podName
	m[newMemberPodIPVar] = a.podIP

	return m, nil
}

type memberLeave struct {
	namespace   string
	clusterName string
	compName    string
	podName     string
	podIP       string
}

var _ lifecycleAction = &memberLeave{}

func (a *memberLeave) name() string {
	return "memberLeave"
}

func (a *memberLeave) parameters(ctx context.Context, cli client.Reader) (map[string]string, error) {
	m, err := parameters4Member(ctx, cli, a.namespace, a.clusterName, a.compName)
	if err != nil {
		return nil, err
	}

	// - KB_LEAVE_MEMBER_POD_NAME: The pod name of the replica being removed from the group.
	// - KB_LEAVE_MEMBER_POD_IP: The IP address of the replica being removed from the group.
	m[leaveMemberPodNameVar] = a.podName
	m[leaveMemberPodIPVar] = a.podIP

	return m, nil
}

func parameters4Member(ctx context.Context, cli client.Reader, namespace, clusterName, compName string) (map[string]string, error) {
	// The container executing this action has access to following environment variables:
	//
	// - KB_SERVICE_PORT: The port used by the database service.
	// - KB_SERVICE_USER: The username with the necessary permissions to interact with the database service.
	// - KB_SERVICE_PASSWORD: The corresponding password for KB_SERVICE_USER to authenticate with the database service.
	// - KB_PRIMARY_POD_FQDN: The FQDN of the primary Pod within the replication group.
	// - KB_MEMBER_ADDRESSES: A comma-separated list of Pod addresses for all replicas in the group.
	//
	// Expected action output:
	// - On Failure: An error message, if applicable, indicating why the action failed.
	m := make(map[string]string)
	m[servicePortVar] = "" // TODO
	m[serviceUserVar] = ""
	m[servicePasswordVar] = ""
	m[primaryPodFQDNVar] = ""
	m[membersAddressVar] = ""
	return m, nil
}
