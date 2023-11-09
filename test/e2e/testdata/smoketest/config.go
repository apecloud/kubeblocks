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

package smoketest

import (
	"log"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/apecloud/kubeblocks/test/e2e"
	e2eutil "github.com/apecloud/kubeblocks/test/e2e/util"
)

func Config() {
	BeforeEach(func() {
	})

	AfterEach(func() {
	})

	dir, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}

	Context("Configure running e2e information", func() {
		It("create a secret to save the access key and ", func() {
			secret := "kubectl get secret " + ConfigType + "-credential-for-backuprepo -n kb-system | grep " +
				ConfigType + "-credential-for-backuprepo | awk '{print $1}'"
			if checkResourceExists(secret) {
				log.Println("secret " + ConfigType + "-credential-for-backuprepo already exists")
			} else {
				var accessKey, secretKey string
				if ConfigType == "s3" {
					accessKey = e2eutil.ExecCommand("aws configure get aws_access_key_id")
					secretKey = e2eutil.ExecCommand("aws configure get aws_secret_access_key")
				} else {
					accessKey = e2eutil.ExecCommand("aliyun configure get access_key_id")
					secretKey = e2eutil.ExecCommand("aliyun configure get access_key_secret")
				}
				createSecret := "kubectl create secret generic " + ConfigType + "-credential-for-backuprepo \\\n" +
					"  -n kb-system \\\n" +
					"  --from-literal=accessKeyId=" + e2eutil.StringStrip(accessKey) + " \\\n" +
					"  --from-literal=secretAccessKey=" + e2eutil.StringStrip(secretKey)
				b := e2eutil.ExecuteCommand(createSecret)
				Expect(b).Should(BeTrue())
			}
		})
		It(" configure backup-repo", func() {
			repo := "kubectl get BackupRepo | grep my-repo | awk '{print $1}'"
			if checkResourceExists(repo) {
				log.Println("BackupRepo already exists")
			} else {
				var yaml string
				if ConfigType == "oss" {
					yaml = dir + "/testdata/config/backuprepo_oss.yaml"
				} else {
					yaml = dir + "/testdata/config/backuprepo_s3.yaml"
				}
				b := e2eutil.OpsYaml(yaml, "create")
				Expect(b).Should(BeTrue())
			}
		})
		It(" configure componentresourceconstraint custom", func() {
			componentResourceConstraint := "kubectl get ComponentResourceConstraint | grep kb-resource-constraint-e2e | awk '{print $1}'"
			if checkResourceExists(componentResourceConstraint) {
				log.Println("ComponentResourceConstraint already exists")
			} else {
				b := e2eutil.OpsYaml(dir+"/testdata/config/componentresourceconstraint_custom.yaml", "create")
				Expect(b).Should(BeTrue())
			}
		})
		It(" configure custom class", func() {
			componentClassDefinition := "kubectl get ComponentClassDefinition | grep custom-class | awk '{print $1}'"
			if checkResourceExists(componentClassDefinition) {
				log.Println("ComponentClassDefinition already exists")
			} else {
				b := e2eutil.OpsYaml(dir+"/testdata/config/custom_class.yaml", "create")
				Expect(b).Should(BeTrue())
			}
		})
		It(" configure pg cluster version", func() {
			clusterVersion := "kubectl get ClusterVersion | grep postgresql-14.7.2-latest | awk '{print $1}'"
			if checkResourceExists(clusterVersion) {
				log.Println("postgresql-14.7.2-latest clusterVersion already exists")
			} else {
				b := e2eutil.OpsYaml(dir+"/testdata/config/postgresql_cv.yaml", "create")
				Expect(b).Should(BeTrue())
			}
		})
	})
}

func DeleteConfig() {
	BeforeEach(func() {
	})

	AfterEach(func() {
	})

	Context("delete e2e test resources", func() {
		It("Check backup exists ", func() {
			backupArr := e2eutil.ExecCommandReadline("kubectl get backup | awk '{print $1}'")
			if len(backupArr) > 0 {
				deleteResource("kubectl delete backup --all")
			}
		})
		It("delete secret and backuprepo", func() {
			deleteResource("kubectl delete secret " + ConfigType + "-credential-for-backuprepo -n kb-system")
			deleteResource("kubectl delete backuprepo my-repo")
		})

		It("delete resources", func() {
			deleteResource("kubectl delete ComponentResourceConstraint kb-resource-constraint-e2e")
			deleteResource("kubectl delete ComponentClassDefinition custom-class")
		})

		It("delete cv", func() {
			deleteResource("kubectl delete ClusterVersion  postgresql-14.7.2-latest")
		})

		It("delete clusters", func() {
			deleteResource("kubectl delete cluster --all")
		})
	})
}

func checkResourceExists(command string) bool {
	if len(e2eutil.ExecCommand(command)) > 0 {
		return true
	}
	return false
}

func deleteResource(cmd string) {
	deleteCv := e2eutil.ExecuteCommand(cmd)
	Expect(deleteCv).Should(BeTrue())
}
