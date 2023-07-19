package postgres

import (
	"encoding/json"
	"strconv"
	"strings"

	mapset "github.com/deckarep/golang-set"
	"github.com/dlclark/regexp2"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

const (
	asynchronous = "asynchronous"
	synchronous  = "synchronous"
)

type PidFile struct {
	pid     int32
	dataDir string
	startTs int64
	port    int
}

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

type PGStandby struct {
	Types   string
	Amount  int
	Members mapset.Set
	HasStar bool
}

func parsePGSyncStandby(standbyRow string) (*PGStandby, error) {
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
		Members: mapset.NewSet(),
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
		num := getMatchLastGroupNumber(rs, standbyRow, match.String(), start)
		if groupNames[num+2] != space {
			matches = append(matches, []string{groupNames[num+2], match.String(), strconv.FormatInt(int64(start), 10)})
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
	if matches[0][0] == anyA && matches[1][0] == num && matches[2][0] == parenthesisStart && matches[length-1][0] == parenthesisEnd {
		result.Types = quorum
		amount, err := strconv.Atoi(matches[1][1])
		if err != nil {
			amount = 0
		}
		result.Amount = amount
		syncList = matches[3 : length-1]
	} else if matches[0][0] == first && matches[1][0] == num && matches[2][0] == parenthesisStart && matches[length-1][0] == parenthesisEnd {
		result.Types = priority
		amount, err := strconv.Atoi(matches[1][1])
		if err != nil {
			amount = 0
		}
		result.Amount = amount
		syncList = matches[3 : length-1]
	} else if matches[0][0] == num && matches[1][0] == parenthesisStart && matches[length-1][0] == parenthesisEnd {
		result.Types = priority
		amount, err := strconv.Atoi(matches[0][1])
		if err != nil {
			amount = 0
		}
		result.Amount = amount
		syncList = matches[2 : length-1]
	} else {
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
			result.Members.Add(strings.Replace(sync[1][1:len(sync)-1], `""`, `"`, -1))
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

type DataBaseStatus struct {
	isLeader    bool
	WalPosition int64
}

type history struct {
	parentTimeline int64
	switchPoint    int64
}

func parsePgLsn(str string) int64 {
	list := strings.Split(str, "/")
	prefix, _ := strconv.ParseInt(list[0], 16, 64)
	suffix, _ := strconv.ParseInt(list[1], 16, 64)
	return prefix*0x100000000 + suffix
}

func parseSingleQuery(str string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	str = strings.Trim(str, "[]")

	err := json.Unmarshal([]byte(str), &result)
	if err != nil {
		return nil, errors.Errorf("json unmarshal failed, err:%v", err)
	}

	return result, nil
}

func parsePrimaryConnInfo(str string) map[string]string {
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
