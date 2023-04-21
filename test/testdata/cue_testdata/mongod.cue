//Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
//This file is part of KubeBlocks project
//
//This program is free software: you can redistribute it and/or modify
//it under the terms of the GNU Affero General Public License as published by
//the Free Software Foundation, either version 3 of the License, or
//(at your option) any later version.
//
//This program is distributed in the hope that it will be useful
//but WITHOUT ANY WARRANTY; without even the implied warranty of
//MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//GNU Affero General Public License for more details.
//
//You should have received a copy of the GNU Affero General Public License
//along with this program.  If not, see <http://www.gnu.org/licenses/>.

#MongodParameter: {
	net: {
		port: int & >=0 & <=65535

		unixDomainSocket: {
			// Enables Unix Domain Sockets used for all network connections
			enabled:    bool | *false
			pathPrefix: string
			...
		}
		tls: {
			// Enables TLS used for all network connections
			mode: string & "disabled" | "allowTLS" | "preferTLS" | "requireTLS"

			certificateKeyFile: string
			CAFile:             string
			CRLFile:            string
			...
		}
		...
	}
	tls: {
		// Enables TLS used for all network connections
		mode: string & "disabled" | "allowTLS" | "preferTLS" | "requireTLS"

		certificateKeyFile: string
		CAFile:             string
		CRLFile:            string
		...
	}

	...
}

// configuration require
configuration: #MongodParameter & {
}
