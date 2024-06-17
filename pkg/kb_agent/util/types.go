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

package util

const (
	CheckRoleOperation    = "checkRole"
	KBAgentEventFieldPath = "spec.containers{kb-agent}"
)

type CronJob struct {
	TimeoutSeconds   int `json:"timeoutSeconds,omitempty"`
	PeriodSeconds    int `json:"periodSeconds,omitempty"`
	SuccessThreshold int `json:"successThreshold,omitempty"`
	FailureThreshold int `json:"failureThreshold,omitempty"`
}

type Handlers struct {
	Command []string          `json:"command,omitempty"`
	GPRC    map[string]string `json:"grpc,omitempty"`
	CronJob *CronJob          `json:"cronJob,omitempty"`
}
