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

package unstructured

import (
	"bufio"
	"reflect"
	"strings"
)

type item struct {
	comments []string
	lineNo   int
	values   []string
}

type lexer struct {
	lines []string
	dict  map[string][]item
}

const trimChars = " \r\n\t"

func (i *item) addToken(str string) {
	if i.values == nil {
		i.values = make([]string, 0)
	}
	i.values = append(i.values, str)
}

func (l *lexer) parseParameter(paramLine string, paramID int) (item, error) {
	paramItem := item{lineNo: paramID}
	itemWrap := fsm{
		param:           &paramItem,
		splitCharacters: trimChars,
	}
	return paramItem, itemWrap.Parse(paramLine)
}

func (l *lexer) appendConfigLine(parameterLine string) {
	l.lines = append(l.lines, parameterLine)
}

func (l *lexer) appendValidConfigParameter(param item, fromNo int) {
	newItem := param
	key := newItem.values[0]
	l.addParameterComments(&newItem, fromNo+1, param.lineNo)
	if _, ok := l.dict[key]; !ok {
		l.dict[key] = make([]item, 0)
	}
	l.dict[key] = append(l.dict[key], newItem)
}

func (l *lexer) addParameterComments(param *item, start, end int) {
	if start+1 >= end {
		return
	}
	param.comments = l.lines[start:end]
}

func (l *lexer) load(str string) error {
	var err error

	param := item{lineNo: -1}
	scanner := bufio.NewScanner(strings.NewReader(str))
	for scanner.Scan() {
		parameterLine := strings.Trim(scanner.Text(), trimChars)
		lineNo := len(l.lines)
		l.appendConfigLine(parameterLine)
		if parameterLine == "" || parameterLine[0] == '#' {
			continue
		}
		lastScanNo := param.lineNo
		if param, err = l.parseParameter(parameterLine, lineNo); err != nil {
			return err
		}
		l.appendValidConfigParameter(param, lastScanNo)
	}
	return nil
}

func (l *lexer) remove(it *item) {
	v, ok := l.dict[it.values[0]]
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
		l.dict[it.values[0]] = append(v[:index], v[index+1:]...)
	}
}
