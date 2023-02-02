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

package engine

type ClientType string

const (
	CLI     ClientType = "cli"
	DJANGO  ClientType = "django"
	DOTNET  ClientType = ".net"
	GO      ClientType = "go"
	JAVA    ClientType = "java"
	NODEJS  ClientType = "node.js"
	PHP     ClientType = "php"
	PRISMA  ClientType = "prisma"
	PYTHON  ClientType = "python"
	RAILS   ClientType = "rails"
	RUST    ClientType = "rust"
	SYMFONY ClientType = "symfony"
)

func ClientTypes() []string {
	return []string{CLI.String(), DJANGO.String(), DOTNET.String(), GO.String(),
		JAVA.String(), NODEJS.String(), PHP.String(), PRISMA.String(),
		PYTHON.String(), RAILS.String(), RUST.String(), SYMFONY.String(),
	}
}

func (t ClientType) String() string {
	return string(t)
}
