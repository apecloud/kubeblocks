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

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

var (
	configSMTPServerExample = `
	# Set smtp server config
	kbcli alert config-smtpserver --smtp-from alert-test@apecloud.com --smtp-smarthost smtp.feishu.cn:587 --smtp-auth-username alert-test@apecloud.com --smtp-auth-password 123456abc --smtp-auth-identity alert-test@apecloud.com
	`
)

type configSMTPServerOptions struct {
	smtpFrom         string
	smtpSmarthost    string
	smtpAuthUsername string
	smtpAuthPassword string
	smtpAuthIdentity string

	baseOptions
}

func newConfigSMTPServerCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &configSMTPServerOptions{baseOptions: baseOptions{IOStreams: streams}}
	cmd := &cobra.Command{
		Use:     "config-smtpserver",
		Short:   "Set smtp server config",
		Example: configSMTPServerExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(f))
			cmdutil.CheckErr(o.validate())
			cmdutil.CheckErr(o.run())
		},
	}

	cmd.Flags().StringVar(&o.smtpFrom, "smtp-from", "", "The email address to send alert.")
	cmd.Flags().StringVar(&o.smtpSmarthost, "smtp-smarthost", "", "The smtp host to send alert.")
	cmd.Flags().StringVar(&o.smtpAuthUsername, "smtp-auth-username", "", "The username to authenticate to the smarthost.")
	cmd.Flags().StringVar(&o.smtpAuthPassword, "smtp-auth-password", "", "The password to authenticate to the smarthost.")
	cmd.Flags().StringVar(&o.smtpAuthIdentity, "smtp-auth-identity", "", "The identity to authenticate to the smarthost.")

	return cmd
}

func (o *configSMTPServerOptions) validate() error {
	if !validEmail(o.smtpFrom) {
		return fmt.Errorf("smtp-from is invalid")
	}

	if o.smtpSmarthost == "" {
		return fmt.Errorf("smtp-smarthost is required")
	}

	if o.smtpAuthUsername == "" {
		return fmt.Errorf("smtp-auth-username is required")
	}

	if o.smtpAuthPassword == "" {
		return fmt.Errorf("smtp-auth-password is required")
	}

	return nil
}

func (o *configSMTPServerOptions) run() error {
	data, err := getConfigData(o.alertConfigMap, alertConfigFileName)
	if err != nil {
		return err
	}

	// get global
	global := getGlobalFromData(data)

	// write smtp server config
	global["smtp_from"] = o.smtpFrom
	global["smtp_smarthost"] = o.smtpSmarthost
	global["smtp_auth_username"] = o.smtpAuthUsername
	global["smtp_auth_password"] = o.smtpAuthPassword
	global["smtp_auth_identity"] = o.smtpAuthIdentity

	data["global"] = global

	// update global config
	return updateConfig(o.client, o.alertConfigMap, alertConfigFileName, data)
}
