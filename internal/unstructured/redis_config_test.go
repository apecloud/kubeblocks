/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package unstructured

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/test/testdata"
)

func TestRedisConfig(t *testing.T) {
	c, err := LoadConfig("test", "", appsv1alpha1.RedisCfg)
	require.Nil(t, err)

	tests := []struct {
		keyArgs   []string
		valueArgs string
		wantErr   bool
		testKey   map[string]string
	}{{
		keyArgs:   []string{"port"},
		valueArgs: "6379",
		wantErr:   false,
		testKey: map[string]string{
			"port": "6379",
		},
	}, {
		keyArgs:   []string{"client-output-buffer-limit", "normal"},
		valueArgs: "0 0 0",
		wantErr:   false,
		testKey: map[string]string{
			"client-output-buffer-limit": "normal 0 0 0",
		},
	}, {
		keyArgs:   []string{"client-output-buffer-limit", "pubsub"},
		valueArgs: "256mb 64mb 60",
		wantErr:   false,
		testKey: map[string]string{
			"client-output-buffer-limit.pubsub": "256mb 64mb 60",
		},
	}, {
		keyArgs:   []string{"client-output-buffer-limit", "normal"},
		valueArgs: "128mb 32mb 0",
		wantErr:   false,
		testKey: map[string]string{
			"client-output-buffer-limit.normal": "128mb 32mb 0",
			"client-output-buffer-limit.pubsub": "256mb 64mb 60",
			"port":                              "6379",
		},
	}}
	for _, tt := range tests {
		t.Run("config_test", func(t *testing.T) {
			if err := c.Update(strings.Join(tt.keyArgs, "."), tt.valueArgs); (err != nil) != tt.wantErr {
				t.Errorf("Update() error = %v, wantErr %v", err, tt.wantErr)
			}

			for key, value := range tt.testKey {
				v, _ := c.GetString(key)
				if v != value {
					t.Errorf("GetString() param = %v, expected %v", key, value)
				}
			}
		})
	}
}

func TestRedisConfigGetAllParameters(t *testing.T) {
	type mockfn = func() ConfigObject

	tests := []struct {
		name string
		fn   mockfn
		want map[string]interface{}
	}{{
		name: "xxx",
		fn: func() ConfigObject {
			c, _ := LoadConfig("test", "", appsv1alpha1.RedisCfg)
			_ = c.Update("port", "123")
			_ = c.Update("a.b", "123 234")
			_ = c.Update("a.c", "345")
			_ = c.Update("a.d", "1 2")
			_ = c.Update("a.d.e", "1 2")
			return c
		},
		want: map[string]interface{}{
			"port":    "123",
			"a b 123": "234",
			"a c 345": "",
			"a d 1":   "2",
			"a d e":   "1 2",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.fn()
			if got := r.GetAllParameters(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetAllParameters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRedisConfig_Marshal(t *testing.T) {
	testDataFn := func(file string) string {
		b, _ := testdata.GetTestDataFileContent(file)
		return string(b)
	}

	commentsConfig := `# Lists may also be compressed.
# Compress depth is the number of quicklist ziplist nodes from *each* side of
# the list to *exclude* from compression.  The head and tail of the list
# are always uncompressed for fast push/pop operations.  Settings are:
# 0: disable all list compression
# 1: depth 1 means "don't start compressing until after 1 node into the list,
#    going from either the head or tail"
#    So: [head]->node->node->...->node->[tail]
#    [head], [tail] will always be uncompressed; inner nodes will compress.
# 2: [head]->[next]->node->node->...->node->[prev]->[tail]
#    2 here means: don't compress head or head->next or tail->prev or tail,
#    but compress all nodes between them.
# 3: [head]->[next]->[next]->node->node->...->node->[prev]->[prev]->[tail]
# etc.
list-compress-depth 0

# Sets have a special encoding in just one case: when a set is composed
# of just strings that happen to be integers in radix 10 in the range
# of 64 bit signed integers.
# The following configuration setting sets the limit in the size of the
# set in order to use this special memory saving encoding.
set-max-intset-entries 512

# Similarly to hashes and lists, sorted sets are also specially encoded in
# order to save a lot of space. This encoding is only used when the length and
# elements of a sorted set are below the following limits:
zset-max-listpack-entries 128
zset-max-listpack-value 64`

	tests := []struct {
		name    string
		input   string
		want    string
		updated map[string]interface{}
		wantErr bool
	}{{
		name:  "redis_cont_test",
		input: testDataFn("config_encoding/redis.conf"),
		want:  testDataFn("config_encoding/redis.conf"),
	}, {
		name:  "redis_cont_test",
		input: commentsConfig,
		want:  commentsConfig,
		updated: map[string]interface{}{
			"zset-max-listpack-entries": 128,
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadConfig(tt.name, tt.input, appsv1alpha1.RedisCfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for key, value := range tt.updated {
				require.Nil(t, config.Update(key, value))
			}
			got, err := config.Marshal()
			require.Nil(t, err)
			if got != tt.want {
				t.Errorf("Marshal() got = %v, want %v", got, tt.want)
			}
		})
	}
}
