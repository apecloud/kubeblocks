package e2e

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var K8sClient client.Client
var Ctx context.Context
var Cancel context.CancelFunc
var Logger logr.Logger
