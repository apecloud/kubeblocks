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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/dlclark/regexp2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cast"
	"golang.org/x/exp/slices"

	"github.com/apecloud/kubeblocks/pkg/lorry/dcs"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines"
)

var (
	ClusterHasNoLeader = errors.New("cluster has no leader now")
)

var fs = afero.NewOsFs()

const (
	PGDATA  = "PGDATA"
	PGMAJOR = "PG_MAJOR"
)

const (
	ReplicationMode = "replication_mode"
	SyncStandBys    = "sync_standbys"
	PrimaryConnInfo = "primary_conninfo"
	TimeLine        = "timeline"
)

const (
	Asynchronous = "asynchronous"
	Synchronous  = "synchronous"
)

const (
	first            = "first"
	star             = "star"
	ident            = "ident"
	doubleQuote      = "double_quote"
	space            = "space"
	anyA             = "any"
	num              = "num"
	comma            = "comma"
	parenthesisStart = "parenthesis_start"
	parenthesisEnd   = "parenthesis_end"
	quorum           = "quorum"
	priority         = "priority"
	off              = "off"
)

type PgBaseIFace interface {
	GetMemberRoleWithHost(ctx context.Context, host string) (string, error)
	IsMemberHealthy(ctx context.Context, cluster *dcs.Cluster, member *dcs.Member) bool
	Query(ctx context.Context, sql string) (result []byte, err error)
	Exec(ctx context.Context, sql string) (result int64, err error)
}

type PgIFace interface {
	engines.DBManager
	PgBaseIFace
}

type PgxIFace interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error)
	Ping(ctx context.Context) error
}

// PgxPoolIFace is interface representing pgx pool
type PgxPoolIFace interface {
	PgxIFace
	Acquire(ctx context.Context) (*pgxpool.Conn, error)
	Close()
}

type LocalCommand interface {
	Run() error
	GetStdout() *bytes.Buffer
	GetStderr() *bytes.Buffer
}

type ConsensusMemberHealthStatus struct {
	Connected   bool
	LogDelayNum int64
}

type PidFile struct {
	pid     int32
	dataDir string
	startTS int64
	port    int
}

func readPidFile(dataDir string) (*PidFile, error) {
	file := &PidFile{}
	f, err := fs.Open(dataDir + "/postmaster.pid")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	scanner := bufio.NewScanner(f)
	var text []string
	for scanner.Scan() {
		text = append(text, scanner.Text())
	}

	pid, err := strconv.ParseInt(text[0], 10, 32)
	if err != nil {
		return nil, err
	}
	file.pid = int32(pid)
	file.dataDir = text[1]
	startTS, _ := strconv.ParseInt(text[2], 10, 64)
	file.startTS = startTS
	port, err := strconv.ParseInt(text[3], 10, 32)
	if err != nil {
		return nil, err
	}
	file.port = int(port)

	return file, nil
}

type PGStandby struct {
	Types   string
	Amount  int
	Members mapset.Set[string]
	HasStar bool
}

func ParsePGSyncStandby(standbyRow string) (*PGStandby, error) {
	pattern := `(?P<first> [fF][iI][rR][sS][tT] )
				|(?P<any> [aA][nN][yY] )
				|(?P<space> \s+ )
				|(?P<ident> [A-Za-z_][A-Za-z_0-9\$]* )
				|(?P<double_quote> " (?: [^"]+ | "" )* " )
				|(?P<star> [*] )
				|(?P<num> \d+ )
				|(?P<comma> , )
				|(?P<parenthesis_start> \( )
				|(?P<parenthesis_end> \) )
				|(?P<JUNK> . ) `
	patterns := []string{
		`(?P<first> [fF][iI][rR][sS][tT]) `,
		`(?P<any> [aA][nN][yY]) `,
		`(?P<space> \s+ )`,
		`(?P<ident> [A-Za-z_][A-Za-z_0-9\$]* )`,
		`(?P<double_quote> "(?:[^"]+|"")*") `,
		`(?P<star> [*] )`,
		`(?P<num> \d+ )`,
		`(?P<comma> , )`,
		`(?P<parenthesis_start> \( )`,
		`(?P<parenthesis_end> \) )`,
		`(?P<JUNK> .) `,
	}
	result := &PGStandby{
		Types:   off,
		Members: mapset.NewSet[string](),
	}

	rs := make([]*regexp2.Regexp, len(patterns))
	var patternPrefix string
	for i, p := range patterns {
		if i != 0 {
			patternPrefix += `|`
		}
		patternPrefix += p
		rs[i] = regexp2.MustCompile(patternPrefix, regexp2.IgnorePatternWhitespace+regexp2.RE2)
	}

	r := regexp2.MustCompile(pattern, regexp2.RE2+regexp2.IgnorePatternWhitespace)
	groupNames := r.GetGroupNames()

	match, err := r.FindStringMatch(standbyRow)
	if err != nil {
		return nil, err
	}

	var matches [][]string
	start := 0
	for match != nil {
		nums := getMatchLastGroupNumber(rs, standbyRow, match.String(), start)
		if groupNames[nums+2] != space {
			matches = append(matches, []string{groupNames[nums+2], match.String(), strconv.FormatInt(int64(start), 10)})
		}
		start = match.Index + match.Length

		match, err = r.FindNextMatch(match)
		if err != nil {
			return nil, err
		}
	}

	length := len(matches)
	if length == 0 {
		return result, nil
	}
	var syncList [][]string
	switch {
	case length >= 3 && matches[0][0] == anyA && matches[1][0] == num && matches[2][0] == parenthesisStart && matches[length-1][0] == parenthesisEnd:
		result.Types = quorum
		result.Amount = cast.ToInt(matches[1][1])
		syncList = matches[3 : length-1]
	case length >= 3 && matches[0][0] == first && matches[1][0] == num && matches[2][0] == parenthesisStart && matches[length-1][0] == parenthesisEnd:
		result.Types = priority
		result.Amount = cast.ToInt(matches[1][1])
		syncList = matches[3 : length-1]
	case length >= 2 && matches[0][0] == num && matches[1][0] == parenthesisStart && matches[length-1][0] == parenthesisEnd:
		result.Types = priority
		result.Amount = cast.ToInt(matches[0][1])
		syncList = matches[2 : length-1]
	default:
		result.Types = priority
		result.Amount = 1
		syncList = matches
	}

	for i, sync := range syncList {
		switch {
		case i%2 == 1: // odd elements are supposed to be commas
			if len(syncList) == i+1 {
				return nil, errors.Errorf("Unparseable synchronous_standby_names value: Unexpected token %s %s at %s", sync[0], sync[1], sync[2])
			} else if sync[0] != comma {
				return nil, errors.Errorf("Unparseable synchronous_standby_names value: Got token %s %s while expecting comma at %s", sync[0], sync[1], sync[2])
			}
		case slices.Contains([]string{ident, first, anyA}, sync[0]):
			result.Members.Add(sync[1])
		case sync[0] == star:
			result.Members.Add(sync[1])
			result.HasStar = true
		case sync[0] == doubleQuote:
			result.Members.Add(strings.ReplaceAll(sync[1][1:len(sync)-1], `""`, `"`))
		default:
			return nil, errors.Errorf("Unparseable synchronous_standby_names value: Unexpected token %s %s at %s", sync[0], sync[1], sync[2])
		}
	}

	return result, nil
}

func getMatchLastGroupNumber(rs []*regexp2.Regexp, str string, substr string, start int) int {
	for i := len(rs) - 1; i >= 0; i-- {
		match, err := rs[i].FindStringMatchStartingAt(str, start)
		if match == nil || err != nil {
			return i
		}
		if match.String() != substr {
			return i
		}
	}

	return -1
}

type HistoryFile struct {
	History []History
}

type History struct {
	ParentTimeline int64
	SwitchPoint    int64
}

func ParseHistory(str string) *HistoryFile {
	result := &HistoryFile{
		History: []History{},
	}

	lines := strings.Split(str, "\n")
	for _, line := range lines {
		values := strings.Split(line, "|")
		if len(values) <= 1 {
			continue
		}
		content := strings.TrimSpace(values[1])
		history := strings.Split(content, " ")
		if len(history) > 2 {
			result.History = append(result.History, History{
				ParentTimeline: cast.ToInt64(history[0]),
				SwitchPoint:    ParsePgLsn(history[1]),
			})
		}
	}

	return result
}

func ParsePgLsn(str string) int64 {
	list := strings.Split(str, "/")
	if len(list) < 2 {
		return 0
	}

	prefix, _ := strconv.ParseInt(list[0], 16, 64)
	suffix, _ := strconv.ParseInt(list[1], 16, 64)
	return prefix*0x100000000 + suffix
}

func FormatPgLsn(lsn int64) string {
	return fmt.Sprintf("%X/%08X", lsn>>32, lsn&0xFFFFFFFF)
}

func ParsePrimaryConnInfo(str string) map[string]string {
	infos := strings.Split(str, " ")
	result := make(map[string]string)

	for _, info := range infos {
		v := strings.Split(info, "=")
		if len(v) >= 2 {
			result[v[0]] = v[1]
		}
	}

	return result
}

func ParsePgWalDumpError(errorInfo string, lsnStr string) string {
	prefixPattern := fmt.Sprintf("error in WAL record at %s: invalid record length at ", lsnStr)
	suffixPattern := ": wanted "

	startIndex := strings.Index(errorInfo, prefixPattern) + len(prefixPattern)
	endIndex := strings.Index(errorInfo, suffixPattern)

	if startIndex == -1 || endIndex == -1 {
		return ""
	}

	return errorInfo[startIndex:endIndex]
}
