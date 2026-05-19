/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
)

func TestNewPeriodicalEnqueueSource(t *testing.T) {
	cli := fake.NewClientBuilder().WithScheme(testScheme()).Build()
	src := NewPeriodicalEnqueueSource(cli, &dpv1alpha1.BackupList{}, 5*time.Second, PeriodicalEnqueueSourceOption{})
	require.NotNil(t, src)
	assert.Equal(t, 5*time.Second, src.period)
	assert.NotNil(t, src.objList)
	assert.NotNil(t, src.Client)
}

func TestPeriodicalEnqueueSource_String(t *testing.T) {
	cli := fake.NewClientBuilder().WithScheme(testScheme()).Build()
	src := NewPeriodicalEnqueueSource(cli, &dpv1alpha1.BackupList{}, 5*time.Second, PeriodicalEnqueueSourceOption{})
	s := src.String()
	assert.Contains(t, s, "periodical enqueue source")
	assert.Contains(t, s, "BackupList")
}

func TestPeriodicalEnqueueSource_String_NilList(t *testing.T) {
	src := &PeriodicalEnqueueSource{}
	s := src.String()
	assert.Equal(t, "periodical enqueue source: unknown type", s)
}
