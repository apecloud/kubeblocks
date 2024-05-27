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

package models

import "strings"

const (
	PRIMARY   = "primary"
	SECONDARY = "secondary"
	MASTER    = "master"
	SLAVE     = "slave"
	LEADER    = "Leader"
	FOLLOWER  = "Follower"
	LEARNER   = "Learner"
	CANDIDATE = "Candidate"
)

// IsLikelyPrimaryRole returns true if the role is primary,
// it is used for the case where db manager do not implemement the IsLeader method.
// use it curefully, as it is for normal case, and may be wrong for some special cases.
func IsLikelyPrimaryRole(role string) bool {
	return strings.EqualFold(role, PRIMARY) || strings.EqualFold(role, MASTER) || strings.EqualFold(role, LEADER)
}
