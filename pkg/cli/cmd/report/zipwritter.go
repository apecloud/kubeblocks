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

package report

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/apecloud/kubeblocks/pkg/cli/util"
)

type reportWritter interface {
	Init(file string, printer printers.ResourcePrinterFunc) error
	Close() error
	WriteKubeBlocksVersion(fileName string, client kubernetes.Interface) error
	WriteObjects(folderName string, objects []*unstructured.UnstructuredList, format string) error
	WriteSingleObject(prefix string, kind string, name string, object runtime.Object, format string) error
	WriteEvents(folderName string, events map[string][]corev1.Event, format string) error
	WriteLogs(folderName string, ctx context.Context, client kubernetes.Interface, pods *corev1.PodList, logOptions corev1.PodLogOptions, allContainers bool) error
}

var _ reportWritter = &reportZipWritter{}

func NewReportWritter() reportWritter {
	return &reportZipWritter{}
}

type reportZipWritter struct {
	outputFile *os.File
	zipper     *zip.Writer
	printer    printers.ResourcePrinterFunc
}

func (w *reportZipWritter) Init(file string, printer printers.ResourcePrinterFunc) error {
	var err error
	// check if file exists
	exists, err := util.FileExists(file)
	if exists {
		return fmt.Errorf("file %s already exists", file)
	} else if err != nil {
		return err
	}

	// create file
	if w.outputFile, err = util.CreateAndCleanFile(file); err != nil {
		return fmt.Errorf("could not create zip file: %w", err)
	}
	w.zipper = zip.NewWriter(w.outputFile)
	w.printer = printer
	return nil
}

func (w *reportZipWritter) Close() error {
	var err error
	if w.outputFile == nil || w.zipper == nil {
		klog.Warning("zipWritter is not initialized")
		return nil
	}
	// sync file
	if err = w.outputFile.Sync(); err != nil {
		return fmt.Errorf("could not sync zip file: %s, error: %w", w.outputFile.Name(), err)
	}
	// close zipper
	if err = w.zipper.Close(); err != nil {
		return fmt.Errorf("could not close zip file: %s, error: %w", w.outputFile.Name(), err)
	}
	// close file
	if err = w.outputFile.Close(); err != nil {
		return fmt.Errorf("could not close zip file: %s, error: %w", w.outputFile.Name(), err)
	}
	return nil
}

func (w *reportZipWritter) WriteKubeBlocksVersion(fileName string, client kubernetes.Interface) error {
	var (
		err     error
		writter io.Writer
	)

	writter, err = w.zipper.Create(fileName)
	if err != nil {
		return fmt.Errorf("could not create zip file: %s, with err: %v", fileName, err)
	}

	version, err := util.GetVersionInfo(client)
	if err == nil {
		_, _ = writter.Write([]byte(fmt.Sprintf("Kubernetes: %s\n", version.Kubernetes)))
		_, _ = writter.Write([]byte(fmt.Sprintf("KubeBlocks: %s\n", version.KubeBlocks)))
		_, _ = writter.Write([]byte(fmt.Sprintf("Kbcli: %s\n", version.Cli)))
	}

	provider, err := util.GetK8sProvider(version.Kubernetes, client)
	if err == nil {
		_, _ = writter.Write([]byte(fmt.Sprintf("Kubernetes provider: %s\n", provider)))
	}
	return nil
}

func (w *reportZipWritter) WriteObjects(folderName string, objects []*unstructured.UnstructuredList, format string) error {
	if _, err := w.zipper.Create(folderName + "/"); err != nil {
		return fmt.Errorf("could not create zip file: %s, with err: %v", folderName, err)
	}

	for _, list := range objects {
		if len(list.Items) == 0 {
			continue
		}
		for _, obj := range list.Items {
			if err := w.WriteSingleObject(folderName, obj.GetKind(), obj.GetName(), &obj, format); err != nil {
				return fmt.Errorf("could not write object %s: %v", obj, err)
			}
		}
	}
	return nil
}

func (w *reportZipWritter) WriteSingleObject(prefix string, kind string, name string, object runtime.Object, format string) error {
	fileName := fmt.Sprintf("%s-%s.%s", kind, name, format)
	writter, err := w.zipper.Create(filepath.Join(prefix, fileName))
	if err != nil {
		return fmt.Errorf("could not create zip file: %s, with err %v", fileName, err)
	}
	return w.printer(object, writter)
}

func (w *reportZipWritter) WriteEvents(folderName string, events map[string][]corev1.Event, format string) error {
	if _, err := w.zipper.Create(folderName + "/"); err != nil {
		return fmt.Errorf("could not create zip file: %s, with err: %v", folderName, err)
	}

	for source, eventlist := range events {
		fileName := fmt.Sprintf("%s-events.%s", source, format)
		writter, err := w.zipper.Create(filepath.Join(folderName, fileName))
		if err != nil {
			return fmt.Errorf("could not create zip file: %s, with err: %v", fileName, err)
		}
		for _, item := range eventlist {
			if err := w.printer(&item, writter); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *reportZipWritter) WriteLogs(folderName string, ctx context.Context, client kubernetes.Interface, pods *corev1.PodList, logOptions corev1.PodLogOptions, allContainers bool) error {
	if _, err := w.zipper.Create(folderName + "/"); err != nil {
		return fmt.Errorf("could not create zip file: %s, with err: %v", "logs", err)
	}

	for _, pod := range pods.Items {
		if pod.Spec.Containers == nil || len(pod.Spec.Containers) == 0 {
			continue
		}
		// write pod log to file
		for _, container := range pod.Spec.Containers {
			containerName := container.Name
			logOptions.Container = containerName
			logFileName := filepath.Join(folderName, fmt.Sprintf("%s-%s.log", pod.Name, containerName))
			klog.V(1).Infof("writing logs for pod %s, container %s", pod.Name, containerName)

			writter, err := w.zipper.Create(logFileName)
			if err != nil {
				return fmt.Errorf("could not create zip file: %s, with err: %v", logFileName, err)
			}
			// get previous logs
			logOptions.Previous = true
			fmt.Fprint(writter, "=============Previous logs:=============\n")
			if err = util.WritePogStreamingLog(ctx, client, &pod, logOptions, writter); err != nil {
				klog.V(1).Infof("failed to get previous logs for pod %s, with err %v", pod.Name, err)
			}
			// get current logs
			logOptions.Previous = false
			fmt.Fprint(writter, "=============Current logs:=============\n")
			if err = util.WritePogStreamingLog(ctx, client, &pod, logOptions, writter); err != nil {
				klog.V(1).Infof("failed to get current logs for pod %s, with err %v", pod.Name, err)
			}
			if !allContainers {
				break
			}
		}
	}
	return nil
}
