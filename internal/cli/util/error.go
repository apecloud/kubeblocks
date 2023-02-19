/*
Copyright ApeCloud, Inc.

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
	"fmt"
	"os"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

var (
	invalidAuthAPIVersion     = "exec plugin: invalid apiVersion \"client.authentication.k8s.io/v1alpha1\""
	invalidAuthAPIVersionHint = "if you are using Amazon EKS, please update AWS CLI to the latest version and update the kubeconfig file for your cluster,\nrefer to https://docs.aws.amazon.com/eks/latest/userguide/create-kubeconfig.html"
)

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

	// check invalid authentication apiVersion and output hint message
	if err.Error() == invalidAuthAPIVersion {
		printErr(err)
		fmt.Fprintf(os.Stderr, "hint: %s\n", invalidAuthAPIVersionHint)
		os.Exit(cmdutil.DefaultErrorExitCode)
	}

	// check other errors
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
