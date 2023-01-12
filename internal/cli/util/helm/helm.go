/*
Copyright ApeCloud Inc.

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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/containers/common/pkg/retry"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type InstallOpts struct {
	Name            string
	Chart           string
	Namespace       string
	Sets            []string
	Wait            bool
	Version         string
	TryTimes        int
	Login           bool
	CreateNamespace bool
}

type LoginOpts struct {
	User   string
	Passwd string
	URL    string
}

type Option func(*cli.EnvSettings)

func WithContext(context string) Option {
	return func(es *cli.EnvSettings) {
		es.KubeContext = context
	}
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
	if err = yaml.Unmarshal(b, &f); err != nil {
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
	if err = yaml.Unmarshal(b, &f); err != nil {
		return err
	}

	if f.Has(r.Name) {
		f.Remove(r.Name)
		if err = f.WriteFile(repoFile, 0644); err != nil {
			return err
		}
	}
	return nil
}

// getInstalled get helm package if installed.
func (i *InstallOpts) getInstalled(cfg *action.Configuration) (*release.Release, error) {
	res, err := action.NewGet(cfg).Run(i.Name)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, driver.ErrReleaseNotFound
	}
	if !statusDeployed(res) {
		return nil, ErrReleaseNotDeployed
	}
	return res, nil
}

// Install will install a Chart
func (i *InstallOpts) Install(cfg *action.Configuration) (string, error) {
	ctx := context.Background()
	opts := retry.Options{
		MaxRetry: 1 + i.TryTimes,
	}

	spinner := util.Spinner(os.Stdout, "Install %s", i.Chart)
	defer spinner(false)

	var notes string
	if err := retry.IfNecessary(ctx, func() error {
		var err1 error
		if notes, err1 = i.tryInstall(cfg); err1 != nil {
			return err1
		}
		return nil
	}, &opts); err != nil {
		return "", errors.Errorf("install chart %s error: %s", i.Name, err.Error())
	}

	spinner(true)
	return notes, nil
}

func (i *InstallOpts) tryInstall(cfg *action.Configuration) (string, error) {
	var err error

	if _, err = i.getInstalled(cfg); err != nil && !releaseNotFound(err) {
		return "", err
	}

	settings := cli.New()

	// TODO: Does not work now
	// If a release does not exist, install it.
	histClient := action.NewHistory(cfg)
	histClient.Max = 1
	if _, err := histClient.Run(i.Name); err != nil && err != driver.ErrReleaseNotFound {
		return "", err
	}

	client := action.NewInstall(cfg)
	client.ReleaseName = i.Name
	client.Namespace = i.Namespace
	client.CreateNamespace = i.CreateNamespace
	client.Wait = i.Wait
	client.Timeout = time.Second * 300
	client.Version = i.Version

	cp, err := client.ChartPathOptions.LocateChart(i.Chart, settings)
	if err != nil {
		return "", err
	}

	setOpts := values.Options{
		Values: i.Sets,
	}

	p := getter.All(settings)
	vals, err := setOpts.MergeValues(p)
	if err != nil {
		return "", err
	}

	// Check Chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(cp)
	if err != nil {
		return "", err
	}

	// Create context and prepare the handle of SIGTERM
	ctx := context.Background()
	_, cancel := context.WithCancel(ctx)

	// Set up channel on which to send signal notifications.
	// We must use a buffered channel or risk missing the signal
	// if we're not ready to receive when the signal is sent.
	cSignal := make(chan os.Signal, 2)
	signal.Notify(cSignal, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-cSignal
		fmt.Println("Install has been cancelled")
		cancel()
	}()

	release, err := client.RunWithContext(ctx, chartRequested, vals)
	if err != nil && err.Error() != "cannot re-use a name that is still in use" {
		return "", err
	}
	return release.Info.Notes, nil
}

// UnInstall will uninstall a Chart
func (i *InstallOpts) UnInstall(cfg *action.Configuration) error {
	ctx := context.Background()
	opts := retry.Options{
		MaxRetry: 1 + i.TryTimes,
	}

	spinner := util.Spinner(os.Stdout, "Uninstall %s", i.Name)
	defer spinner(false)
	if err := retry.IfNecessary(ctx, func() error {
		if err := i.tryUnInstall(cfg); err != nil {
			return err
		}
		return nil
	}, &opts); err != nil {
		return errors.Errorf("uninstall chart %s error: %s", i.Name, err.Error())
	}

	spinner(true)
	return nil
}

func (i *InstallOpts) tryUnInstall(cfg *action.Configuration) error {
	client := action.NewUninstall(cfg)
	client.Wait = i.Wait
	client.Timeout = time.Second * 300

	// Create context and prepare the handle of SIGTERM
	ctx := context.Background()
	_, cancel := context.WithCancel(ctx)

	// Set up channel on which to send signal notifications.
	// We must use a buffered channel or risk missing the signal
	// if we're not ready to receive when the signal is sent.
	cSignal := make(chan os.Signal, 2)
	signal.Notify(cSignal, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-cSignal
		fmt.Println("Install has been cancelled")
		cancel()
	}()

	if _, err := client.Run(i.Name); err != nil {
		return err
	}
	return nil
}

func NewActionConfig(ns string, config string, opts ...Option) (*action.Configuration, error) {
	var err error
	settings := cli.New()
	cfg := new(action.Configuration)

	settings.SetNamespace(ns)
	settings.KubeConfig = config
	for _, opt := range opts {
		opt(settings)
	}
	if cfg.RegistryClient, err = registry.NewClient(
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptEnableCache(true),
		registry.ClientOptWriter(io.Discard),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	); err != nil {
		return nil, err
	}
	if err = cfg.Init(settings.RESTClientGetter(), settings.Namespace(),
		os.Getenv("HELM_DRIVER"),
		func(format string, v ...interface{}) {}); err != nil {
		return nil, err
	}
	return cfg, nil
}

func FakeActionConfig() *action.Configuration {
	registryClient, err := registry.NewClient()
	if err != nil {
		return nil
	}

	return &action.Configuration{
		Releases:       storage.Init(driver.NewMemory()),
		KubeClient:     &kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}},
		Capabilities:   chartutil.DefaultCapabilities,
		RegistryClient: registryClient,
		Log:            func(format string, v ...interface{}) {},
	}
}

// Upgrade will upgrade a Chart
func (i *InstallOpts) Upgrade(cfg *action.Configuration) (string, error) {
	ctx := context.Background()
	opts := retry.Options{
		MaxRetry: 1 + i.TryTimes,
	}

	spinner := util.Spinner(os.Stdout, "Upgrade %s", i.Chart)
	defer spinner(false)

	var notes string
	if err := retry.IfNecessary(ctx, func() error {
		var err1 error
		if notes, err1 = i.tryUpgrade(cfg); err1 != nil {
			return err1
		}
		return nil
	}, &opts); err != nil {
		return "", fmt.Errorf("Upgrade chart %s error: %w", i.Name, err)
	}

	spinner(true)
	return notes, nil
}

func (i *InstallOpts) tryUpgrade(cfg *action.Configuration) (string, error) {
	installed, err := i.getInstalled(cfg)
	if err != nil {
		return "", err
	}

	settings := cli.New()

	client := action.NewUpgrade(cfg)
	client.Namespace = i.Namespace
	client.Wait = i.Wait
	client.Timeout = time.Second * 300
	if len(i.Version) > 0 {
		client.Version = i.Version
	} else {
		client.Version = installed.Chart.AppVersion()
	}
	client.ReuseValues = true

	cp, err := client.ChartPathOptions.LocateChart(i.Chart, settings)
	if err != nil {
		return "", err
	}

	setOpts := values.Options{
		Values: i.Sets,
	}

	p := getter.All(settings)
	vals, err := setOpts.MergeValues(p)
	if err != nil {
		return "", err
	}

	// Check Chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(cp)
	if err != nil {
		return "", err
	}

	// Create context and prepare the handle of SIGTERM
	ctx := context.Background()
	_, cancel := context.WithCancel(ctx)

	// Set up channel on which to send signal notifications.
	// We must use a buffered channel or risk missing the signal
	// if we're not ready to receive when the signal is sent.
	cSignal := make(chan os.Signal, 2)
	signal.Notify(cSignal, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-cSignal
		fmt.Println("Upgrade has been cancelled")
		cancel()
	}()

	// update crds before helm upgrade
	for _, obj := range chartRequested.CRDObjects() {
		// Read in the resources
		target, err := cfg.KubeClient.Build(bytes.NewBuffer(obj.File.Data), false)
		if err != nil {
			return "", errors.Wrapf(err, "failed to update CRD %s", obj.Name)
		}

		//var originalCrd *chart.CRD
		//for _, oldObj := range installed.Chart.CRDObjects() {
		//	if oldObj.Name == obj.Name {
		//		originalCrd = &oldObj
		//		break
		//	}
		//}
		//if originalCrd == nil {
		//	if _, err := cfg.KubeClient.Create(target); err != nil {
		//		if apierrors.IsAlreadyExists(err) {
		//			crdName := target[0].Name
		//			cfg.Log("CRD %s is already present. Skipping.", crdName)
		//			continue
		//		}
		//		return "", errors.Wrapf(err, "failed to update CRD %s", obj.Name)
		//	}
		//	continue
		//}
		//original, err := cfg.KubeClient.Build(bytes.NewBuffer(originalCrd.File.Data), false)
		//if err != nil {
		//	return "", errors.Wrapf(err, "failed to update CRDs")
		//}

		// Send them to Kube
		if _, err := cfg.KubeClient.Update(target, target, false); err != nil {
			return "", errors.Wrapf(err, "failed to update CRD %s", obj.Name)
		}
	}

	released, err := client.RunWithContext(ctx, i.Name, chartRequested, vals)
	if err != nil {
		return "", err
	}
	return released.Info.Notes, nil
}
