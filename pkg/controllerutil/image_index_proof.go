/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package controllerutil

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	dockerManifestListMediaType  = "application/vnd.docker.distribution.manifest.list.v2+json"
	dockerImageManifestMediaType = "application/vnd.docker.distribution.manifest.v2+json"
)

type imageIndexProofConfig struct {
	IndexDigest string `mapstructure:"indexDigest"`
	IndexBase64 string `mapstructure:"indexBase64"`
}

type imagePlatform struct {
	operatingSystem string
	architecture    string
}

type imageIndexProof struct {
	children map[imagePlatform]digest.Digest
}

type imageIndexProofSet map[digest.Digest]imageIndexProof

func compileImageIndexProofs(configs []imageIndexProofConfig) (imageIndexProofSet, error) {
	proofs := make(imageIndexProofSet, len(configs))
	for i, config := range configs {
		parent, proof, err := compileImageIndexProof(config)
		if err != nil {
			return nil, fmt.Errorf("imageIndexProofs[%d]: %w", i, err)
		}
		if _, ok := proofs[parent]; ok {
			return nil, fmt.Errorf("imageIndexProofs[%d]: duplicate index digest %q", i, parent)
		}
		proofs[parent] = proof
	}
	return proofs, nil
}

func compileImageIndexProof(config imageIndexProofConfig) (digest.Digest, imageIndexProof, error) {
	if config.IndexDigest == "" {
		return "", imageIndexProof{}, fmt.Errorf("indexDigest can't be empty")
	}
	if config.IndexBase64 == "" {
		return "", imageIndexProof{}, fmt.Errorf("indexBase64 can't be empty")
	}

	parent, err := digest.Parse(config.IndexDigest)
	if err != nil {
		return "", imageIndexProof{}, fmt.Errorf("invalid indexDigest: %w", err)
	}
	if !parent.Algorithm().Available() {
		return "", imageIndexProof{}, fmt.Errorf("indexDigest algorithm %q is unavailable", parent.Algorithm())
	}
	raw, err := base64.StdEncoding.DecodeString(config.IndexBase64)
	if err != nil {
		return "", imageIndexProof{}, fmt.Errorf("decode indexBase64: %w", err)
	}
	if actual := parent.Algorithm().FromBytes(raw); actual != parent {
		return "", imageIndexProof{}, fmt.Errorf("raw index digest %q does not match indexDigest %q", actual, parent)
	}

	var index ocispec.Index
	if err := json.Unmarshal(raw, &index); err != nil {
		return "", imageIndexProof{}, fmt.Errorf("decode raw index JSON: %w", err)
	}
	if index.SchemaVersion != 2 {
		return "", imageIndexProof{}, fmt.Errorf("unsupported schemaVersion %d", index.SchemaVersion)
	}
	if index.MediaType != ocispec.MediaTypeImageIndex && index.MediaType != dockerManifestListMediaType {
		return "", imageIndexProof{}, fmt.Errorf("unsupported index mediaType %q", index.MediaType)
	}

	children := make(map[imagePlatform]digest.Digest)
	ambiguous := make(map[imagePlatform]struct{})
	for _, descriptor := range index.Manifests {
		if descriptor.Platform == nil {
			continue
		}
		platform := imagePlatform{
			operatingSystem: descriptor.Platform.OS,
			architecture:    descriptor.Platform.Architecture,
		}
		if platform.operatingSystem == "" || platform.architecture == "" ||
			strings.EqualFold(platform.operatingSystem, "unknown") ||
			strings.EqualFold(platform.architecture, "unknown") {
			continue
		}
		if _, found := ambiguous[platform]; found {
			continue
		}

		unusable := descriptor.Platform.Variant != "" || descriptor.Platform.OSVersion != "" ||
			len(descriptor.Platform.OSFeatures) != 0 || descriptor.Size <= 0 ||
			(descriptor.MediaType != ocispec.MediaTypeImageManifest && descriptor.MediaType != dockerImageManifestMediaType)
		child, digestErr := digest.Parse(descriptor.Digest.String())
		if digestErr != nil || !child.Algorithm().Available() {
			unusable = true
		}
		if _, found := children[platform]; found {
			unusable = true
		}
		if unusable {
			delete(children, platform)
			ambiguous[platform] = struct{}{}
			continue
		}
		children[platform] = child
	}
	if len(children) == 0 {
		return "", imageIndexProof{}, fmt.Errorf("raw index has no uniquely usable OS/architecture platform")
	}
	return parent, imageIndexProof{children: children}, nil
}

func (proofs imageIndexProofSet) matchPair(specDigest, statusDigest, operatingSystem, architecture string) bool {
	spec, specErr := digest.Parse(specDigest)
	status, statusErr := digest.Parse(statusDigest)
	if specErr != nil || statusErr != nil {
		return false
	}
	platform := imagePlatform{operatingSystem: operatingSystem, architecture: architecture}
	if proof, ok := proofs[spec]; ok && proof.children[platform] == status {
		return true
	}
	if proof, ok := proofs[status]; ok && proof.children[platform] == spec {
		return true
	}
	return false
}

func (proofs imageIndexProofSet) containsPair(specDigest, statusDigest string) bool {
	spec, specErr := digest.Parse(specDigest)
	status, statusErr := digest.Parse(statusDigest)
	if specErr != nil || statusErr != nil {
		return false
	}
	containsChild := func(proof imageIndexProof, child digest.Digest) bool {
		for _, candidate := range proof.children {
			if candidate == child {
				return true
			}
		}
		return false
	}
	if proof, ok := proofs[spec]; ok && containsChild(proof, status) {
		return true
	}
	if proof, ok := proofs[status]; ok && containsChild(proof, spec) {
		return true
	}
	return false
}
