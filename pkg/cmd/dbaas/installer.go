package dbaas

import (
	"context"
	"sync"
	"time"

	"github.com/apecloud/kubeblocks/pkg/utils"
	"github.com/apecloud/kubeblocks/pkg/utils/helm"
	"github.com/containers/common/pkg/retry"
	"github.com/pkg/errors"
)

// Installer will handle the playground cluster creation and management
type Installer struct {
	Ctx         context.Context
	Namespace   string
	KubeConfig  string
	ClusterName string
	DBCluster   string
}

func (d *Installer) Install() error {
	var err error

	var wg sync.WaitGroup
	err = installCharts(d, &wg)
	if err != nil {
		return errors.Wrap(err, "Failed to install dbaas")
	}
	utils.Info("Waiting for dbaas operator to be ready...")
	time.Sleep(10 * time.Second)
	wg.Wait()

	return nil
}

func installCharts(in *Installer, wg *sync.WaitGroup) error {
	install := func(cs []helm.InstallOpts, wg *sync.WaitGroup) error {
		ctx := context.Background()
		for _, c := range cs {
			opts := retry.Options{
				MaxRetry: 1 + c.TryTimes,
			}
			if err := retry.IfNecessary(ctx, func() error {
				if _, err := c.Install(utils.ConfigPath(in.KubeConfig)); err != nil {
					return err
				}
				return nil
			}, &opts); err != nil {
				return errors.Errorf("Install chart %s error: %s", c.Name, err.Error())
			}
		}
		return nil
	}

	utils.Info("Installing dbaas...")
	charts := []helm.InstallOpts{
		{
			Name:      "opendbaas-core",
			Chart:     "oci://yimeisun.azurecr.io/helm-chart/opendbaas-core",
			Wait:      true,
			Version:   "0.1.0-alpha.5",
			Namespace: "default",
			Sets: []string{
				"image.tag=latest",
				"image.pullPolicy=Always",
			},
			LoginOpts: &helm.LoginOpts{
				User:   helmUser,
				Passwd: helmPasswd,
				URL:    helmURL,
			},
			TryTimes: 2,
		},
	}
	err := install(charts, wg)
	if err != nil {
		return err
	}

	return nil
}

// Uninstall remove dbaas
func (in *Installer) Uninstall() error {

	uninstall := func(cs []helm.InstallOpts) error {
		ctx := context.Background()
		for _, c := range cs {
			opts := retry.Options{
				MaxRetry: 1 + c.TryTimes,
			}
			if err := retry.IfNecessary(ctx, func() error {
				if _, err := c.Uninstall(utils.ConfigPath(in.KubeConfig)); err != nil {
					return err
				}
				return nil
			}, &opts); err != nil {
				return errors.Errorf("Uninstall chart %s error: %s", c.Name, err.Error())
			}
		}
		return nil
	}

	utils.Info("Uninstalling dbaas...")
	charts := []helm.InstallOpts{
		{
			Name: "opendbaas-core",
		},
	}
	err := uninstall(charts)
	if err != nil {
		return err
	}

	return nil
}
