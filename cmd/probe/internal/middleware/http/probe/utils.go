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

package probe

import (
	"context"
	"os"
	"time"

	"github.com/go-errors/errors"

	"github.com/apecloud/kubeblocks/internal/constant"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	secretName     = "probe-token"
	maxRetryTime   = 10
	retryInterval  = 5 * time.Second
	tokenKeyInData = "token"
)

func getToken(ctx context.Context, log logr.Logger) (string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error(err, "get k8s client config failed")
		return "", err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error(err, "k8s client create failed")
		return "", err
	}

	namespace := os.Getenv(constant.KBEnvNamespace)

	// try best effort to get token
	var secret *corev1.Secret
	for i := 0; i < maxRetryTime; i++ {
		secret, err = clientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
		if err == nil {
			break
		}
		log.Error(err, "get secret key failed")
		time.Sleep(retryInterval)
	}
	if err != nil {
		return "", err
	}

	data, ok := secret.Data[tokenKeyInData]
	if !ok {
		return "", errors.Errorf("field %s missing in secret", tokenKeyInData)
	}

	return string(data), nil
}
