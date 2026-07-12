/*
Copyright 2026 The KubeBlocks Authors.

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

package helm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStorageProviderTemplatesUseCommonLabels(t *testing.T) {
	templates, err := filepath.Glob("../../deploy/helm/templates/storageprovider/*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) == 0 {
		t.Fatal("no StorageProvider templates found")
	}

	for _, template := range templates {
		content, err := os.ReadFile(template)
		if err != nil {
			t.Fatal(err)
		}
		if !metadataUsesCommonLabels(string(content)) {
			t.Errorf("%s metadata does not use the common KubeBlocks labels", filepath.Base(template))
		}
	}
}

func metadataUsesCommonLabels(content string) bool {
	const metadataStart = "metadata:\n"
	const specStart = "\nspec:\n"
	const labelsLine = "  labels:"
	const commonLabelsLine = `    {{- include "kubeblocks.labels" . | nindent 4 }}`

	start := strings.Index(content, metadataStart)
	if start == -1 {
		return false
	}
	metadata := content[start+len(metadataStart):]
	end := strings.Index(metadata, specStart)
	if end == -1 {
		return false
	}
	lines := strings.Split(metadata[:end], "\n")
	for i := 0; i+1 < len(lines); i++ {
		if lines[i] == labelsLine && lines[i+1] == commonLabelsLine {
			return true
		}
	}
	return false
}
