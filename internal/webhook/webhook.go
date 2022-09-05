/*
Copyright 2022.

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

package webhook

import (
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	setupLog = ctrl.Log.WithName("webhook-setup")
	// HandlerMap contains all admission webhook handlers.
	HandlerMap = map[string]admission.Handler{}
	webhookMgr *webHookManager
)

type webHookManager struct {
	client client.Client
}

func GetWebHookClient() client.Client {
	if webhookMgr != nil {
		return webhookMgr.client
	}
	return nil
}

func SetupWithManager(mgr manager.Manager) error {
	server := mgr.GetWebhookServer()
	webhookMgr = &webHookManager{mgr.GetClient()}
	// register admission handlers
	for path, handler := range HandlerMap {
		server.Register(path, &webhook.Admission{Handler: handler})
		setupLog.V(3).Info("Registered webhook handler", "path", path)
	}
	return nil
}
