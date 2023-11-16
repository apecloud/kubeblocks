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

package mysql

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	singleValueInterval = regexp.MustCompile("^([0-9]+)$")
	multiValueInterval  = regexp.MustCompile("^([0-9]+)[-]([0-9]+)$")
)

// GTIDItem represents an item in a set of GTID ranges,
// for example, the item: "ee194423-3040-11ee-9393-eab5dfc9b22a:1-5:8-10"
type GTIDItem struct {
	ServerUUID string
	Ranges     string
}

func NewGTIDItem(gtidString string) (*GTIDItem, error) {
	gtidString = strings.TrimSpace(gtidString)
	tokens := strings.SplitN(gtidString, ":", 2)
	if len(tokens) != 2 {
		return nil, fmt.Errorf("GTID wrong format: %s", gtidString)
	}
	if tokens[0] == "" {
		return nil, fmt.Errorf("GTID no server UUID: %s", tokens[0])
	}
	if tokens[1] == "" {
		return nil, fmt.Errorf("GTID no range: %s", tokens[1])
	}
	gtidItem := &GTIDItem{ServerUUID: tokens[0], Ranges: tokens[1]}
	return gtidItem, nil
}

func (gtid *GTIDItem) String() string {
	return fmt.Sprintf("%s:%s", gtid.ServerUUID, gtid.Ranges)
}

func (gtid *GTIDItem) Explode() (result []*GTIDItem) {
	intervals := strings.Split(gtid.Ranges, ":")
	for _, interval := range intervals {
		if submatch := multiValueInterval.FindStringSubmatch(interval); submatch != nil {
			intervalStart, _ := strconv.Atoi(submatch[1])
			intervalEnd, _ := strconv.Atoi(submatch[2])
			for i := intervalStart; i <= intervalEnd; i++ {
				result = append(result, &GTIDItem{ServerUUID: gtid.ServerUUID, Ranges: fmt.Sprintf("%d", i)})
			}
		} else if submatch := singleValueInterval.FindStringSubmatch(interval); submatch != nil {
			result = append(result, &GTIDItem{ServerUUID: gtid.ServerUUID, Ranges: interval})
		}
	}
	return result
}

type GTIDSet struct {
	Items []*GTIDItem
}

func NewOracleGtidSet(gtidSet string) (res *GTIDSet, err error) {
	res = &GTIDSet{}

	gtidSet = strings.TrimSpace(gtidSet)
	if gtidSet == "" {
		return res, nil
	}
	gtids := strings.Split(gtidSet, ",")
	for _, gtid := range gtids {
		gtid = strings.TrimSpace(gtid)
		if gtid == "" {
			continue
		}
		if gtidRange, err := NewGTIDItem(gtid); err == nil {
			res.Items = append(res.Items, gtidRange)
		} else {
			return res, err
		}
	}
	return res, nil
}

func (gtidSet *GTIDSet) RemoveUUID(uuid string) (removed bool) {
	var filteredEntries []*GTIDItem
	for _, item := range gtidSet.Items {
		if item.ServerUUID == uuid {
			removed = true
		} else {
			filteredEntries = append(filteredEntries, item)
		}
	}
	if removed {
		gtidSet.Items = filteredEntries
	}
	return removed
}

func (gtidSet *GTIDSet) RetainUUID(uuid string) (anythingRemoved bool) {
	return gtidSet.RetainUUIDs([]string{uuid})
}

func (gtidSet *GTIDSet) RetainUUIDs(uuids []string) (anythingRemoved bool) {
	retainUUIDs := map[string]bool{}
	for _, uuid := range uuids {
		retainUUIDs[uuid] = true
	}
	var filteredEntries []*GTIDItem
	for _, item := range gtidSet.Items {
		if retainUUIDs[item.ServerUUID] {
			filteredEntries = append(filteredEntries, item)
		} else {
			anythingRemoved = true
		}
	}
	if anythingRemoved {
		gtidSet.Items = filteredEntries
	}
	return anythingRemoved
}

func (gtidSet *GTIDSet) SharedUUIDs(other *GTIDSet) (shared []string) {
	gtidSetUUIDs := map[string]bool{}
	for _, item := range gtidSet.Items {
		gtidSetUUIDs[item.ServerUUID] = true
	}
	for _, item := range other.Items {
		if gtidSetUUIDs[item.ServerUUID] {
			shared = append(shared, item.ServerUUID)
		}
	}
	return shared
}

func (gtidSet *GTIDSet) Explode() (result []*GTIDItem) {
	for _, entries := range gtidSet.Items {
		result = append(result, entries.Explode()...)
	}
	return result
}

func (gtidSet *GTIDSet) String() string {
	var tokens []string
	for _, item := range gtidSet.Items {
		tokens = append(tokens, item.String())
	}
	return strings.Join(tokens, ",")
}

func (gtidSet *GTIDSet) IsEmpty() bool {
	return len(gtidSet.Items) == 0
}
