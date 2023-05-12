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

package alert

import (
	"fmt"
	"strings"
)

// addon name
const (
	// alertManagerAddonName is the name of alertmanager addon
	alertManagerAddonName = "prometheus"

	// webhookAdaptorAddonName is the name of webhook adaptor addon
	webhookAdaptorAddonName = "alertmanager-webhook-adaptor"
)

var (
	addonCMSuffix = map[string]string{
		alertManagerAddonName:   alertConfigMapNameSuffix,
		webhookAdaptorAddonName: webhookAdaptorConfigMapNameSuffix,
	}
)

// configmap name suffix
const (
	// alertConfigMapNameSuffix is the suffix of alertmanager configmap name
	alertConfigMapNameSuffix = "alertmanager-config"

	// webhookAdaptorConfigMapNameSuffix is the suffix of webhook adaptor configmap name
	webhookAdaptorConfigMapNameSuffix = "config"
)

// config file name
const (
	// alertConfigFileName is the name of alertmanager config file
	alertConfigFileName = "alertmanager.yml"

	// webhookAdaptorFileName is the name of webhook adaptor config file
	webhookAdaptorFileName = "config.yml"
)

const (
	routeMatcherClusterKey  = "app_kubernetes_io_instance"
	routeMatcherSeverityKey = "severity"
	routeMatcherOperator    = "=~"
)

const (
	routeMatcherClusterType  = "cluster"
	routeMatcherSeverityType = "severity"
)

// severity is the severity of alert
type severity string

const (
	// severityCritical is the critical severity
	severityCritical severity = "critical"
	// severityWarning is the warning severity
	severityWarning severity = "warning"
	// severityInfo is the info severity
	severityInfo severity = "info"
)

type webhookKey string

// webhook keys
const (
	webhookURL   webhookKey = "url"
	webhookToken webhookKey = "token"
)

type webhookType string

const (
	feishuWebhookType   webhookType = "feishu-webhook"
	wechatWebhookType   webhookType = "wechat-webhook"
	dingtalkWebhookType webhookType = "dingtalk-webhook"
	unknownWebhookType  webhookType = "unknown"
)

type slackKey string

// slackConfig keys
const (
	slackAPIURL    slackKey = "api_url"
	slackChannel   slackKey = "channel"
	slackUsername  slackKey = "username"
	slackTitleLink slackKey = "title_link"
)

// emailConfig is the email config of receiver
type emailConfig struct {
	To string `json:"to"`
}

// webhookConfig is the webhook config of receiver
type webhookConfig struct {
	URL          string `json:"url"`
	SendResolved bool   `json:"send_resolved"`
	MaxAlerts    int    `json:"max_alerts,omitempty"`
}

// slackConfig is the alertmanager slack config of receiver
// ref: https://prometheus.io/docs/alerting/latest/configuration/#slack_config
type slackConfig struct {
	APIURL    string `json:"api_url,omitempty"`
	Channel   string `json:"channel,omitempty"`
	Username  string `json:"username,omitempty"`
	TitleLink string `json:"title_link"`
}

// receiver is the receiver of alert
type receiver struct {
	Name           string           `json:"name"`
	EmailConfigs   []*emailConfig   `json:"email_configs,omitempty"`
	SlackConfigs   []*slackConfig   `json:"slack_configs,omitempty"`
	WebhookConfigs []*webhookConfig `json:"webhook_configs,omitempty"`
}

// route is the route of receiver
type route struct {
	Receiver string   `json:"receiver"`
	Continue bool     `json:"continue,omitempty"`
	Matchers []string `json:"matchers,omitempty"`
}

type webhookAdaptorReceiverParams struct {
	URL    string `json:"url"`
	Secret string `json:"secret,omitempty"`
}

type webhookAdaptorReceiver struct {
	Name   string                       `json:"name"`
	Type   string                       `json:"type"`
	Params webhookAdaptorReceiverParams `json:"params"`
}

func (w *webhookConfig) string() string {
	return fmt.Sprintf("url=%s", w.URL)
}

func (s *slackConfig) string() string {
	var cfgs []string
	if s.APIURL != "" {
		cfgs = append(cfgs, fmt.Sprintf("api_url=%s", s.APIURL))
	}
	if s.Channel != "" {
		cfgs = append(cfgs, fmt.Sprintf("channel=%s", s.Channel))
	}
	if s.Username != "" {
		cfgs = append(cfgs, fmt.Sprintf("username=%s", s.Username))
	}
	return strings.Join(cfgs, ",")
}

func (e *emailConfig) string() string {
	return e.To
}
