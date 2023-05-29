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

package download

import (
	"io"
	"net/http"
	"os"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

type Fetcher interface {
	// Get gets the file and returns an stream to read the file.
	Get(uri string) (io.ReadCloser, error)
}

var _ Fetcher = HTTPFetcher{}

// HTTPFetcher is used to get a file from a http:// or https:// schema path.
type HTTPFetcher struct{}

// Get gets the file and returns an stream to read the file.
func (HTTPFetcher) Get(uri string) (io.ReadCloser, error) {
	klog.V(2).Infof("Fetching %q", uri)
	resp, err := http.Get(uri)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to download %q", uri)
	}
	return resp.Body, nil
}

var _ Fetcher = fileFetcher{}

type fileFetcher struct{ f string }

func (f fileFetcher) Get(_ string) (io.ReadCloser, error) {
	klog.V(2).Infof("Reading %q", f.f)
	file, err := os.Open(f.f)
	return file, errors.Wrapf(err, "failed to open archive file %q for reading", f.f)
}

// NewFileFetcher returns a local file reader.
func NewFileFetcher(path string) Fetcher { return fileFetcher{f: path} }
