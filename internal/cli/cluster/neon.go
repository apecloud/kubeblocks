package cluster

import (
	"embed"
)

var (
	// run `make generate` to generate this embed file
	//go:embed charts/neon-cluster.tgz
	neonChart embed.FS
)

func init() {
	registerClusterType("neon", neonChart, "neon-cluster.tgz", "neon")
}
