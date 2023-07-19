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
