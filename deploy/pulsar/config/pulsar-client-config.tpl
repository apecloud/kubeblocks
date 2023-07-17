#
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
#

# Configuration for pulsar-client and pulsar-admin CLI tools

# URL for Pulsar REST API (for admin operations)
# For TLS:
# webServiceUrl=https://localhost:8443/

{{- $webServiceUrl := getEnvByName ( index $.podSpec.containers 0 ) "webServiceUrl" }}
{{- if eq $webServiceUrl "" }}
    {{ $webServiceUrl = "http://localhost:8080/" }}
{{- end }}

webServiceUrl={{- $webServiceUrl }}


# URL for Pulsar Binary Protocol (for produce and consume operations)
# For TLS:
# brokerServiceUrl=pulsar+ssl://localhost:6651/

{{- $brokerServiceUrl := getEnvByName ( index $.podSpec.containers 0 ) "brokerServiceUrl" }}
{{- if eq $brokerServiceUrl "" }}
    {{ $brokerServiceUrl = "pulsar://localhost:6650/" }}
{{- end }}

brokerServiceUrl= {{- $brokerServiceUrl }}
