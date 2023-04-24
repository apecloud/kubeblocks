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

package helm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/containers/common/pkg/retry"
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/helmpath"
	kubefake "helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"

	"github.com/apecloud/kubeblocks/internal/cli/types"
)

const defaultTimeout = time.Second * 600

type InstallOpts struct {
	Name            string
	Chart           string
	Namespace       string
	Wait            bool
	Version         string
	TryTimes        int
	Login           bool
	CreateNamespace bool
	ValueOpts       *values.Options
	Timeout         time.Duration
	Atomic          bool
	DisableHooks    bool
}

type Option func(*cli.EnvSettings)

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
		return errors.Errorf("repository name (%s) contains '/', please specify a different name without '/'", r.Name)
	}

	if f.Has(r.Name) {
		existing := f.Get(r.Name)
		if *r != *existing && r.Name != types.KubeBlocksChartName {
			// The input coming in for the Name is different from what is already
			// configured. Return an error.
			return errors.Errorf("repository name (%s) already exists, please specify a different name", r.Name)
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

	if err = f.WriteFile(repoFile, 0644); err != nil {
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

// GetInstalled get helm package release info if installed.
func (i *InstallOpts) GetInstalled(cfg *action.Configuration) (*release.Release, error) {
	res, err := action.NewGet(cfg).Run(i.Name)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, driver.ErrReleaseNotFound
	}
	if !statusDeployed(res) {
		return nil, errors.Wrapf(ErrReleaseNotDeployed, "current version not in right status, try to fix it first, \n"+
			"uninstall and install kubeblocks could be a way to fix error")
	}
	return res, nil
}

// Install will install a Chart
func (i *InstallOpts) Install(cfg *Config) (string, error) {
	ctx := context.Background()
	opts := retry.Options{
		MaxRetry: 1 + i.TryTimes,
	}

	actionCfg, err := NewActionConfig(cfg)
	if err != nil {
		return "", err
	}

	var notes string
	if err := retry.IfNecessary(ctx, func() error {
		var err1 error
		if notes, err1 = i.tryInstall(actionCfg); err1 != nil {
			return err1
		}
		return nil
	}, &opts); err != nil {
		return "", errors.Errorf("install chart %s error: %s", i.Name, err.Error())
	}

	return notes, nil
}

func (i *InstallOpts) tryInstall(cfg *action.Configuration) (string, error) {
	released, err := i.GetInstalled(cfg)
	if released != nil {
		return released.Info.Notes, nil
	}
	if err != nil && !releaseNotFound(err) {
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
	client.WaitForJobs = i.Wait
	client.Timeout = i.Timeout
	client.Version = i.Version
	client.Atomic = i.Atomic

	if client.Timeout == 0 {
		client.Timeout = defaultTimeout
	}

	cp, err := client.ChartPathOptions.LocateChart(i.Chart, settings)
	if err != nil {
		return "", err
	}

	p := getter.All(settings)
	vals, err := i.ValueOpts.MergeValues(p)
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

	released, err = client.RunWithContext(ctx, chartRequested, vals)
	if err != nil {
		return "", err
	}
	return released.Info.Notes, nil
}

// Uninstall will uninstall a Chart
func (i *InstallOpts) Uninstall(cfg *Config) error {
	ctx := context.Background()
	opts := retry.Options{
		MaxRetry: 1 + i.TryTimes,
	}
	if cfg.Namespace() == "" {
		cfg.SetNamespace(i.Namespace)
	}

	actionCfg, err := NewActionConfig(cfg)
	if err != nil {
		return err
	}

	if err := retry.IfNecessary(ctx, func() error {
		if err := i.tryUninstall(actionCfg); err != nil {
			return err
		}
		return nil
	}, &opts); err != nil {
		return err
	}
	return nil
}

func (i *InstallOpts) tryUninstall(cfg *action.Configuration) error {
	client := action.NewUninstall(cfg)
	client.Wait = i.Wait
	client.Timeout = defaultTimeout
	client.DisableHooks = i.DisableHooks

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

func NewActionConfig(cfg *Config) (*action.Configuration, error) {
	if cfg.fake {
		return fakeActionConfig(), nil
	}

	var err error
	settings := cli.New()
	actionCfg := new(action.Configuration)
	settings.SetNamespace(cfg.namespace)
	settings.KubeConfig = cfg.kubeConfig
	if cfg.kubeContext != "" {
		settings.KubeContext = cfg.kubeContext
	}
	settings.Debug = cfg.debug

	if actionCfg.RegistryClient, err = registry.NewClient(
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptEnableCache(true),
		registry.ClientOptWriter(io.Discard),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	); err != nil {
		return nil, err
	}

	// do not output warnings
	getter := settings.RESTClientGetter()
	getter.(*genericclioptions.ConfigFlags).WrapConfigFn = func(c *rest.Config) *rest.Config {
		c.WarningHandler = rest.NoWarnings{}
		return c
	}

	if err = actionCfg.Init(settings.RESTClientGetter(),
		settings.Namespace(),
		os.Getenv("HELM_DRIVER"),
		cfg.logFn); err != nil {
		return nil, err
	}
	return actionCfg, nil
}

func fakeActionConfig() *action.Configuration {
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
func (i *InstallOpts) Upgrade(cfg *Config) error {
	ctx := context.Background()
	opts := retry.Options{
		MaxRetry: 1 + i.TryTimes,
	}

	actionCfg, err := NewActionConfig(cfg)
	if err != nil {
		return err
	}

	if err = retry.IfNecessary(ctx, func() error {
		var err1 error
		if _, err1 = i.tryUpgrade(actionCfg); err1 != nil {
			return err1
		}
		return nil
	}, &opts); err != nil {
		return err
	}

	return nil
}

func (i *InstallOpts) tryUpgrade(cfg *action.Configuration) (string, error) {
	installed, err := i.GetInstalled(cfg)
	if err != nil {
		return "", err
	}

	settings := cli.New()

	client := action.NewUpgrade(cfg)
	client.Namespace = i.Namespace
	client.Wait = i.Wait
	client.WaitForJobs = i.Wait
	client.Timeout = i.Timeout
	if client.Timeout == 0 {
		client.Timeout = defaultTimeout
	}

	if len(i.Version) > 0 {
		client.Version = i.Version
	} else {
		client.Version = installed.Chart.AppVersion()
	}
	// do not use helm's ReuseValues, do it ourselves, helm's default upgrade also set it to false
	// if ReuseValues set to true, helm will use old values instead of new ones, which will cause nil pointer error if new values added.
	client.ReuseValues = false

	cp, err := client.ChartPathOptions.LocateChart(i.Chart, settings)
	if err != nil {
		return "", err
	}

	p := getter.All(settings)
	vals, err := i.ValueOpts.MergeValues(p)
	if err != nil {
		return "", err
	}
	// get coalesced values of current chart
	currentValues, err := chartutil.CoalesceValues(installed.Chart, installed.Config)
	if err != nil {
		return "", err
	}
	// merge current values into vals, so current release's user values can be kept
	installed.Chart.Values = currentValues
	vals, err = chartutil.CoalesceValues(installed.Chart, vals)
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

		// helm only use the original.Info part for looking up original CRD in Update interface
		// so set original with target as they have same .Info part
		original := target
		if _, err := cfg.KubeClient.Update(original, target, false); err != nil {
			return "", errors.Wrapf(err, "failed to update CRD %s", obj.Name)
		}
	}

	released, err := client.RunWithContext(ctx, i.Name, chartRequested, vals)
	if err != nil {
		return "", err
	}
	return released.Info.Notes, nil
}

func GetChartVersions(chartName string) ([]*semver.Version, error) {
	settings := cli.New()
	rf, err := repo.LoadFile(settings.RepositoryConfig)
	if err != nil {
		if os.IsNotExist(errors.Cause(err)) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	var ind *repo.IndexFile
	for _, re := range rf.Repositories {
		n := re.Name
		if n != types.KubeBlocksRepoName {
			continue
		}

		// load index file
		f := filepath.Join(settings.RepositoryCache, helmpath.CacheIndexFile(n))
		ind, err = repo.LoadIndexFile(f)
		if err != nil {
			return nil, err
		}
		break
	}

	// do not find any index file
	if ind == nil {
		return nil, nil
	}

	var versions []*semver.Version
	for chart, entry := range ind.Entries {
		if len(entry) == 0 || chart != chartName {
			continue
		}
		for _, v := range entry {
			ver, err := semver.NewVersion(v.Version)
			if err != nil {
				return nil, err
			}
			versions = append(versions, ver)
		}
	}
	return versions, nil
}

// AddValueOptionsFlags add helm value flags
func AddValueOptionsFlags(f *pflag.FlagSet, v *values.Options) {
	f.StringSliceVarP(&v.ValueFiles, "values", "f", []string{}, "Specify values in a YAML file or a URL (can specify multiple)")
	f.StringArrayVar(&v.Values, "set", []string{}, "Set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&v.StringValues, "set-string", []string{}, "Set STRING values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	f.StringArrayVar(&v.FileValues, "set-file", []string{}, "Set values from respective files specified via the command line (can specify multiple or separate values with commas: key1=path1,key2=path2)")
	f.StringArrayVar(&v.JSONValues, "set-json", []string{}, "Set JSON values on the command line (can specify multiple or separate values with commas: key1=jsonval1,key2=jsonval2)")
}

func ValueOptsIsEmpty(valueOpts *values.Options) bool {
	if valueOpts == nil {
		return true
	}
	return len(valueOpts.ValueFiles) == 0 &&
		len(valueOpts.StringValues) == 0 &&
		len(valueOpts.Values) == 0 &&
		len(valueOpts.FileValues) == 0 &&
		len(valueOpts.JSONValues) == 0
}

func GetQuiteLog() action.DebugLog {
	return func(format string, v ...interface{}) {}
}

func GetVerboseLog(out io.Writer) action.DebugLog {
	return func(format string, v ...interface{}) {
		fmt.Fprintf(out, format+"\n", v...)
	}
}
