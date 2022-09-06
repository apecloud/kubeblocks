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

package helm

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/briandowns/spinner"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/client-go/util/homedir"

	"github.com/apecloud/kubeblocks/pkg/utils"
)

type InstallOpts struct {
	Name      string
	Chart     string
	Namespace string
	Sets      []string
	Wait      bool
	Version   string
	TryTimes  int
	LoginOpts *LoginOpts
}

type LoginOpts struct {
	User   string
	Passwd string
	URL    string
}

// AddRepo will add a repo
func AddRepo(r *repo.Entry) error {
	settings := cli.New()
	repoFile := settings.RepositoryConfig
	b, err := os.ReadFile(repoFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var f repo.File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return err
	}

	// Check if the repo Name is legal
	if strings.Contains(r.Name, "/") {
		return errors.Errorf("repository Name (%s) contains '/', please specify a different Name without '/'", r.Name)
	}

	if f.Has(r.Name) {
		existing := f.Get(r.Name)
		if *r != *existing {

			// The input coming in for the Name is different from what is already
			// configured. Return an error.
			return errors.Errorf("repository Name (%s) already exists, please specify a different Name", r.Name)
		}

		// The add is idempotent so do nothing
		return nil
	}

	cp, err := repo.NewChartRepository(r, getter.All(settings))
	if err != nil {
		return err
	}

	if _, err := cp.DownloadIndexFile(); err != nil {
		return errors.Wrapf(err, "looks like %q is not a valid Chart repository or cannot be reached", r.URL)
	}

	f.Update(r)

	if err := f.WriteFile(repoFile, 0644); err != nil {
		return err
	}
	utils.Infof("%s has been added to your repositories\n", r.Name)
	return nil
}

// RemoveRepo will remove a repo
func RemoveRepo(r *repo.Entry) error {
	settings := cli.New()
	repoFile := settings.RepositoryConfig
	b, err := os.ReadFile(repoFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var f repo.File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return err
	}

	if f.Has(r.Name) {
		f.Remove(r.Name)
		if err := f.WriteFile(repoFile, 0644); err != nil {
			return err
		}
	}
	utils.Infof("%s has been remove to your repositories\n", r.Name)
	return nil
}

// GetInstalled get helm package if installed.
func (i *InstallOpts) GetInstalled(cfg string) (*release.Release, error) {
	settings := cli.New()
	actionConfig, err := NewActionConfig(settings, i.Namespace, cfg)
	if err != nil {
		return nil, err
	}
	getClient := action.NewGet(actionConfig)
	res, err := getClient.Run(i.Name)
	if err != nil {
		if strings.Contains(err.Error(), "release: not found") {
			return nil, nil
		}
		utils.Infof("Failed check %s installed\n", i.Name)
		return nil, err
	}
	return res, nil
}

// Install will install a Chart
func (i *InstallOpts) Install(cfg string) (*release.Release, error) {
	utils.InfoP(1, "Install "+i.Chart+"...")
	s := spinner.New(spinner.CharSets[rand.Intn(44)], 100*time.Millisecond)
	if err := s.Color("green"); err != nil {
		return nil, err
	}
	s.Start()
	defer s.Stop()

	res, _ := i.GetInstalled(cfg)
	if res != nil {
		return res, nil
	}

	settings := cli.New()
	actionConfig, err := NewActionConfig(settings, i.Namespace, cfg)
	if err != nil {
		return nil, err
	}

	err = i.TryToLogin(actionConfig)
	if err != nil {
		return nil, err
	}

	// TODO: Does not work now
	// If a release does not exist, install it.
	histClient := action.NewHistory(actionConfig)
	histClient.Max = 1
	if _, err := histClient.Run(i.Name); err != nil && err != driver.ErrReleaseNotFound {
		return nil, err
	}

	client := action.NewInstall(actionConfig)
	client.ReleaseName = i.Name
	client.Namespace = i.Namespace
	client.CreateNamespace = true
	client.Wait = i.Wait
	client.Timeout = time.Second * 300
	client.Version = i.Version
	client.Keyring = defaultKeyring()

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

	// Create context and prepare the handle of SIGTERM
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// Set up channel on which to send signal notifications.
	// We must use a buffered channel or risk missing the signal
	// if we're not ready to receive when the signal is sent.
	cSignal := make(chan os.Signal, 2)
	signal.Notify(cSignal, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-cSignal
		utils.Infof("Install has been cancelled.\n")
		cancel()
	}()

	res, err = client.RunWithContext(ctx, chartRequested, vals)
	if err != nil && err.Error() != "cannot re-use a name that is still in use" {
		return nil, err
	}
	return res, nil
}

func (i *InstallOpts) TryToLogin(cfg *action.Configuration) error {
	if i.LoginOpts == nil {
		return nil
	}

	return cfg.RegistryClient.Login(i.LoginOpts.URL, registry.LoginOptBasicAuth(i.LoginOpts.User, i.LoginOpts.Passwd),
		registry.LoginOptInsecure(false))
}

func NewActionConfig(s *cli.EnvSettings, ns string, config string) (*action.Configuration, error) {
	var settings = s
	cfg := new(action.Configuration)
	if settings == nil {
		settings = cli.New()
	}

	settings.SetNamespace(ns)
	settings.KubeConfig = config
	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptEnableCache(true),
		registry.ClientOptWriter(io.Discard),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	)
	if err != nil {
		return nil, err
	}
	cfg.RegistryClient = registryClient

	debug := func(format string, v ...interface{}) {
		if settings.Debug {
			format = fmt.Sprintf("[debug] %s\n", format)
			//nolint
			log.Output(2, fmt.Sprintf(format, v...))
		}
	}

	err = cfg.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), debug)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// defaultKeyring returns the expanded path to the default keyring.
func defaultKeyring() string {
	if v, ok := os.LookupEnv("GNUPGHOME"); ok {
		return filepath.Join(v, "pubring.gpg")
	}
	return filepath.Join(homedir.HomeDir(), ".gnupg", "pubring.gpg")
}

// Install will install a Chart
func (i *InstallOpts) UnInstall(cfg string) (*release.UninstallReleaseResponse, error) {
	utils.InfoP(1, "UnInstall "+i.Chart+"...")
	s := spinner.New(spinner.CharSets[rand.Intn(44)], 100*time.Millisecond)
	if err := s.Color("green"); err != nil {
		return nil, err
	}
	s.Start()
	defer s.Stop()

	settings := cli.New()
	actionConfig, err := NewActionConfig(settings, i.Namespace, cfg)
	if err != nil {
		return nil, err
	}

	err = i.TryToLogin(actionConfig)
	if err != nil {
		return nil, err
	}

	client := action.NewUninstall(actionConfig)
	client.Wait = i.Wait
	client.Timeout = time.Second * 300

	res, err := client.Run(i.Name)
	// ignore not found error
	if err != nil && !strings.Contains(err.Error(), "release: not found") {
		return nil, err
	}
	return res, nil
}
