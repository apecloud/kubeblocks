/*
Copyright © 2022 The dbctl Authors

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

package provider

import (
	"helm.sh/helm/v3/pkg/repo"

	"github.com/apecloud/kubeblocks/pkg/utils/helm"
)

type DataProtection struct {
	ServerVersion string
	AccessKey     string
	SecretKey     string
	Region        string
	S3Endpoint    string
	S3Bucket      string
}

func (o *DataProtection) GetRepos() []repo.Entry {
	return []repo.Entry{
		{
			Name: "prometheus-community",
			URL:  "https://prometheus-community.github.io/helm-charts",
		},
		{
			Name: "mysql-operator",
			URL:  "https://mysql.github.io/mysql-operator/",
		},
		{
			Name: "piraeus-charts",
			URL:  "https://piraeus.io/helm-charts/",
		},
		{
			Name: "cloudve",
			URL:  "https://github.com/CloudVE/helm-charts/raw/master",
		},
	}
}

func (o *DataProtection) GetBaseCharts(ns string) []helm.InstallOpts {
	return []helm.InstallOpts{}
}

func (o *DataProtection) GetDBCharts(ns string, dbname string) []helm.InstallOpts {
	return []helm.InstallOpts{
		{
			Name:      "mysql-operator",
			Chart:     "mysql-operator/mysql-operator",
			Wait:      true,
			Version:   "2.0.6",
			Namespace: ns,
			Sets:      []string{},
			TryTimes:  2,
		},
		{
			Name:      dbname,
			Chart:     "oci://yimeisun.azurecr.io/helm-chart/mysql-innodbcluster",
			Wait:      true,
			Namespace: "default",
			Version:   "1.1.0",
			Sets: []string{
				"serverVersion=" + o.ServerVersion,
			},
			LoginOpts: &helm.LoginOpts{
				User:   "yimeisun",
				Passwd: "8V+PmX1oSDv4pumDvZp6m7LS8iPgbY3A",
				URL:    "yimeisun.azurecr.io",
			},
			TryTimes: 2,
		},
		{
			Name:      "snapshot-controller",
			Chart:     "piraeus-charts/snapshot-controller",
			Wait:      true,
			Version:   "1.5.1",
			Namespace: ns,
			Sets: []string{
				"image.repository=registry.aliyuncs.com/google_containers/snapshot-controller",
			},
			TryTimes: 2,
		},
		{
			Name:      "csi-s3",
			Chart:     "cloudve/csi-s3",
			Wait:      true,
			Version:   "0.31.3",
			Namespace: ns,
			Sets: []string{
				"secret.accessKey=" + o.AccessKey,
				"secret.secretKey=" + o.SecretKey,
				"secret.endpoint=" + o.S3Endpoint,
				"storageClass.singleBucket=" + o.S3Bucket,
				"storageClass.mountOptions='--memory-limit 1000 --dir-mode 0777 --file-mode 0666 --region " + o.Region + "'",
			},
			TryTimes: 2,
		},
	}
}
