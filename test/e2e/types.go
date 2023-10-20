/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
var Version string
var Provider string
var Region string
var SecretID string
var SecretKey string
var InitEnv bool
var TestType string
var SkipCase string
var ConfigType string
var TestResults []Result

type Result struct {
	CaseName        string
	ExecuteResult   bool
	TroubleShooting string
}

func NewResult(name string, result bool, troubleShooting string) Result {
	return Result{
		CaseName:        name,
		ExecuteResult:   result,
		TroubleShooting: troubleShooting,
	}
}
