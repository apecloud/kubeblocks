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

package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/apecloud/kubeblocks/internal/sqlchannel/util"
)

// Test case for Init() function
func TestInit(t *testing.T) {
	kafkaOps := mockKafkaOps(t)

	err := kafkaOps.Init()
	if err != nil {
		t.Errorf("Error during Init(): %s", err)
	}

	assert.Equal(t, "kafka", kafkaOps.DBType)
	assert.NotNil(t, kafkaOps.InitIfNeed)
	assert.NotNil(t, kafkaOps.OperationMap[CheckStatusOperation])
}

func TestCheckStatusOps(t *testing.T) {
	// TODO: find mock way
}

func mockKafkaOps(t *testing.T) *KafkaOperations {
	// todo 没有配置文件
	kafkaOps, _ := NewKafka()
	_ = kafkaOps.Init()
	return kafkaOps
}
