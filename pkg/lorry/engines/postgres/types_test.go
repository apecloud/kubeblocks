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

package postgres

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestReadPidFile(t *testing.T) {
	fs = afero.NewMemMapFs()

	t.Run("can't open file", func(t *testing.T) {
		pidFile, err := readPidFile("")
		assert.Nil(t, pidFile)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "file does not exist")
	})

	t.Run("read pid file success", func(t *testing.T) {
		data := "97\n/postgresql/data\n1692770488\n5432\n/var/run/postgresql\n*\n  2388960         4\nready"
		err := afero.WriteFile(fs, "/postmaster.pid", []byte(data), 0644)
		if err != nil {
			t.Fatal(err)
		}

		pidFile, err := readPidFile("")
		assert.Nil(t, err)
		assert.Equal(t, pidFile.pid, int32(97))
		assert.Equal(t, pidFile.port, 5432)
		assert.Equal(t, pidFile.dataDir, "/postgresql/data")
		assert.Equal(t, pidFile.startTS, int64(1692770488))
	})

	t.Run("pid invalid", func(t *testing.T) {
		data := "test\n/postgresql/data\n1692770488\n5432\n/var/run/postgresql\n*\n  2388960         4\nready"
		err := afero.WriteFile(fs, "/postmaster.pid", []byte(data), 0644)
		if err != nil {
			t.Fatal(err)
		}

		pidFile, err := readPidFile("")
		assert.Nil(t, pidFile)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "invalid syntax")
	})

	t.Run("pid invalid", func(t *testing.T) {
		data := "97\n/postgresql/data\n1692770488\ntest\n/var/run/postgresql\n*\n  2388960         4\nready"
		err := afero.WriteFile(fs, "/postmaster.pid", []byte(data), 0644)
		if err != nil {
			t.Fatal(err)
		}

		pidFile, err := readPidFile("")
		assert.Nil(t, pidFile)
		assert.NotNil(t, err)
		assert.ErrorContains(t, err, "invalid syntax")
	})
}

func TestParsePGSyncStandby(t *testing.T) {
	t.Run("empty sync string", func(t *testing.T) {
		syncStandBys := ""
		resp, err := ParsePGSyncStandby(syncStandBys)

		assert.Nil(t, err)
		assert.Equal(t, off, resp.Types)
	})

	t.Run("only first", func(t *testing.T) {
		syncStandBys := "FiRsT"
		resp, err := ParsePGSyncStandby(syncStandBys)

		assert.Nil(t, err)
		assert.True(t, resp.Members.Contains("FiRsT"))
		assert.Equal(t, priority, resp.Types)
		assert.Equal(t, 1, resp.Amount)
	})

	t.Run("custom values", func(t *testing.T) {
		syncStandBys := `ANY 4("a",*,b)`
		resp, err := ParsePGSyncStandby(syncStandBys)

		assert.Nil(t, err)
		assert.Equal(t, quorum, resp.Types)
		assert.True(t, resp.HasStar)
		assert.True(t, resp.Members.Contains("a"))
		assert.Equal(t, 4, resp.Amount)
	})

	t.Run("custom values", func(t *testing.T) {
		syncStandBys := ` a , b `
		resp, err := ParsePGSyncStandby(syncStandBys)

		assert.Nil(t, err)
		assert.Equal(t, priority, resp.Types)
		assert.False(t, resp.HasStar)
		assert.True(t, resp.Members.Contains("a"))
		assert.True(t, resp.Members.Contains("b"))
		assert.Equal(t, 1, resp.Amount)
	})

	t.Run("custom values", func(t *testing.T) {
		syncStandBys := `FIRST 2 (s1,s2,s3)`
		resp, err := ParsePGSyncStandby(syncStandBys)

		assert.Nil(t, err)
		assert.Equal(t, priority, resp.Types)
		assert.False(t, resp.HasStar)
		assert.True(t, resp.Members.Contains("s1"))
		assert.True(t, resp.Members.Contains("s2"))
		assert.Equal(t, 2, resp.Amount)
	})

	t.Run("custom values", func(t *testing.T) {
		syncStandBys := `2 (s1,s2,s3)`
		resp, err := ParsePGSyncStandby(syncStandBys)

		assert.Nil(t, err)
		assert.Equal(t, priority, resp.Types)
		assert.False(t, resp.HasStar)
		assert.True(t, resp.Members.Contains("s1"))
		assert.True(t, resp.Members.Contains("s2"))
		assert.Equal(t, 2, resp.Amount)
	})

	t.Run("can't parse synchronous standby name", func(t *testing.T) {
		syncStandBys := `ANY 4("a" b,"c c")`
		resp, err := ParsePGSyncStandby(syncStandBys)

		assert.NotNil(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "Unparseable synchronous_standby_names value")
	})
}

func TestParsePGLsn(t *testing.T) {
	t.Run("legal lsn str", func(t *testing.T) {
		lsnStr := "16/B374D848"

		lsn := ParsePgLsn(lsnStr)
		assert.Equal(t, int64(97500059720), lsn)
	})

	t.Run("illegal lsn str", func(t *testing.T) {
		lsnStr := "B374D848"

		lsn := ParsePgLsn(lsnStr)
		assert.Equal(t, int64(0), lsn)
	})
}

func TestFormatPgLsn(t *testing.T) {
	t.Run("format lsn", func(t *testing.T) {
		lsn := int64(16777376)

		lsnStr := FormatPgLsn(lsn)
		assert.Equal(t, "0/010000A0", lsnStr)
	})
}

func TestParsePrimaryConnInfo(t *testing.T) {
	t.Run("legal primary conn info str", func(t *testing.T) {
		primaryConnInfoStr := "host=pg-pg-replication-0.pg-pg-replication-headless port=5432 user=postgres application_name=my-application"

		result := ParsePrimaryConnInfo(primaryConnInfoStr)
		assert.NotNil(t, result)
		assert.Equal(t, "pg-pg-replication-0.pg-pg-replication-headless", result["host"])
		assert.Equal(t, "5432", result["port"])
		assert.Equal(t, "postgres", result["user"])
		assert.Equal(t, "my-application", result["application_name"])
	})

	t.Run("illegal primary conn info str", func(t *testing.T) {
		primaryConnInfoStr := "host pg-pg-replication-0.pg-pg-replication-headless port 5432 user postgres application_name my-application"

		result := ParsePrimaryConnInfo(primaryConnInfoStr)
		assert.NotNil(t, result)
		assert.Equal(t, map[string]string{}, result)
	})
}

func TestParseHistory(t *testing.T) {
	t.Run("parse history success", func(t *testing.T) {
		historyStr :=
			`     filename     |                       content
------------------+------------------------------------------------------
 00000003.history | 1       0/50000A0       no recovery target specified+
                  |                                                     +
                  | 2       0/60000A0       no recovery target specified+
                  |`

		history := ParseHistory(historyStr)
		assert.NotNil(t, history)
		assert.Len(t, history.History, 2)
		assert.Equal(t, int64(1), history.History[0].ParentTimeline)
		assert.Equal(t, int64(2), history.History[1].ParentTimeline)
		assert.Equal(t, ParsePgLsn("0/50000A0 "), history.History[0].SwitchPoint)
		assert.Equal(t, ParsePgLsn("0/60000A0 "), history.History[1].SwitchPoint)
	})
}

func TestParsePgWalDumpError(t *testing.T) {
	t.Run("parse success", func(t *testing.T) {
		errorInfo := "pg_waldump: fatal: error in WAL record at 0/182E220: invalid record length at 0/182E298: wanted 24, got 0"

		resp := ParsePgWalDumpError(errorInfo, "0/182E220")

		assert.Equal(t, "0/182E298", resp)
	})

	t.Run("parse failed", func(t *testing.T) {
		errorInfo := "pg_waldump: fatal: error in WAL record at 0/182E220: invalid record length at 0/182E298"

		resp := ParsePgWalDumpError(errorInfo, "0/182E220")

		assert.Equal(t, "", resp)
	})
}
