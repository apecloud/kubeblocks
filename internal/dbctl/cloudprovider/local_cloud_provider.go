/*
Copyright 2022 The KubeBlocks Authors

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
package cloudprovider

const (
	Local = "local"
)

type localCloudProvider struct {
}

func (p *localCloudProvider) Name() string {
	return Local
}

func (p *localCloudProvider) Apply(destroy bool) error {
	return nil
}

func (p *localCloudProvider) Instance() (Instance, error) {
	return &localInstance{}, nil
}

type localInstance struct {
}

func (l *localInstance) GetIP() string {
	return "127.0.0.1"
}
