// Copyright (C) 2022-2025 ApeCloud Co., Ltd
//
// This file is part of KubeBlocks project
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

import "time"

#TestSA: {
	field1: string
	field2: int
	filed3: {
		field1: string
		field2: int
		field3: time.Time
	}
}

// mysql config validator
#Exemplar: {
	l: {
		name: string
	}
	sa: #TestSA

	ta: {
		field1: string
		field2: int
	}

	...

	// not support AdditionalProperties.AdditionalProperties
	// x: [Name=_]: #TestSA
}

[N=string]: #Exemplar & {
	l: name: N
}

// configuration require
configuration: #Exemplar & {
}
