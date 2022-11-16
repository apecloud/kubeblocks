/*
Copyright ApeCloud Inc.

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

package configmap

import (
	"context"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

type WatchEventHandler func(event fsnotify.Event) error
type NotifyEventFilter func(event fsnotify.Event) (bool, error)

const (
	DefaultRetryCount     = 3
	DefaultSleepRetryTime = 10
)

type ConfigMapVolumeWatcher struct {
	retryCount int

	// volumeDirectory watch directory witch volumeCount
	volumeDirectory []string

	// regexSelector string
	handler WatchEventHandler
	filters []NotifyEventFilter

	ctx     context.Context
	watcher *fsnotify.Watcher
}

func NewVolumeWatcher(volume []string, ctx context.Context) *ConfigMapVolumeWatcher {
	watcher := &ConfigMapVolumeWatcher{
		volumeDirectory: volume,
		ctx:             ctx,
		retryCount:      DefaultRetryCount,
		filters:         make([]NotifyEventFilter, 0),
	}
	// default add cm volume filter
	watcher.AddFilter(CreateValidConfigMapFilter())
	return watcher
}

func (w *ConfigMapVolumeWatcher) AddHandler(handler WatchEventHandler) *ConfigMapVolumeWatcher {
	w.handler = handler
	return w
}

func (w *ConfigMapVolumeWatcher) AddFilter(filter NotifyEventFilter) *ConfigMapVolumeWatcher {
	w.filters = append(w.filters, filter)
	return w
}

func (w *ConfigMapVolumeWatcher) SetRetryCount(count int) *ConfigMapVolumeWatcher {
	w.retryCount = count
	return w
}

func (w *ConfigMapVolumeWatcher) Close() error {
	if w.watcher != nil {
		return w.watcher.Close()
	}
	return nil
}

func (w *ConfigMapVolumeWatcher) Run() error {
	if w.handler == nil {
		return cfgcore.MakeError("require process event handler.")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return cfgcore.MakeError("failed to create fs notify watcher.")
	}

	go w.loopNotifyEvent(watcher, w.ctx)
	for _, d := range w.volumeDirectory {
		logrus.Info("add watched fs directory: ", d)
		err = watcher.Add(d)
		if err != nil {
			return cfgcore.WrapError(err, "failed to add watch directory[%s] failed", d)
		}
	}

	w.watcher = watcher
	return nil
}

func doFilter(filters []NotifyEventFilter, event fsnotify.Event) bool {
	for _, filter := range filters {
		if ok, err := filter(event); ok && err == nil {
			return true
		}
	}
	return false
}

func (w *ConfigMapVolumeWatcher) loopNotifyEvent(watcher *fsnotify.Watcher, ctx context.Context) {
	for {
		select {
		case event := <-watcher.Events:
			logrus.Tracef("watch fsnotify event: [%s]", event.String())
			if !doFilter(w.filters, event) {
				continue
			}
			logrus.Debugf("volume configmap updated. [%s]", event.String())
			runWithRetry(w.handler, event, w.retryCount)
		case err := <-watcher.Errors:
			logrus.Error(err)
		case <-ctx.Done():
			logrus.Info("The process has received the end signal.")
			return
		}
	}
}

func runWithRetry(handler WatchEventHandler, event fsnotify.Event, retryCount int) {
	var err error
	for {
		if err = handler(event); err == nil {
			return
		}
		retryCount--
		if retryCount <= 0 {
			return
		}
		logrus.Errorf("event handler failed, will retry after [%d]s : %s", DefaultSleepRetryTime, err)
		time.Sleep(time.Second * DefaultRetryCount)
	}
}
