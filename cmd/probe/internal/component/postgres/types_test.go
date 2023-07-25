package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePGSyncStandby(t *testing.T) {
	t.Run("empty sync string", func(t *testing.T) {
		syncStandBys := ""
		resp, err := parsePGSyncStandby(syncStandBys)

		assert.Nil(t, err)
		assert.Equal(t, off, resp.Types)
	})

	t.Run("only first", func(t *testing.T) {
		syncStandBys := "FiRsT"
		resp, err := parsePGSyncStandby(syncStandBys)

		assert.Nil(t, err)
		assert.True(t, resp.Members.Contains("FiRsT"))
		assert.Equal(t, priority, resp.Types)
		assert.Equal(t, 1, resp.Amount)
	})

	t.Run("custom values", func(t *testing.T) {
		syncStandBys := `ANY 4("a",*,b)`
		resp, err := parsePGSyncStandby(syncStandBys)

		assert.Nil(t, err)
		assert.Equal(t, quorum, resp.Types)
		assert.True(t, resp.HasStar)
		assert.True(t, resp.Members.Contains("a"))
		assert.Equal(t, 4, resp.Amount)
	})

	t.Run("custom values", func(t *testing.T) {
		syncStandBys := ` a , b `
		resp, err := parsePGSyncStandby(syncStandBys)

		assert.Nil(t, err)
		assert.Equal(t, priority, resp.Types)
		assert.False(t, resp.HasStar)
		assert.True(t, resp.Members.Contains("a"))
		assert.True(t, resp.Members.Contains("b"))
		assert.Equal(t, 1, resp.Amount)
	})

	t.Run("can't parse synchronous standby name", func(t *testing.T) {
		syncStandBys := `ANY 4("a" b,"c c")`
		resp, err := parsePGSyncStandby(syncStandBys)

		assert.NotNil(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "Unparseable synchronous_standby_names value")
	})
}

func TestParsePGLsn(t *testing.T) {
	t.Run("legal lsn str", func(t *testing.T) {
		lsnStr := "16/B374D848"

		lsn := parsePgLsn(lsnStr)
		assert.Equal(t, int64(97500059720), lsn)
	})

	t.Run("illegal lsn str", func(t *testing.T) {
		lsnStr := "B374D848"

		lsn := parsePgLsn(lsnStr)
		assert.Equal(t, int64(0), lsn)
	})
}

func TestParseSingleQuery(t *testing.T) {
	t.Run("legal query response", func(t *testing.T) {
		queryResp := `[{"name":"primary_conninfo","setting":"host=pg-pg-replication-0.pg-pg-replication-headless port=5432 user=postgres application_name=my-application"}]`

		result, err := parseSingleQuery(queryResp)
		assert.Nil(t, err)
		assert.Equal(t, "host=pg-pg-replication-0.pg-pg-replication-headless port=5432 user=postgres application_name=my-application", result["setting"])
	})

	t.Run("illegal query response", func(t *testing.T) {
		queryResp := `{"name":"primary_conninfo","setting":"host=pg-pg-replication-0.pg-pg-replication-headless 
						port=5432 user=postgres application_name=my-application"}`

		result, err := parseSingleQuery(queryResp)
		assert.NotNil(t, err)
		assert.Nil(t, result)
	})
}

func TestParsePrimaryConnInfo(t *testing.T) {
	t.Run("legal primary conn info str", func(t *testing.T) {
		primaryConnInfoStr := "host=pg-pg-replication-0.pg-pg-replication-headless port=5432 user=postgres application_name=my-application"

		result := parsePrimaryConnInfo(primaryConnInfoStr)
		assert.NotNil(t, result)
		assert.Equal(t, "pg-pg-replication-0.pg-pg-replication-headless", result["host"])
		assert.Equal(t, "5432", result["port"])
		assert.Equal(t, "postgres", result["user"])
		assert.Equal(t, "my-application", result["application_name"])
	})

	t.Run("illegal primary conn info str", func(t *testing.T) {
		primaryConnInfoStr := "host pg-pg-replication-0.pg-pg-replication-headless port 5432 user postgres application_name my-application"

		result := parsePrimaryConnInfo(primaryConnInfoStr)
		assert.NotNil(t, result)
		assert.Equal(t, map[string]string{}, result)
	})
}
