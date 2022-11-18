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

var dataValueParser = map[ValueType]ValueParser{
	BooleanTYpe: func(s string) (interface{}, error) {
		return strconv.ParseBool(s)
	},
	IntegerType: func(s string) (interface{}, error) {
		if v, err := strconv.ParseInt(s, 10, 64); err != nil {
			return strconv.ParseUint(s, 10, 64)
		} else {
			return v, nil
		}
	},
	FloatType: func(s string) (interface{}, error) {
		return strconv.ParseFloat(s, 64)
	},
	StringType: EmptyParser,
	ListType:   EmptyParser,
}

func main() {
	var filePath string
	flag.StringVar(&filePath, "file-path", "", "The generate cue scripts from file.")
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

		processParameter(fields)
	}

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

func processParameter(fields []string) {

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
	printParameterWithCue(param, os.Stdout)
}

func printParameterWithCue(param ParameterType, writer io.Writer) int {
	var (
		buffer        bytes.Buffer
		PREFIX_STRING = "        "
	)

	if len(param.Document) > 0 {
		buffer.WriteString(PREFIX_STRING)
		buffer.WriteString("// ")
		buffer.WriteString(param.Document)
		buffer.WriteByte('\n')
	}

	buffer.WriteString(PREFIX_STRING)
	if strings.ContainsAny(param.Name, "-.") {
		buffer.WriteByte('"')
		buffer.WriteString(param.Name)
		buffer.WriteByte('"')
	} else {
		buffer.WriteString(param.Name)
	}

	if param.DefaultValue == "" {
		buffer.WriteByte('?')
	}
	buffer.WriteString(": ")
	if param.Type == IntegerType {
		buffer.WriteString("int")
	} else {
		buffer.WriteString(string(param.Type))
	}
	restrict := param.ParameterRestrict
	if restrict != nil {
		printRestrictParam(&buffer, restrict, param.Type)
	}

	if len(param.DefaultValue) > 0 {
		printDefaultValue(&buffer, param.DefaultValue, param.Type)
	}

	buffer.WriteString("\n\n")

	num, _ := writer.Write(buffer.Bytes())
	return num
}

func printDefaultValue(buffer *bytes.Buffer, value string, valueType ValueType) {
	buffer.WriteString(" | *")
	printElem(buffer, value, valueType)
}

func printRestrictParam(buffer *bytes.Buffer, restrict *ParameterRestrict, valueType ValueType) {
	buffer.WriteString(" & ")
	if restrict.isEnum {
		for i := 0; i < len(restrict.EnumList); i++ {
			if i > 0 {
				buffer.WriteString(" | ")
			}
			printElem(buffer, restrict.EnumList[i], valueType)
		}
	} else {
		buffer.WriteString(">= ")
		printElem(buffer, restrict.Min, valueType)
		buffer.WriteString(" & <= ")
		printElem(buffer, restrict.Max, valueType)
		// buffer.WriteString(fmt.Sprintf("%v", restrict.Min))
		// buffer.WriteString(fmt.Sprintf("%v", restrict.Max))
	}
}

func printElem(buffer *bytes.Buffer, value interface{}, valueType ValueType) {
	if valueType == StringType {
		buffer.WriteString("\"")
	}
	buffer.WriteString(fmt.Sprintf("%v", value))
	if valueType == StringType {
		buffer.WriteString("\"")
	}
}

func parseValue(s string, valueType ValueType) interface{} {
	v, err := dataValueParser[valueType](s)
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

		if typeValue, err := dataValueParser[valueType](v); err != nil {
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
