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

package alert

const (
	// alertmanagerYmlJSONPath is the json path of alertmanager.yml in KubeBlocks Helm Chart
	alertmanagerYmlJSONPath = "prometheus.alertmanagerFiles.\"alertmanager\\.yml\""
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

// webhookConfig keys
const (
	webhookURL   webhookKey = "url"
	webhookToken webhookKey = "token"
)

type slackKey string

// slackConfig keys
const (
	slackAPIURL   slackKey = "api_url"
	slackChannel  slackKey = "channel"
	slackUsername slackKey = "username"
)

// emailConfig is the email config of receiver
type emailConfig struct {
	To string `json:"to,omitempty"`
}

// webhookConfig is the webhook config of receiver
type webhookConfig struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

type slackConfig struct {
	APIURL   string `json:"api_url,omitempty"`
	Channel  string `json:"channel,omitempty"`
	Username string `json:"username,omitempty"`
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
	Matchers []string `json:"matchers"`
}
