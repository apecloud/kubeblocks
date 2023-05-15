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
	"sync"
	"syscall"

	"time"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
)

var char = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

type WindowsSpinner struct { // no thread/goroutine safe
	msg        string
	lastOutput string
	FinalMSG   string
	active     bool
	chars      []string
	cancel     chan struct{}
	Writer     io.Writer
	delay      time.Duration
	mu         *sync.RWMutex
}

func NewWindowsSpinner(w io.Writer, opts ...Option) *WindowsSpinner {
	res := &WindowsSpinner{
		chars:  char,
		active: false,
		cancel: make(chan struct{}, 1),
		Writer: w,
		mu:     &sync.RWMutex{},
		delay:  100 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(res)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		res.Done("")
		os.Exit(0)
	}()
	res.Start()
	return res
}

func (s *WindowsSpinner) updateSpinnerMessage(msg string) {
	s.msg = fmt.Sprintf(" %s", msg)
}

func (s *WindowsSpinner) Done(status string) {
	if status != "" {
		s.FinalMSG = fmt.Sprintf("%s %s\n", strings.TrimPrefix(s.msg, " "), status)
	}
	s.stop()
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
	s.active = true

	go func() {
		for {
			for i := 0; i < len(s.chars); i++ {
				select {
				case <-s.cancel:
					return
				default:
					s.mu.Lock()
					if !s.active {
						defer s.mu.Unlock()
						return
					}
					outPlain := fmt.Sprintf("\r%s%s", s.chars[i], s.msg)
					s.erase()
					s.lastOutput = outPlain
					//fmt.Print(outPlain)
					fmt.Fprint(s.Writer, outPlain)
					s.mu.Unlock()
					// fmt.Fprint(s.Writer, outPlain)
					time.Sleep(s.delay)
				}
			}
		}
	}()
}

func (s *WindowsSpinner) SetMessage(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msg = msg
}

func (s *WindowsSpinner) SetFinalMsg(msg string) {
	s.FinalMSG = msg
}

// remove lastOutplain
func (s *WindowsSpinner) erase() {
	split := strings.Split(s.lastOutput, "\n")
	for i := 0; i < len(split); i++ {
		if i > 0 {
			//fmt.Print("\033[A")
			fmt.Fprint(s.Writer, "\033[A")
		}
		//fmt.Print("\r\033[K")
		fmt.Fprint(s.Writer, "\r\033[K")
	}
}

// stop stops the indicator.
func (s *WindowsSpinner) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		s.active = false
		if s.FinalMSG != "" {
			s.erase()
			//fmt.Print(s.FinalMSG)
			fmt.Fprint(s.Writer, s.FinalMSG)

		}
		s.cancel <- struct{}{}
		close(s.cancel)
	}
}
