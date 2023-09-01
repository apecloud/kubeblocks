// Copyright (C) 2022-2023 ApeCloud Co., Ltd
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

#SystemLog: {
	destination:        string
	path:               string
	logAppend:          bool
	verbosity:          int
	quiet:              bool
	traceAllExceptions: bool
	syslogFacility:     string
	logRotate:          string
	timeStampFormat:    string

	component: #Component
	...
}

#Net: {
	port:                   int
	bindIp:                 string
	bindIpAll:              bool
	maxIncomingConnections: int
	wireObjectCheck:        bool
	tls:                    #Tls
}

#Tls: {
	certificateSelector:        string
	clusterCertificateSelector: string
	mode:                       string
	certificateKeyFile:         string
	certificateKeyFilePassword: string
	clusterFile:                string
	clusterPassword:            string
	CAFile:                     string
	clusterCAFile:              string
	...
}

#Component: {
	accessControl: {
		verbosity: int
	}
	command: {
		verbosity: int
	}
	replication: {
		verbosity: int
		election: {
			verbosity: int
		}
		rollback: {
			verbosity: int
		}
	}
	storage: #Storage
	...
}

#Storage: {
	verbosity: int
	journal: {
		verbosity: int
	}
	...
}

#MongodbParameters: {
	systemLog: #SystemLog
	net:       #Net
}

configuration: #MongodbParameters & {
}
