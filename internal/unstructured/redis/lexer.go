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

package redis

import (
	"bufio"
	"reflect"
	"sort"
	"strings"
)

type Item struct {
	LineNo   int
	Comments []string
	Values   []string
}

type Lexer struct {
	lines []string
	dict  map[string][]Item

	isUpdated bool
}

const trimChars = " \r\n\t"

func (i *Item) addToken(str string) {
	if i.Values == nil {
		i.Values = make([]string, 0)
	}
	i.Values = append(i.Values, str)
}

func (l *Lexer) GetItem(key string) []Item {
	return l.dict[key]
}

func (l *Lexer) ParseParameter(paramLine string, paramID int) (Item, error) {
	paramItem := Item{LineNo: paramID}
	itemWrap := fsm{
		param:           &paramItem,
		splitCharacters: trimChars,
	}
	return paramItem, itemWrap.Parse(paramLine)
}

func (l *Lexer) appendConfigLine(parameterLine string) {
	l.lines = append(l.lines, parameterLine)
}

func (l *Lexer) AppendValidParameter(param Item, fromNo int) {
	newItem := param
	key := newItem.Values[0]
	l.addParameterComments(&newItem, fromNo+1, param.LineNo)
	if _, ok := l.dict[key]; !ok {
		l.dict[key] = make([]Item, 0)
	}
	l.dict[key] = append(l.dict[key], newItem)
	l.isUpdated = true
}

func (l *Lexer) addParameterComments(param *Item, start, end int) {
	if start+1 >= end {
		return
	}
	param.Comments = l.lines[start:end]
}

func (l *Lexer) Load(str string) error {
	var err error

	param := Item{LineNo: -1}
	scanner := bufio.NewScanner(strings.NewReader(str))
	l.dict = make(map[string][]Item)
	for scanner.Scan() {
		parameterLine := strings.Trim(scanner.Text(), trimChars)
		lineNo := len(l.lines)
		l.appendConfigLine(parameterLine)
		if parameterLine == "" || parameterLine[0] == '#' {
			continue
		}
		lastScanNo := param.LineNo
		if param, err = l.ParseParameter(parameterLine, lineNo); err != nil {
			return err
		}
		l.AppendValidParameter(param, lastScanNo)
	}

	l.isUpdated = false
	return nil
}

func (l *Lexer) RemoveParameter(it *Item) {
	v, ok := l.dict[it.Values[0]]
	if !ok {
		return
	}

	index := -1
	for i := range v {
		if reflect.DeepEqual(&v[i], it) {
			index = i
			break
		}
	}

	if index >= 0 {
		l.dict[it.Values[0]] = append(v[:index], v[index+1:]...)
	}
	l.isUpdated = true
}

func (l *Lexer) SortParameters() []Item {
	items := make([]Item, 0)
	for _, v := range l.dict {
		items = append(items, v...)
	}
	sort.SliceStable(items, func(i, j int) bool {
		no1 := items[i].LineNo
		no2 := items[j].LineNo
		if no1 == no2 {
			return strings.Compare(items[i].Values[0], items[j].Values[0]) < 0
		}
		return no1 < no2
	})
	return items
}

func (l *Lexer) Empty() bool {
	return len(l.dict) == 0
}

func (l *Lexer) GetAllParams() map[string][]Item {
	return l.dict
}

func (l Lexer) IsUpdated() bool {
	return l.isUpdated
}

func (l *Lexer) ToString() string {
	return strings.Join(l.lines, "\n")
}
