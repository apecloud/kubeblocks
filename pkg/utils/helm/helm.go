/*
Copyright © 2022 The OpenCli Authors

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

package helm

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"

	"jihulab.com/infracreate/dbaas-system/opencli/pkg/utils"
)

var settings = cli.New()

type RepoEntry struct {
	Name string
	Url  string
}

type InstallOpts struct {
	Name      string
	Chart     string
	Namespace string
	Sets      []string
	Wait      bool
}

func debug(format string, v ...interface{}) {
	if settings.Debug {
		format = fmt.Sprintf("[debug] %s\n", format)
		utils.Debugf(fmt.Sprintf(format, v...))
	}
}
func SetKubeconfig(config string) {
	settings.KubeConfig = config
}

func SetNamespace(ns string) {
	settings.SetNamespace(ns)
}

// Add will add a repo
func (r *RepoEntry) Add() error {
	repoFile := settings.RepositoryConfig
	b, err := ioutil.ReadFile(repoFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var f repo.File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return err
	}

	c := repo.Entry{
		Name: r.Name,
		URL:  r.Url,
	}

	// Check if the repo Name is legal
	if strings.Contains(r.Name, "/") {
		return errors.Errorf("repository Name (%s) contains '/', please specify a different Name without '/'", r.Name)
	}

	if f.Has(r.Name) {
		existing := f.Get(r.Name)
		if c != *existing {

			// The input coming in for the Name is different from what is already
			// configured. Return an error.
			return errors.Errorf("repository Name (%s) already exists, please specify a different Name", r.Name)
		}

		// The add is idempotent so do nothing
		return nil
	}

	cp, err := repo.NewChartRepository(&c, getter.All(settings))
	if err != nil {
		return err
	}

	if _, err := cp.DownloadIndexFile(); err != nil {
		return errors.Wrapf(err, "looks like %q is not a valid Chart repository or cannot be reached", r.Url)
	}

	f.Update(&c)

	if err := f.WriteFile(repoFile, 0644); err != nil {
		return err
	}
	utils.Infof("%s has been added to your repositories\n", r.Name)
	return nil
}

// Install will install a Chart
func (i *InstallOpts) Install() (*release.Release, error) {
	actionConfig := new(action.Configuration)
	helmDriver := os.Getenv("HELM_DRIVER")
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), helmDriver, debug); err != nil {
		return nil, err
	}

	client := action.NewInstall(actionConfig)
	client.ReleaseName = i.Name

	cp, err := client.ChartPathOptions.LocateChart(i.Chart, settings)
	if err != nil {
		return nil, err
	}

	setOpts := values.Options{
		Values: i.Sets,
	}

	p := getter.All(settings)
	vals, err := setOpts.MergeValues(p)
	if err != nil {
		return nil, err
	}

	// Check Chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(cp)
	if err != nil {
		return nil, err
	}

	if i.Namespace != "" {
		client.Namespace = i.Namespace
		client.CreateNamespace = true
	} else {
		client.Namespace = settings.Namespace()
	}
	client.Wait = i.Wait

	res, err := client.Run(chartRequested, vals)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func NewActionConfig() (*action.Configuration, error) {
	cfg := new(action.Configuration)
	err := cfg.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), debug)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
