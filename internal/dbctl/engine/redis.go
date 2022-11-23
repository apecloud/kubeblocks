/*
Copyright ApeCloud Inc.

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

package engine

type redis struct{}

const (
	redisEngineName    = "redis"
	redisClient        = "redis-cli"
	redisContainerName = "redis"
)

var _ Interface = &redis{}

func (r *redis) ConnectCommand(database string) []string {
	return []string{redisClient}
}

func (r *redis) EngineName() string {
	return redisEngineName
}

func (r *redis) EngineContainer() string {
	return redisContainerName
}
