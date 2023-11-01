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
