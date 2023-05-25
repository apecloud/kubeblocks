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
