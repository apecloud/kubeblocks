package dbaas

import (
	"context"
	"github.com/apecloud/kubeblocks/pkg/utils"
	"github.com/apecloud/kubeblocks/pkg/utils/helm"
	"github.com/containers/common/pkg/retry"
	config "github.com/k3d-io/k3d/v5/pkg/config/v1alpha4"
	"github.com/pkg/errors"
	"sync"
	"time"
)

// Installer will handle the playground cluster creation and management
type Installer struct {
	cfg config.ClusterConfig

	Ctx         context.Context
	Namespace   string
	KubeConfig  string
	ClusterName string
	DBCluster   string
}

func (d *Installer) InstallDeps() error {
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
