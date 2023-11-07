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

package testutils

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/containers/common/pkg/retry"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const defaultTimeout = time.Second * 600

type Config struct {
	namespace   string
	kubeConfig  string
	debug       bool
	kubeContext string
	logFn       action.DebugLog
	fake        bool
}

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
	ForceUninstall  bool

	// for helm template
	DryRun     *bool
	OutputDir  string
	IncludeCRD bool
}

// AddRepo adds a repo
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
		if *r != *existing && r.Name != KubeBlocksChartName {
			// The input Name is different from the existing one, return an error
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

func statusDeployed(rl *release.Release) bool {
	if rl == nil {
		return false
	}
	return release.StatusDeployed == rl.Info.Status
}

// GetInstalled gets helm package release info if installed.
func (i *InstallOpts) GetInstalled(cfg *action.Configuration) (*release.Release, error) {
	var ErrReleaseNotDeployed = fmt.Errorf("release: not in deployed status")
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

// Install installs a Chart
func (i *InstallOpts) Install(cfg *Config) (*release.Release, error) {
	ctx := context.Background()
	opts := retry.Options{
		MaxRetry: 1 + i.TryTimes,
	}

	actionCfg, err := NewActionConfig(cfg)
	if err != nil {
		return nil, err
	}

	var rel *release.Release
	if err = retry.IfNecessary(ctx, func() error {
		release, err1 := i.tryInstall(actionCfg)
		if err1 != nil {
			return err1
		}
		rel = release
		return nil
	}, &opts); err != nil {
		return nil, errors.Errorf("install chart %s error: %s", i.Name, err.Error())
	}

	return rel, nil
}

func ReleaseNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, driver.ErrReleaseNotFound) ||
		strings.Contains(err.Error(), driver.ErrReleaseNotFound.Error())
}

func (i *InstallOpts) tryInstall(cfg *action.Configuration) (*release.Release, error) {
	if i.DryRun == nil || !*i.DryRun {
		released, err := i.GetInstalled(cfg)
		if released != nil {
			return released, nil
		}
		if err != nil && !ReleaseNotFound(err) {
			return nil, err
		}
	}
	settings := cli.New()

	// TODO: Does not work now
	// If a release does not exist, install it.
	histClient := action.NewHistory(cfg)
	histClient.Max = 1
	if _, err := histClient.Run(i.Name); err != nil &&
		!errors.Is(err, driver.ErrReleaseNotFound) {
		return nil, err
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

	// for helm template
	if i.DryRun != nil {
		client.DryRun = *i.DryRun
		client.OutputDir = i.OutputDir
		client.IncludeCRDs = i.IncludeCRD
		client.Replace = true
		client.ClientOnly = true
	}

	if client.Timeout == 0 {
		client.Timeout = defaultTimeout
	}

	cp, err := client.ChartPathOptions.LocateChart(i.Chart, settings)
	if err != nil {
		return nil, err
	}

	p := getter.All(settings)
	vals, err := i.ValueOpts.MergeValues(p)
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
	_, cancel := context.WithCancel(ctx)

	// Set up channel through which to send signal notifications.
	// We must use a buffered channel or risk missing the signal
	// if we're not ready to receive when the signal is sent.
	cSignal := make(chan os.Signal, 2)
	signal.Notify(cSignal, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-cSignal
		fmt.Println("Install has been cancelled")
		cancel()
	}()

	released, err := client.RunWithContext(ctx, chartRequested, vals)
	if err != nil {
		return nil, err
	}
	return released, nil
}

// Uninstall uninstalls a Chart
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

	// Set up channel through which to send signal notifications.
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
		if i.ForceUninstall {
			// Remove secrets left over when uninstalling kubeblocks, when addon CRD is uninstalled before kubeblocks.
			secretCount, errRemove := i.RemoveRemainSecrets(cfg)
			if secretCount == 0 {
				return err
			}
			if errRemove != nil {
				errMsg := fmt.Sprintf("failed to remove remain secrets, please remove them manually, %v", errRemove)
				return errors.Wrap(err, errMsg)
			}
		} else {
			return err
		}
	}
	return nil
}

func (i *InstallOpts) RemoveRemainSecrets(cfg *action.Configuration) (int, error) {
	clientSet, err := cfg.KubernetesClientSet()
	if err != nil {
		return -1, err
	}

	labelSelector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "name",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{i.Name},
			},
			{
				Key:      "owner",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"helm"},
			},
			{
				Key:      "status",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{"uninstalling", "superseded"},
			},
		},
	}

	selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
	if err != nil {
		fmt.Printf("Failed to build label selector: %v\n", err)
		return -1, err
	}
	options := metav1.ListOptions{
		LabelSelector: selector.String(),
	}

	secrets, err := clientSet.CoreV1().Secrets(i.Namespace).List(context.TODO(), options)
	if err != nil {
		return -1, err
	}
	secretCount := len(secrets.Items)
	if secretCount == 0 {
		return 0, nil
	}

	for _, secret := range secrets.Items {
		err := clientSet.CoreV1().Secrets(i.Namespace).Delete(context.TODO(), secret.Name, metav1.DeleteOptions{})
		if err != nil {
			klog.V(1).Info(err)
			return -1, fmt.Errorf("failed to delete Secret %s: %v", secret.Name, err)
		}
	}
	return secretCount, nil
}

func fakeActionConfig() *action.Configuration {
	registryClient, err := registry.NewClient()
	if err != nil {
		return nil
	}

	res := &action.Configuration{
		Releases:       storage.Init(driver.NewMemory()),
		KubeClient:     &kubefake.FailingKubeClient{PrintingKubeClient: kubefake.PrintingKubeClient{Out: io.Discard}},
		Capabilities:   chartutil.DefaultCapabilities,
		RegistryClient: registryClient,
		Log:            func(format string, v ...interface{}) {},
	}
	// to template the kubeblocks manifest, dry-run install will check and valida the KubeVersion in Capabilities is bigger than
	// the KubeVersion in Chart.yaml.
	// in helm v3.11.1 the DefaultCapabilities KubeVersion is 1.20 which lower than the kubeblocks Chart claimed '>=1.22.0-0'
	res.Capabilities.KubeVersion.Version = "v99.99.0"
	return res
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

func NewConfig(namespace string, kubeConfig string, ctx string, debug bool) *Config {
	cfg := &Config{
		namespace:   namespace,
		debug:       debug,
		kubeConfig:  kubeConfig,
		kubeContext: ctx,
	}

	if debug {
		cfg.logFn = GetVerboseLog()
	} else {
		cfg.logFn = GetQuiteLog()
	}
	return cfg
}
func (o *Config) SetNamespace(namespace string) {
	o.namespace = namespace
}

func (o *Config) Namespace() string {
	return o.namespace
}

func GetQuiteLog() action.DebugLog {
	return func(format string, v ...interface{}) {}
}

func GetVerboseLog() action.DebugLog {
	return func(format string, v ...interface{}) {
		klog.Infof(format+"\n", v...)
	}
}
