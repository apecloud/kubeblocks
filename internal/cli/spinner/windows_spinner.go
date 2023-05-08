package spinner

import (
	"fmt"
	"io"
	"strings"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
)

type WindowsSpinner struct {
	msg string
	out io.Writer
}

func (s *WindowsSpinner) UpdateSpinnerMessage(msg string) {
	s.msg = fmt.Sprintf(" %s", msg)
}

func (s *WindowsSpinner) Done(status string) {
	out := fmt.Sprintf("%s %s\n", strings.TrimPrefix(s.msg, " "), status)
	_, _ = fmt.Print(out)
	s.msg = ""
}

func (s *WindowsSpinner) Success() {
	if len(s.msg) == 0 {
		return
	}
	s.Done(printer.BoldGreen("OK"))

}

func (s *WindowsSpinner) Fail() {
	if len(s.msg) == 0 {
		return
	}
	s.Done(printer.BoldRed("FAIL"))
}

func (s *WindowsSpinner) Start() {
}

func (s *WindowsSpinner) SetMessage(msg string) {
}

func (s *WindowsSpinner) SetFinalMsg(msg string) {
}
