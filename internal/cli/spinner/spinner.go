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

package spinner

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/briandowns/spinner"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type Spinner struct {
	s      *spinner.Spinner
	delay  time.Duration
	cancel chan struct{}
}

type Interface interface {
	Start()
	Done(status string)
	Success()
	Fail()
	SetMessage(msg string)
	SetFinalMsg(msg string)
	updateSpinnerMessage(msg string)
}

type Option func(Interface)

func WithMessage(msg string) Option {
	return func(s Interface) {
		s.updateSpinnerMessage(msg)
	}
}

func (s *Spinner) updateSpinnerMessage(msg string) {
	s.s.Suffix = fmt.Sprintf(" %s", msg)
}

func (s *Spinner) SetMessage(msg string) {
	s.updateSpinnerMessage(msg)
	if !s.s.Active() {
		s.Start()
	}
}

func (s *Spinner) Start() {
	if s.cancel != nil {
		return
	}
	if s.delay == 0 {
		s.s.Start()
		return
	}
	s.cancel = make(chan struct{}, 1)
	go func() {
		select {
		case <-s.cancel:
			return
		case <-time.After(s.delay):
			s.s.Start()
			s.cancel = nil
		}
		time.Sleep(50 * time.Millisecond)
	}()
}

func (s *Spinner) Done(status string) {
	if s.cancel != nil {
		close(s.cancel)
	}
	s.stop(status)
}

func (s *Spinner) SetFinalMsg(msg string) {
	s.s.FinalMSG = msg
	if !s.s.Active() {
		s.Start()
	}
}

func (s *Spinner) stop(status string) {
	if s.s == nil {
		return
	}

	if status != "" {
		s.s.FinalMSG = fmt.Sprintf("%s %s\n", strings.TrimPrefix(s.s.Suffix, " "), status)
	}
	s.s.Stop()

	// show cursor in terminal.
	fmt.Fprintf(s.s.Writer, "\033[?25h")
}

func (s *Spinner) Success() {
	s.Done(printer.BoldGreen("OK"))
}

func (s *Spinner) Fail() {
	s.Done(printer.BoldRed("FAIL"))
}

func New(w io.Writer, opts ...Option) Interface {
	if util.IsWindows() {
		return NewWindowsSpinner(w, opts...)
	}

	res := &Spinner{}
	res.s = spinner.New(spinner.CharSets[11],
		100*time.Millisecond,
		spinner.WithWriter(w),
		spinner.WithHiddenCursor(true),
		spinner.WithColor("cyan"),
	)

	for _, opt := range opts {
		opt(res)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	// Capture the interrupt signal, make the `spinner` program exit gracefully, and prevent the cursor from disappearing.
	go func() {
		<-c
		res.Done("")
		os.Exit(0)
	}()
	res.Start()
	return res
}
