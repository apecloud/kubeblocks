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
	"context"
	"testing"

	"github.com/dapr/components-contrib/bindings"
	"github.com/dapr/components-contrib/metadata"
	"github.com/dapr/kit/logger"
	"github.com/stretchr/testify/assert"

	. "github.com/apecloud/kubeblocks/cmd/probe/util"
)

// Test case for Init() function
func TestInit(t *testing.T) {
	kafkaOps := mockKafkaOps(t)
	ctx := context.Background()
	m := getMetadata()

	err := kafkaOps.kafka.Init(ctx, m)
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
	metadata := bindings.Metadata{
		Base: metadata.Base{
			Properties: map[string]string{},
		},
	}

	kafkaOps := NewKafka(logger.NewLogger("test")).(*KafkaOperations)
	_ = kafkaOps.Init(metadata)

	return kafkaOps
}

func getMetadata() map[string]string {
	return map[string]string{
		"consumerGroup": "a", "clientID": "a", "brokers": "a", "maxMessageBytes": "2048",
		"consumeRetryInterval": "200", "initialOffset": "newest", "authType": "mtls",
	}
}
