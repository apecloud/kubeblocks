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

package migration

import "k8s.io/kubectl/pkg/util/templates"

// Cli Migration Command Examples
var (
	CreateTemplate = templates.Examples(`
		# Create a migration task to migrate the entire database under mysql: mydb1 and mytable1 under database: mydb2 to the target mysql
		kbcli migration create mytask --template apecloud-mysql2mysql 
		--source user:123456@127.0.0.1:3306 
		--sink user:123456@127.0.0.1:3305 
		--migration-object '"mydb1","mydb2.mytable1"'
		
		# Create a migration task to migrate the schema: myschema under database: mydb1 under PostgreSQL to the target PostgreSQL
		kbcli migration create mytask --template apecloud-pg2pg 
		--source user:123456@127.0.0.1:3306/mydb1 
		--sink user:123456@127.0.0.1:3305/mydb1
		--migration-object '"myschema"'

		# Use prechecks, data initialization, CDC, but do not perform structure initialization
		kbcli migration create mytask --template apecloud-pg2pg 
		--source user:123456@127.0.0.1:3306/mydb1 
		--sink user:123456@127.0.0.1:3305/mydb1
		--migration-object '"myschema"'
		--steps precheck=true,init-struct=false,init-data=true,cdc=true

		# Create a migration task with two tolerations
		kbcli migration create mytask --template apecloud-pg2pg 
		--source user:123456@127.0.0.1:3306/mydb1 
		--sink user:123456@127.0.0.1:3305/mydb1
		--migration-object '"myschema"'
		--tolerations '"step=global,key=engineType,value=pg,operator=Equal,effect=NoSchedule","step=init-data,key=diskType,value=ssd,operator=Equal,effect=NoSchedule"'

		# Limit resource usage when performing data initialization
		kbcli migration create mytask --template apecloud-pg2pg 
		--source user:123456@127.0.0.1:3306/mydb1 
		--sink user:123456@127.0.0.1:3305/mydb1
		--migration-object '"myschema"'
		--resources '"step=init-data,cpu=1000m,memory=1Gi"'
	`)
	DescribeExample = templates.Examples(`
		# describe a specified migration task
		kbcli migration describe mytask
	`)
	ListExample = templates.Examples(`
		# list all migration tasks
		kbcli migration list

		# list a single migration task with specified NAME
		kbcli migration list mytask

		# list a single migration task in YAML output format
		kbcli migration list mytask -o yaml

		# list a single migration task in JSON output format
		kbcli migration list mytask -o json

		# list a single migration task in wide output format
		kbcli migration list mytask -o wide
	`)
	TemplateExample = templates.Examples(`
		# list all migration templates
		kbcli migration templates

		# list a single migration template with specified NAME
		kbcli migration templates mytemplate

		# list a single migration template in YAML output format
		kbcli migration templates mytemplate -o yaml

		# list a single migration template in JSON output format
		kbcli migration templates mytemplate -o json

		# list a single migration template in wide output format
		kbcli migration templates mytemplate -o wide
	`)
	DeleteExample = templates.Examples(`
		# terminate a migration task named mytask and delete resources in k8s without affecting source and target data in database
		kbcli migration terminate mytask
	`)
	LogsExample = templates.Examples(`
		# Logs when returning to the "init-struct" step from the migration task mytask
		kbcli migration logs mytask --step init-struct

		# Logs only the most recent 20 lines when returning to the "cdc" step from the migration task mytask
		kbcli migration logs mytask --step cdc --tail=20
	`)
)
