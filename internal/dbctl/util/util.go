/*
Copyright © 2022 The dbctl Authors

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

package util

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	"golang.org/x/crypto/ssh"

	"github.com/go-logr/logr"
	l "github.com/k3d-io/k3d/v5/pkg/logger"
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

var (
	// Info print message
	Info func(a ...interface{})
	// Infof print message with format
	Infof func(format string, a ...interface{})
	// InfoP print message with padding
	InfoP func(padding int, a ...interface{})
	// Errf print error with format
	Errf func(format string, a ...interface{})
	// Debugf print error with format
	Debugf func(format string, a ...interface{})
)

func init() {
	Info = func(a ...interface{}) {
		l.Log().Info(a...)
	}
	Infof = func(format string, a ...interface{}) {
		l.Log().Infof(format, a...)
	}
	InfoP = func(padding int, a ...interface{}) {
		l.Log().Infof(fmt.Sprintf("%*s %%v", padding, ""), a...)
	}
	Errf = func(format string, a ...interface{}) {
		l.Log().Errorf(format, a...)
	}
	Debugf = func(format string, a ...interface{}) {
		l.Log().Debugf(format, a...)
	}
	if _, err := GetCliHomeDir(); err != nil {
		l.Log().Error("Failed to create dbctl home dir:", err)
	}
}

// CleanUpPlayground removes the playground directory
func CleanUpPlayground() error {
	tempDir, err := GetTempDir()
	if err != nil {
		return err
	}
	return os.RemoveAll(tempDir)
}

// CloseQuietly closes `io.Closer` quietly. Very handy and helpful for code
// quality too.
func CloseQuietly(d io.Closer) {
	_ = d.Close()
}

// GetCliHomeDir return dbctl home dir
func GetCliHomeDir() (string, error) {
	var cliHome string
	if custom := os.Getenv(types.DBCtlHomeEnv); custom != "" {
		cliHome = custom
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cliHome = filepath.Join(home, types.DBCtlDefaultHome)
	}
	if _, err := os.Stat(cliHome); err != nil && os.IsNotExist(err) {
		err := os.MkdirAll(cliHome, 0750)
		if err != nil {
			return "", errors.Wrap(err, "error when create dbctl home directory")
		}
	}
	return cliHome, nil
}

func GetTempDir() (string, error) {
	dir, err := GetCliHomeDir()
	if err != nil {
		return "", err
	}
	tmpDir := filepath.Join(dir, "tmp")
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		return "", err
	}
	return tmpDir, nil
}

// GetKubeconfigDir returns the kubeconfig directory.
func GetKubeconfigDir() string {
	var kubeconfigDir string
	switch runtime.GOOS {
	case types.GoosDarwin, types.GoosLinux:
		kubeconfigDir = filepath.Join(os.Getenv("HOME"), ".kube")
	case types.GoosWindows:
		kubeconfigDir = filepath.Join(os.Getenv("USERPROFILE"), ".kube")
	}
	return kubeconfigDir
}

func ConfigPath(name string) string {
	if len(name) == 0 {
		return ""
	}

	return filepath.Join(GetKubeconfigDir(), name)
}

func RemoveConfig(name string) error {
	if err := os.Remove(ConfigPath(name)); err != nil {
		return err
	}
	return nil
}

func GetPublicIP() (string, error) {
	resp, err := http.Get("https://ifconfig.me")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// MakeSSHKeyPair make a pair of public and private keys for SSH access.
// Public key is encoded in the format for inclusion in an OpenSSH authorized_keys file.
// Private Key generated is PEM encoded
func MakeSSHKeyPair(pubKeyPath, privateKeyPath string) error {
	if err := os.MkdirAll(path.Dir(pubKeyPath), os.FileMode(0700)); err != nil {
		return err
	}
	if err := os.MkdirAll(path.Dir(privateKeyPath), os.FileMode(0700)); err != nil {
		return err
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	// generate and write private key as PEM
	privateKeyFile, err := os.OpenFile(privateKeyPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer privateKeyFile.Close()

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}
	return os.WriteFile(pubKeyPath, ssh.MarshalAuthorizedKey(pub), 0655)
}

func PrintObjYaml(obj *unstructured.Unstructured) {
	data, err := yaml.Marshal(obj)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println(string(data))
}

type RetryOptions struct {
	MaxRetry int
	Delay    time.Duration
}

func DoWithRetry(ctx context.Context, logger logr.Logger, operation func() error, options *RetryOptions) error {
	err := operation()
	for attempt := 0; err != nil && attempt < options.MaxRetry; attempt++ {
		delay := time.Duration(int(math.Pow(2, float64(attempt)))) * time.Second
		if options.Delay != 0 {
			delay = options.Delay
		}
		logger.Info(fmt.Sprintf("Failed, retrying in %s ... (%d/%d). Error: %v", delay, attempt+1, options.MaxRetry, err))
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return err
		}
		err = operation()
	}
	return err
}
