package dbaas

import (
	"helm.sh/helm/v3/pkg/action"

	"github.com/apecloud/kubeblocks/pkg/utils/helm"
)

// Installer will handle the playground cluster creation and management
type Installer struct {
	cfg *action.Configuration

	Namespace string
	Version   string
}

func (i *Installer) Install() error {
	dbaasChart := helm.InstallOpts{
		Name:      "opendbaas-core",
		Chart:     "oci://yimeisun.azurecr.io/helm-chart/opendbaas-core",
		Wait:      true,
		Version:   i.Version,
		Namespace: i.Namespace,
		Sets: []string{
			"image.tag=latest",
			"image.pullPolicy=Always",
		},
		Login:    true,
		TryTimes: 2,
	}

	err := dbaasChart.Install(i.cfg)
	if err != nil {
		return err
	}

	return nil
}

// Uninstall remove dbaas
func (i *Installer) Uninstall() error {
	chart := helm.InstallOpts{
		Name: "opendbaas-core",
	}

	err := chart.UnInstall(i.cfg)
	if err != nil {
		return err
	}

	return nil
}
