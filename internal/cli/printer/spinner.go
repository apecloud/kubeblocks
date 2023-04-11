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
		go func() {
			<-c
			s.Stop()
			fmt.Print("\033[?25h")
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
