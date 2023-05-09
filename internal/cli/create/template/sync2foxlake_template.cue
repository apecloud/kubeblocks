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

// required, command line input options for parameters and flags
options:{
  name:                 string
  namespace:            string
  source:               string
  sourceEndpointModel:  {}
  sink:                 string
  sinkEndpointModel:    {}
  selectedDatabase:     string
  databaseType:         string
  lag:                  string
  engine:               string
  quota:                string
  tablesIncluded:       [...string]
}
content: {
  apiVersion: "sync2foxlake.apecloud.io/v1alpha1"
  kind:       "Sync2FoxLakeTask"
  metadata: {
    name:         options.name
    namespaces:   options.namespace
  }
  spec: {
    sourceEndpoint: options.sourceEndpointModel
    sinkEndpoint: options.sinkEndpointModel
    syncDatabaseSpec: {
      databaseType: options.databaseType
      databaseSelected: options.selectedDatabase
      lag: options.lag
      engine: options.engine
      isPaused: false
      tablesIncluded: options.tablesIncluded
      quota: options.quota
    }
  }
}

