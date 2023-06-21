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
	"io"

	"github.com/shirou/gopsutil/v3/host"
)

// PrintSystemInfo print system info
func PrintSystemInfo(w io.Writer) {
	hostStat, err := host.Info()
	if err != nil {
		fmt.Fprintf(w, "get host info failed: %v\n", err)
		return
	}
	fmt.Fprintf(w, "---- operating system info ----\n")
	fmt.Fprintf(w, "OS: %s\n", hostStat.OS)
	fmt.Fprintf(w, "Platform: %s-%s\n", hostStat.Platform, hostStat.PlatformVersion)
	fmt.Fprintf(w, "KernelVersion: %s\n", hostStat.KernelVersion)
	fmt.Fprintf(w, "KernelArch: %s\n", hostStat.KernelArch)
	fmt.Fprintf(w, "--------------------------------\n\n")
}
