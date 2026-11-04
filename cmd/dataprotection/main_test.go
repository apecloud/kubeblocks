/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package main

import (
	"strings"
	"testing"

	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

func TestLeaderElectionTimingValidate(t *testing.T) {
	tests := []struct {
		name          string
		leaseDuration int
		renewDeadline int
		retryPeriod   int
		wantErr       string
	}{
		{
			name:          "defaults",
			leaseDuration: 30,
			renewDeadline: 25,
			retryPeriod:   5,
		},
		{
			name:          "longer timings to tolerate a slow API server",
			leaseDuration: 60,
			renewDeadline: 45,
			retryPeriod:   15,
		},
		{
			// 7 seconds is greater than 1.2 * 5 = 6 seconds, so the combination is still valid.
			name:          "renewDeadline just above the jitter bound",
			leaseDuration: 30,
			renewDeadline: 7,
			retryPeriod:   5,
		},
		{
			name:          "zero leaseDuration",
			leaseDuration: 0,
			renewDeadline: 25,
			retryPeriod:   5,
			wantErr:       "leader-elect-lease-duration must be greater than zero",
		},
		{
			name:          "negative leaseDuration",
			leaseDuration: -30,
			renewDeadline: 25,
			retryPeriod:   5,
			wantErr:       "leader-elect-lease-duration must be greater than zero",
		},
		{
			name:          "zero renewDeadline",
			leaseDuration: 30,
			renewDeadline: 0,
			retryPeriod:   5,
			wantErr:       "leader-elect-renew-deadline must be greater than zero",
		},
		{
			name:          "zero retryPeriod",
			leaseDuration: 30,
			renewDeadline: 25,
			retryPeriod:   0,
			wantErr:       "leader-elect-retry-period must be greater than zero",
		},
		{
			name:          "negative retryPeriod",
			leaseDuration: 30,
			renewDeadline: 25,
			retryPeriod:   -5,
			wantErr:       "leader-elect-retry-period must be greater than zero",
		},
		{
			name:          "leaseDuration equal to renewDeadline",
			leaseDuration: 25,
			renewDeadline: 25,
			retryPeriod:   5,
			wantErr:       "must be greater than leader-elect-renew-deadline",
		},
		{
			name:          "leaseDuration smaller than renewDeadline",
			leaseDuration: 20,
			renewDeadline: 25,
			retryPeriod:   5,
			wantErr:       "must be greater than leader-elect-renew-deadline",
		},
		{
			// 6 seconds equals 1.2 * 5, and client-go requires a strictly greater value.
			name:          "renewDeadline exactly at the jitter bound",
			leaseDuration: 30,
			renewDeadline: 6,
			retryPeriod:   5,
			wantErr:       "1.2 * leader-elect-retry-period",
		},
		{
			name:          "renewDeadline below the jitter bound",
			leaseDuration: 30,
			renewDeadline: 5,
			retryPeriod:   5,
			wantErr:       "1.2 * leader-elect-retry-period",
		},
		{
			name:          "renewDeadline equal to retryPeriod",
			leaseDuration: 30,
			renewDeadline: 5,
			retryPeriod:   5,
			wantErr:       "1.2 * leader-elect-retry-period",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLeaderElectionTiming(tt.leaseDuration, tt.renewDeadline, tt.retryPeriod)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateLeaderElectionTiming(%d, %d, %d) = %v, want nil",
						tt.leaseDuration, tt.renewDeadline, tt.retryPeriod, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateLeaderElectionTiming(%d, %d, %d) = nil, want an error containing %q",
					tt.leaseDuration, tt.renewDeadline, tt.retryPeriod, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("validateLeaderElectionTiming(%d, %d, %d) = %q, want it to contain %q",
					tt.leaseDuration, tt.renewDeadline, tt.retryPeriod, err.Error(), tt.wantErr)
			}
		})
	}
}

// TestLeaderElectionTimingDefaults pins the defaults exposed by the flags. They follow the main
// manager (cmd/manager/main.go), so changing them changes the out-of-the-box behavior of every
// installation that does not override the values.
func TestLeaderElectionTimingDefaults(t *testing.T) {
	if defaultLeaderElectLeaseDuration != 30 {
		t.Errorf("defaultLeaderElectLeaseDuration = %d, want 30", defaultLeaderElectLeaseDuration)
	}
	if defaultLeaderElectRenewDeadline != 25 {
		t.Errorf("defaultLeaderElectRenewDeadline = %d, want 25", defaultLeaderElectRenewDeadline)
	}
	if defaultLeaderElectRetryPeriod != 5 {
		t.Errorf("defaultLeaderElectRetryPeriod = %d, want 5", defaultLeaderElectRetryPeriod)
	}

	if err := validateLeaderElectionTiming(
		defaultLeaderElectLeaseDuration, defaultLeaderElectRenewDeadline, defaultLeaderElectRetryPeriod); err != nil {
		t.Errorf("the defaults must pass validation, got %v", err)
	}
}

// TestLeaderElectionViperKeys makes sure every flag is bound to the viper key that the timing
// lookup and the startup validation rely on.
func TestLeaderElectionViperKeys(t *testing.T) {
	tests := []struct {
		flag flagName
		want string
	}{
		{leaderElectLeaseDurationFlagKey, "leader_elect_lease_duration"},
		{leaderElectRenewDeadlineFlagKey, "leader_elect_renew_deadline"},
		{leaderElectRetryPeriodFlagKey, "leader_elect_retry_period"},
	}
	for _, tt := range tests {
		if got := tt.flag.viperName(); got != tt.want {
			t.Errorf("%s.viperName() = %q, want %q", tt.flag, got, tt.want)
		}
	}
}

// TestValidateRequiredToParseConfigsLeaderElection covers the startup path: the timings are read
// from viper and an invalid combination is rejected through validateRequiredToParseConfigs, which
// is what turns a bad configuration into a clear startup error instead of a later failure during
// leader election.
func TestValidateRequiredToParseConfigsLeaderElection(t *testing.T) {
	defer func() {
		viper.Set(leaderElectFlagKey.viperName(), false)
		viper.Set(leaderElectLeaseDurationFlagKey.viperName(), defaultLeaderElectLeaseDuration)
		viper.Set(leaderElectRenewDeadlineFlagKey.viperName(), defaultLeaderElectRenewDeadline)
		viper.Set(leaderElectRetryPeriodFlagKey.viperName(), defaultLeaderElectRetryPeriod)
	}()

	tests := []struct {
		name          string
		leaderElect   bool
		leaseDuration int
		renewDeadline int
		retryPeriod   int
		wantErr       string
	}{
		{
			name:          "defaults with leader election enabled",
			leaderElect:   true,
			leaseDuration: 30,
			renewDeadline: 25,
			retryPeriod:   5,
		},
		{
			name:          "longer timings with leader election enabled",
			leaderElect:   true,
			leaseDuration: 60,
			renewDeadline: 45,
			retryPeriod:   15,
		},
		{
			name:          "leaseDuration not greater than renewDeadline",
			leaderElect:   true,
			leaseDuration: 20,
			renewDeadline: 25,
			retryPeriod:   5,
			wantErr:       "must be greater than leader-elect-renew-deadline",
		},
		{
			name:          "renewDeadline not greater than the jitter bound",
			leaderElect:   true,
			leaseDuration: 30,
			renewDeadline: 5,
			retryPeriod:   5,
			wantErr:       "1.2 * leader-elect-retry-period",
		},
		{
			name:          "non positive retryPeriod",
			leaderElect:   true,
			leaseDuration: 30,
			renewDeadline: 25,
			retryPeriod:   0,
			wantErr:       "leader-elect-retry-period must be greater than zero",
		},
		{
			// The timings are unused without leader election, so they are not validated.
			name:          "invalid timings are ignored when leader election is disabled",
			leaderElect:   false,
			leaseDuration: 1,
			renewDeadline: 100,
			retryPeriod:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set(leaderElectFlagKey.viperName(), tt.leaderElect)
			viper.Set(leaderElectLeaseDurationFlagKey.viperName(), tt.leaseDuration)
			viper.Set(leaderElectRenewDeadlineFlagKey.viperName(), tt.renewDeadline)
			viper.Set(leaderElectRetryPeriodFlagKey.viperName(), tt.retryPeriod)

			err := validateRequiredToParseConfigs()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateRequiredToParseConfigs() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateRequiredToParseConfigs() = nil, want an error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("validateRequiredToParseConfigs() = %q, want it to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
