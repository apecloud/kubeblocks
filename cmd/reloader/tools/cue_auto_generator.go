package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	FileNotExist  = -1
	FileReadError = -2

	RecordFieldCount = 8
	RecordMinField   = 7

	NameField          = 0
	DefaultValueField  = 1
	ValueRestrictField = 2
	ImmutableField     = 3
	ValidTYpeField     = 5
	DataTypeField      = 6
	DocField           = 7
)

var (
	prefixString = ""
	filePath     = ""
)

type ValueType string

const (
	BooleanTYpe = "boolean"
	IntegerType = "integer"
	FloatType   = "float"
	StringType  = "string"
	ListType    = "list" // for string
)

type ValueParser func(s string) (interface{}, error)

func EmptyParser(s string) (interface{}, error) {
	return s, nil
}

var ValueTypeParserMap = map[ValueType]ValueParser{
	BooleanTYpe: func(s string) (interface{}, error) {
		return strconv.ParseBool(s)
	},
	IntegerType: func(s string) (interface{}, error) {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			return v, nil
		}
		return strconv.ParseUint(s, 10, 64)
	},
	FloatType: func(s string) (interface{}, error) {
		return strconv.ParseFloat(s, 64)
	},
	StringType: EmptyParser,
	ListType:   EmptyParser,
}

func main() {
	// file context format
	// parameter name | default value | value restrict | parameter type |  immutable | value type | static/dynamic parameter | doc
	// e.g.
	// default_authentication_plugin\tmysql_native_password\tmysql_native_password, sha256_password, caching_sha2_password\tfalse\tengine-default\tstatic\tstring\tThe default authentication plugin

	flag.StringVar(&filePath, "file-path", "", "The generate cue scripts from file.")
	flag.StringVar(&prefixString, "output-prefix", prefixString, "prefix, default: \"\"")
	flag.Parse()

	f, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("open file[%s] failed. error: %v", filePath, err)
		os.Exit(FileNotExist)
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			fmt.Printf("readline failed. error: %v", err)
			os.Exit(FileReadError)
		}
		fields := strings.SplitN(scanner.Text(), "\t", RecordFieldCount)
		if len(fields) < RecordMinField {
			continue
		}

		wrapOutputCuelang(ConstructParameterType(fields), os.Stdout)
	}
}

func wrapOutputCuelang(parameter *ParameterType, writer io.Writer) {
	if !validateParameter(parameter) {
		return
	}

	wrapper := CueWrapper{
		writer:        writer,
		ParameterType: parameter,
	}

	wrapper.output()
}

func validateParameter(parameter *ParameterType) bool {
	return parameter != nil &&
		parameter.Name != "" &&
		parameter.Type != ""
}

func ConstructParameterType(fields []string) *ParameterType {
	param := ParameterType{
		Name:         fields[NameField],
		Type:         ValueType(fields[DataTypeField]),
		DefaultValue: strings.TrimSpace(fields[DefaultValueField]),
		IsStatic:     true,
		Immutable:    false,
	}
	if len(fields) > RecordMinField {
		param.Document = fields[DocField]
	}

	if r, err := strconv.ParseBool(fields[ImmutableField]); err == nil {
		param.Immutable = r
	}

	if fields[ValidTYpeField] == "dynamic" {
		param.IsStatic = false
	}

	if param.DefaultValue != "" && param.DefaultValue[0] == '{' {
		param.DefaultValue = ""
	}

	switch param.Type {
	case BooleanTYpe:
		param.Type = IntegerType
	case ListType:
		param.Type = StringType
	}

	param.ParameterRestrict = parseParameterRestrict(fields[ValueRestrictField], param.Type)
	return &param
}

type ParameterRestrict struct {
	isEnum bool

	Min interface{}
	Max interface{}

	EnumList []interface{}
}

type ParameterType struct {
	Name         string
	Type         ValueType
	DefaultValue string
	IsStatic     bool

	Immutable         bool
	ParameterRestrict *ParameterRestrict
	Document          string
}

func generateRestrictParam(buffer *bytes.Buffer, restrict *ParameterRestrict, valueType ValueType) {
	buffer.WriteString(" & ")
	if restrict.isEnum {
		for i := 0; i < len(restrict.EnumList); i++ {
			if i > 0 {
				buffer.WriteString(" | ")
			}
			generateElemValue(buffer, restrict.EnumList[i], valueType)
		}
	} else {
		buffer.WriteString(">= ")
		generateElemValue(buffer, restrict.Min, valueType)
		buffer.WriteString(" & <= ")
		generateElemValue(buffer, restrict.Max, valueType)
	}
}

func generateElemValue(buffer *bytes.Buffer, value interface{}, valueType ValueType) {
	if valueType == StringType {
		buffer.WriteString("\"")
	}
	buffer.WriteString(fmt.Sprintf("%v", value))
	if valueType == StringType {
		buffer.WriteString("\"")
	}
}

func parseValue(s string, valueType ValueType) interface{} {
	v, err := ValueTypeParserMap[valueType](s)
	if err != nil {
		fmt.Printf("parse type[%s] value[%s] failed!", valueType, s)
		panic("")
	}
	return v
}

func parseParameterRestrict(s string, valueType ValueType) *ParameterRestrict {
	var (
		IntegerRangeRegex = regexp.MustCompile(`([\+\-]?\d+)-([\+\-]?\d+)`)
		FloatRangeRegex   = regexp.MustCompile(`([\+\-]?\d+(\.\d*)?)-([\+\-]?\d+(\.\d*)?)`)
	)

	if s == "" {
		return nil
	}

	switch valueType {
	case IntegerType:
		r := IntegerRangeRegex.FindStringSubmatch(s)
		if len(r) > 0 {
			t := &ParameterRestrict{
				isEnum: false,
				Min:    parseValue(r[1], valueType),
				Max:    parseValue(r[2], valueType),
			}
			return t
		}
	case FloatType:
		r := FloatRangeRegex.FindStringSubmatch(s)
		if len(r) > 0 {
			t := &ParameterRestrict{
				isEnum: false,
				Min:    parseValue(r[1], valueType),
				Max:    parseValue(r[3], valueType),
			}
			return t
		}
	}

	return parseListParameter(s, valueType)
}

func parseListParameter(s string, valueType ValueType) *ParameterRestrict {
	values := strings.Split(s, ",")
	if len(values) == 0 {
		return nil
	}

	p := &ParameterRestrict{
		isEnum: true,
	}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if len(v) == 0 {
			continue
		}

		if typeValue, err := ValueTypeParserMap[valueType](v); err != nil {
			fmt.Printf("parse failed: [%s] [%s] [%s]\n", s, v, valueType)
			// panic(fmt.Sprintf("parse faild: [%s] [%s]", s, v))
		} else {
			AddRestrictValue(p, typeValue)
		}
	}
	if len(p.EnumList) == 0 {
		return nil
	}
	return p
}

func AddRestrictValue(p *ParameterRestrict, value interface{}) {
	if p.EnumList == nil {
		p.EnumList = make([]interface{}, 0)
	}

	p.EnumList = append(p.EnumList, value)
}

type CueWrapper struct {
	writer io.Writer
	*ParameterType
}

func (w *CueWrapper) output() int {
	var buffer bytes.Buffer
	w.generateCueDocument(&buffer)
	w.generateCueTypeParameter(&buffer)
	w.generateCueRestrict(&buffer)
	w.generateCueDefaultValue(&buffer)

	_ = buffer.WriteByte('\n')
	_ = buffer.WriteByte('\n')
	b, _ := w.writer.Write(buffer.Bytes())
	return b
}

func (w *CueWrapper) generateCueDefaultValue(buffer *bytes.Buffer) {
	if w.DefaultValue == "" {
		return
	}
	buffer.WriteString(" | *")
	generateElemValue(buffer, w.DefaultValue, w.Type)
}

func (w *CueWrapper) generateCueRestrict(buffer *bytes.Buffer) {
	if w.ParameterRestrict != nil {
		generateRestrictParam(buffer, w.ParameterRestrict, w.Type)
	}
}

func (w *CueWrapper) generateCueTypeParameter(buffer *bytes.Buffer) {
	buffer.WriteString(prefixString)
	if strings.ContainsAny(w.Name, "-.") {
		buffer.WriteByte('"')
		buffer.WriteString(w.Name)
		buffer.WriteByte('"')
	} else {
		buffer.WriteString(w.Name)
	}

	if w.DefaultValue == "" {
		buffer.WriteByte('?')
	}
	buffer.WriteString(": ")
	if w.Type == IntegerType {
		buffer.WriteString("int")
	} else {
		buffer.WriteString(string(w.Type))
	}
}

func (w *CueWrapper) generateCueDocument(buffer *bytes.Buffer) {
	if w.Document != "" {
		buffer.WriteString(prefixString)
		buffer.WriteString("// ")
		buffer.WriteString(w.Document)
		buffer.WriteByte('\n')
	}
}
