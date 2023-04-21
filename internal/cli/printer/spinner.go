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

package printer

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/briandowns/spinner"

	"github.com/apecloud/kubeblocks/internal/cli/types"
)

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
		s.HideCursor = true
		_ = s.Color("cyan")
		s.Suffix = fmt.Sprintf(" %s", msg)
		s.Start()

		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		// Capture the interrupt signal, make the `spinner` program exit gracefully, and prevent the cursor from disappearing.
		go func() {
			<-c
			s.Stop()
			// Show cursor in terminal.
			fmt.Fprintf(s.Writer, "\033[?25h")
			os.Exit(0)
		}()
	}

	return func(result bool) {
		once.Do(func() {
			if s != nil {
				s.Stop()
			}
			if result {
				fmt.Fprintf(w, "%s %s\n", msg, BoldGreen("OK"))
			} else {
				fmt.Fprintf(w, "%s %s\n", msg, BoldRed("FAIL"))
			}
		})
	}
}
