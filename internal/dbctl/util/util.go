/*
Copyright ApeCloud Inc.

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
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"github.com/gosuri/uitable"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

var (
	green = color.New(color.FgHiGreen, color.Bold).SprintFunc()
	red   = color.New(color.FgHiRed, color.Bold).SprintFunc()
)

func init() {
	if _, err := GetCliHomeDir(); err != nil {
		fmt.Println("Failed to create dbctl home dir:", err)
	}
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
		if err = os.MkdirAll(cliHome, 0750); err != nil {
			return "", errors.Wrap(err, "error when create dbctl home directory")
		}
	}
	return cliHome, nil
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

func PrintObjYAML(obj *unstructured.Unstructured) error {
	data, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
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

func GenRequestID() string {
	return uuid.New().String()
}

func PrintGoTemplate(wr io.Writer, tpl string, values interface{}) error {
	tmpl, err := template.New("output").Parse(tpl)
	if err != nil {
		return err
	}

	err = tmpl.Execute(wr, values)
	if err != nil {
		return err
	}
	return nil
}

// SetKubeConfig set KUBECONFIG environment
func SetKubeConfig(cfg string) error {
	return os.Setenv("KUBECONFIG", cfg)
}

func Spinner(w io.Writer, fmtstr string, a ...any) func(result bool) {
	msg := fmt.Sprintf(fmtstr, a...)
	var once sync.Once
	var s *spinner.Spinner

	if runtime.GOOS == types.GoosWindows {
		fmt.Fprintf(w, "%s\n", msg)
		return func(result bool) {}
	} else {
		s = spinner.New(spinner.CharSets[11], 100*time.Millisecond)
		s.Writer = w
		_ = s.Color("cyan")
		s.Suffix = fmt.Sprintf("  %s", msg)
		s.Start()
	}

	return func(result bool) {
		once.Do(func() {
			if s != nil {
				s.Stop()
			}
			if result {
				fmt.Fprintf(w, "%s %s\n", msg, green("OK"))
			} else {
				fmt.Fprintf(w, "%s %s\n", msg, red("FAIL"))
			}
		})
	}
}

var addToScheme sync.Once

func NewFactory() cmdutil.Factory {
	getter := genericclioptions.NewConfigFlags(true)

	// Add CRDs to the scheme. They are missing by default.
	addToScheme.Do(func() {
		if err := apiextv1.AddToScheme(scheme.Scheme); err != nil {
			// This should never happen.
			panic(err)
		}
	})
	return cmdutil.NewFactory(getter)
}

func PlaygroundDir() (string, error) {
	cliPath, err := GetCliHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(cliPath, "playground"), nil
}

func GVRToString(gvr schema.GroupVersionResource) string {
	return strings.Join([]string{gvr.Resource, gvr.Version, gvr.Group}, ".")
}

// CheckErr prints a user-friendly error to STDERR and exits with a non-zero exit code.
func CheckErr(err error) {
	// unwrap aggregates of 1
	if agg, ok := err.(utilerrors.Aggregate); ok && len(agg.Errors()) == 1 {
		err = agg.Errors()[0]
	}

	if err == nil {
		return
	}

	// ErrExit and other valid api errors will be checked by cmdutil.CheckErr, now
	// we only check invalid api errors that can not be converted to StatusError.
	if err != cmdutil.ErrExit && apierrors.IsInvalid(err) {
		if _, ok := err.(*apierrors.StatusError); !ok {
			printErr(err)
			os.Exit(cmdutil.DefaultErrorExitCode)
		}
	}

	cmdutil.CheckErr(err)
}

func printErr(err error) {
	msg, ok := cmdutil.StandardErrorMessage(err)
	if !ok {
		msg = err.Error()
		if !strings.HasPrefix(msg, "error: ") {
			msg = fmt.Sprintf("error: %s", msg)
		}
	}
	if len(msg) > 0 {
		// add newline if needed
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		fmt.Fprint(os.Stderr, msg)
	}
}

func PrintTable(out io.Writer, table *uitable.Table) error {
	raw := table.Bytes()
	raw = append(raw, []byte("\n")...)
	_, err := out.Write(raw)
	if err != nil {
		return errors.Wrap(err, "unable to write table output")
	}
	return nil
}

// GetNodeByName choose node by name from a node array
func GetNodeByName(nodes []*corev1.Node, name string) *corev1.Node {
	for _, node := range nodes {
		if node.Name == name {
			return node
		}
	}
	return nil
}

// ResourceIsEmpty check if resource is empty or not
func ResourceIsEmpty(res *resource.Quantity) bool {
	resStr := res.String()
	if resStr == "0" || resStr == "<nil>" {
		return true
	}
	return false
}

func GetPodStatus(pods []*corev1.Pod) (running, waiting, succeeded, failed int) {
	for _, pod := range pods {
		switch pod.Status.Phase {
		case corev1.PodRunning:
			running++
		case corev1.PodPending:
			waiting++
		case corev1.PodSucceeded:
			succeeded++
		case corev1.PodFailed:
			failed++
		}
	}
	return
}
