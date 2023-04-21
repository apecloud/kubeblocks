/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

/*
Copyright the Velero contributors.

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
	"io/ioutil"
	"os/exec"
	"time"

	"github.com/pkg/errors"
	"github.com/vmware-tanzu/velero/pkg/builder"
	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
	"github.com/vmware-tanzu/velero/test/e2e/util/common"
	"golang.org/x/net/context"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/constant"
)

func EnsureClusterExists(ctx context.Context) error {
	return exec.CommandContext(ctx, "kubectl", "cluster-info").Run()
}

func CreateSecretFromFiles(ctx context.Context, client TestClient, namespace string, name string, files map[string]string) error {
	data := make(map[string][]byte)

	for key, filePath := range files {
		contents, err := ioutil.ReadFile(filePath)
		if err != nil {
			return errors.WithMessagef(err, "Failed to read secret file %q", filePath)
		}

		data[key] = contents
	}
	secret := builder.ForSecret(namespace, name).Data(data).Result()
	_, err := client.ClientGo.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	return err
}

// WaitForPods waits until all of the pods have gone to PodRunning state
func WaitForPods(ctx context.Context, client TestClient, namespace string, pods []string) error {
	timeout := 5 * time.Minute
	interval := 5 * time.Second
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		for _, podName := range pods {
			checkPod, err := client.ClientGo.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
			if err != nil {
				fmt.Println(errors.Wrap(err, fmt.Sprintf("Failed to verify pod %s/%s is %s, try again...\n", namespace, podName, corev1api.PodRunning)))
				return false, nil
			}
			// If any pod is still waiting we don't need to check any more so return and wait for next poll interval
			if checkPod.Status.Phase != corev1api.PodRunning {
				fmt.Printf("Pod %s is in state %s waiting for it to be %s\n", podName, checkPod.Status.Phase, corev1api.PodRunning)
				return false, nil
			}
		}
		// All pods were in PodRunning state, we're successful
		return true, nil
	})
	if err != nil {
		return errors.Wrapf(err, fmt.Sprintf("Failed to wait for pods in namespace %s to start running", namespace))
	}
	return nil
}

func GetPvcByPodName(ctx context.Context, namespace, podName string) ([]string, error) {
	// Example:
	//    NAME                                  STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS             AGE
	//    kibishii-data-kibishii-deployment-0   Bound    pvc-94b9fdf2-c30f-4a7b-87bf-06eadca0d5b6   1Gi        RWO            kibishii-storage-class   115s
	cmds := []*common.OsCommandLine{}
	cmd := &common.OsCommandLine{
		Cmd:  "kubectl",
		Args: []string{"get", "pvc", "-n", namespace},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "grep",
		Args: []string{podName},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "awk",
		Args: []string{"{print $1}"},
	}
	cmds = append(cmds, cmd)

	return common.GetListByCmdPipes(ctx, cmds)
}

func GetPvByPvc(ctx context.Context, namespace, pvc string) ([]string, error) {
	// Example:
	// 	  NAME                                       CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM                                              STORAGECLASS             REASON   AGE
	//    pvc-3f784366-58db-40b2-8fec-77307807e74b   1Gi        RWO            Delete           Bound    bsl-deletion/kibishii-data-kibishii-deployment-0   kibishii-storage-class            6h41m
	cmds := []*common.OsCommandLine{}
	cmd := &common.OsCommandLine{
		Cmd:  "kubectl",
		Args: []string{"get", "pv"},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "grep",
		Args: []string{namespace + "/" + pvc},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "awk",
		Args: []string{"{print $1}"},
	}
	cmds = append(cmds, cmd)

	return common.GetListByCmdPipes(ctx, cmds)
}

func CRDShouldExist(ctx context.Context, name string) error {
	return CRDCountShouldBe(ctx, name, 1)
}

func CRDShouldNotExist(ctx context.Context, name string) error {
	return CRDCountShouldBe(ctx, name, 0)
}

func CRDCountShouldBe(ctx context.Context, name string, count int) error {
	crdList, err := GetCRD(ctx, name)
	if err != nil {
		return errors.Wrap(err, "Fail to get CRDs")
	}
	len := len(crdList)
	if len != count {
		return errors.New(fmt.Sprintf("CRD count is expected as %d instead of %d", count, len))
	}
	return nil
}

func GetCRD(ctx context.Context, name string) ([]string, error) {
	cmds := []*common.OsCommandLine{}
	cmd := &common.OsCommandLine{
		Cmd:  "kubectl",
		Args: []string{"get", "crd"},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "grep",
		Args: []string{name},
	}
	cmds = append(cmds, cmd)

	cmd = &common.OsCommandLine{
		Cmd:  "awk",
		Args: []string{"{print $1}"},
	}
	cmds = append(cmds, cmd)

	return common.GetListByCmdPipes(ctx, cmds)
}

func AddLabelToPv(ctx context.Context, pv, label string) error {
	return exec.CommandContext(ctx, "kubectl", "label", "pv", pv, label).Run()
}

func AddLabelToPvc(ctx context.Context, pvc, namespace, label string) error {
	args := []string{"label", "pvc", pvc, "-n", namespace, label}
	fmt.Println(args)
	return exec.CommandContext(ctx, "kubectl", args...).Run()
}

func AddLabelToPod(ctx context.Context, podName, namespace, label string) error {
	args := []string{"label", "pod", podName, "-n", namespace, label}
	fmt.Println(args)
	return exec.CommandContext(ctx, "kubectl", args...).Run()
}

func AddLabelToCRD(ctx context.Context, crd, label string) error {
	args := []string{"label", "crd", crd, label}
	fmt.Println(args)
	return exec.CommandContext(ctx, "kubectl", args...).Run()
}

func KubectlApplyByFile(ctx context.Context, file string) error {
	args := []string{"apply", "-f", file, "--force=true"}
	return exec.CommandContext(ctx, "kubectl", args...).Run()
}

func KubectlConfigUseContext(ctx context.Context, kubectlContext string) error {
	cmd := exec.CommandContext(ctx, "kubectl",
		"config", "use-context", kubectlContext)
	fmt.Printf("Kubectl config use-context cmd =%v\n", cmd)
	stdout, stderr, err := veleroexec.RunCommand(cmd)
	fmt.Print(stdout)
	fmt.Print(stderr)
	return err
}

func GetAPIVersions(client *TestClient, name string) ([]string, error) {
	var version []string
	APIGroup, err := client.ClientGo.Discovery().ServerGroups()
	if err != nil {
		return nil, errors.Wrap(err, "Fail to get server API groups")
	}
	for _, group := range APIGroup.Groups {
		fmt.Println(group.Name)
		if group.Name == name {
			for _, v := range group.Versions {
				fmt.Println(v.Version)
				version = append(version, v.Version)
			}
			return version, nil
		}
	}
	return nil, errors.New("Server API groups is empty")
}

func CreateFileToPod(ctx context.Context, namespace, podName, volume, filename, content string) error {
	arg := []string{"exec", "-n", namespace, "-c", podName, podName,
		"--", "/bin/sh", "-c", fmt.Sprintf("echo ns-%s pod-%s volume-%s  > /%s/%s", namespace, podName, volume, volume, filename)}
	cmd := exec.CommandContext(ctx, "kubectl", arg...)
	fmt.Printf("Kubectl exec cmd =%v\n", cmd)
	return cmd.Run()
}
func ReadFileFromPodVolume(ctx context.Context, namespace, podName, volume, filename string) (string, error) {
	arg := []string{"exec", "-n", namespace, "-c", podName, podName,
		"--", "cat", fmt.Sprintf("/%s/%s", volume, filename)}
	cmd := exec.CommandContext(ctx, "kubectl", arg...)
	fmt.Printf("Kubectl exec cmd =%v\n", cmd)
	stdout, stderr, err := veleroexec.RunCommand(cmd)
	fmt.Print(stdout)
	fmt.Print(stderr)
	return stdout, err
}

func KubectlGetInfo(cmdName string, arg []string) {
	cmd := exec.CommandContext(context.Background(), cmdName, arg...)
	fmt.Printf("Kubectl exec cmd =%v\n", cmd)
	stdout, stderr, err := veleroexec.RunCommand(cmd)
	fmt.Println(stdout)
	if err != nil {
		fmt.Println(stderr)
		fmt.Println(err)
	}
}

func DeleteVeleroDs(ctx context.Context) error {
	args := []string{"delete", "ds", "-n", "velero", "--all", "--force", "--grace-period", "0"}
	fmt.Println(args)
	return exec.CommandContext(ctx, "kubectl", args...).Run()
}

func WaitForCRDEstablished(crdName string) error {
	arg := []string{"wait", "--for", "condition=established", "--timeout=60s", "crd/" + crdName}
	cmd := exec.CommandContext(context.Background(), "kubectl", arg...)
	fmt.Printf("Kubectl exec cmd =%v\n", cmd)
	stdout, stderr, err := veleroexec.RunCommand(cmd)
	fmt.Println(stdout)
	if err != nil {
		fmt.Println(stderr)
		fmt.Println(err)
		return err
	}
	return nil
}

func BuildAddonLabelSelector() string {
	return fmt.Sprintf("%s=%s,%s=%s",
		constant.AppInstanceLabelKey, types.KubeBlocksReleaseName,
		constant.AppNameLabelKey, types.KubeBlocksChartName)
}
