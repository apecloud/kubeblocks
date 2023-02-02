package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
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
	prefixString        = ""
	filePath            = ""
	typeName            = "MyParameter"
	ignoreStringDefault = true
	booleanPromotion    = false
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

var numberRegex = regexp.MustCompile(`^\d+$`)

var ValueTypeParserMap = map[ValueType]ValueParser{
	BooleanTYpe: func(s string) (interface{}, error) {
		if booleanPromotion && numberRegex.MatchString(s) {
			return nil, cfgcore.MakeError("boolean parser failed")
		}
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
	flag.StringVar(&typeName, "type-name", typeName, "cue parameter type name. ")
	flag.BoolVar(&ignoreStringDefault, "ignore-string-default", ignoreStringDefault, "ignore string default. ")
	flag.BoolVar(&booleanPromotion, "boolean-promotion", booleanPromotion, "enable using OFF or ON. ")
	flag.Parse()

	f, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("open file[%s] failed. error: %v", filePath, err)
		os.Exit(FileNotExist)
	}

	writer := os.Stdout
	scanner := bufio.NewScanner(f)
	wrapOutputTypeDefineBegin(typeName, writer)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			fmt.Printf("readline failed. error: %v", err)
			os.Exit(FileReadError)
		}
		fields := strings.SplitN(scanner.Text(), "\t", RecordFieldCount)
		if len(fields) < RecordMinField {
			continue
		}
		wrapOutputCueLang(ConstructParameterType(fields), writer)
	}
	wrapOutputTypeDefineEnd(writer)
}

func wrapOutputTypeDefineEnd(writer io.Writer) int {
	r, _ := writer.Write([]byte(fmt.Sprintf("\n  %s...\n%s}", prefixString, prefixString)))
	return r
}

func wrapOutputTypeDefineBegin(typeName string, writer io.Writer) int {
	r, _ := writer.Write([]byte(fmt.Sprintf("%s#%s: {\n\n", prefixString, typeName)))
	return r
}

func wrapOutputCueLang(parameter *ParameterType, writer io.Writer) {
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
	// type promotion
	types := []ValueType{BooleanTYpe, IntegerType, StringType}
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
	if param.Type == ListType {
		param.Type = StringType
	}
	checkAndUpdateDefaultValue(&param)

	if param.Type != BooleanTYpe {
		pr, _ := parseParameterRestrict(fields[ValueRestrictField], param.Type)
		param.ParameterRestrict = pr
		return &param
	}

	for _, vt := range types {
		pr, err := parseParameterRestrict(fields[ValueRestrictField], vt)
		if err != nil {
			continue
		}
		if vt == IntegerType && booleanPromotion && pr.isEnum {
			fields[ValueRestrictField] += ", OFF, ON"
			continue
		}
		param.Type = vt
		param.ParameterRestrict = pr
		break
	}
	return &param
}

func checkAndUpdateDefaultValue(param *ParameterType) {
	var (
		defaultValue = param.DefaultValue
		valueType    = param.Type
	)

	formatString := func(v interface{}) string {
		return fmt.Sprintf("%v", v)
	}

	if defaultValue == "" {
		return
	}
	if defaultValue[0] == '{' {
		param.DefaultValue = ""
	}

	switch valueType {
	case BooleanTYpe:
		checkAndUpdateBoolDefaultValue(param, formatString)
	case StringType:
		if ignoreStringDefault {
			param.DefaultValue = ""
		}
	case IntegerType, FloatType:
		if v, err := ValueTypeParserMap[param.Type](param.DefaultValue); err != nil || formatString(v) != defaultValue {
			param.DefaultValue = ""
		}
	}
}

func checkAndUpdateBoolDefaultValue(param *ParameterType, formatString func(v interface{}) string) {
	if booleanPromotion {
		return
	}

	v, err := ValueTypeParserMap[BooleanTYpe](param.DefaultValue)
	if err != nil {
		param.DefaultValue = ""
		return
	}
	param.DefaultValue = formatString(v)
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

func parseValue(s string, valueType ValueType) (interface{}, error) {
	v, err := ValueTypeParserMap[valueType](s)
	if err != nil {
		return nil, cfgcore.MakeError("parse type[%s] value[%s] failed!", valueType, s)
	}
	return v, nil
}

func parseParameterRestrict(s string, valueType ValueType) (*ParameterRestrict, error) {
	var (
		IntegerRangeRegex = regexp.MustCompile(`([\+\-]?\d+)-([\+\-]?\d+)`)
		FloatRangeRegex   = regexp.MustCompile(`([\+\-]?\d+(\.\d*)?)-([\+\-]?\d+(\.\d*)?)`)

		pr  *ParameterRestrict
		err error
	)

	setValueHelper := func(rv reflect.Value, s string, valueType ValueType) error {
		if rv.Kind() != reflect.Pointer || rv.IsNil() {
			return cfgcore.MakeError("invalid return type")
		}

		value, err := parseValue(s, valueType)
		if err != nil {
			return err
		}
		reflect.Indirect(rv).Set(reflect.Indirect(reflect.ValueOf(value)))
		return nil
	}
	integerTypeHandle := func(s string) (*ParameterRestrict, error) {
		r := IntegerRangeRegex.FindStringSubmatch(s)
		if len(r) == 0 {
			return nil, nil
		}
		t := &ParameterRestrict{isEnum: false}
		if err := setValueHelper(reflect.ValueOf(&t.Min), r[1], valueType); err != nil {
			return nil, err
		}
		if err := setValueHelper(reflect.ValueOf(&t.Max), r[2], valueType); err != nil {
			return nil, err
		}
		return t, nil
	}
	floatTypeHandle := func(s string) (*ParameterRestrict, error) {
		r := FloatRangeRegex.FindStringSubmatch(s)
		if len(r) == 0 {
			return nil, nil
		}
		t := &ParameterRestrict{isEnum: false}
		if err := setValueHelper(reflect.ValueOf(&t.Min), r[1], valueType); err != nil {
			return nil, err
		}
		if err := setValueHelper(reflect.ValueOf(&t.Max), r[3], valueType); err != nil {
			return nil, err
		}
		return t, nil
	}

	if s == "" {
		return nil, nil
	}

	switch valueType {
	case IntegerType:
		pr, err = integerTypeHandle(s)
	case FloatType:
		pr, err = floatTypeHandle(s)
	}

	if err != nil {
		return nil, err
	}
	if pr != nil {
		return pr, nil
	}

	return parseListParameter(s, valueType)
}

func parseListParameter(s string, valueType ValueType) (*ParameterRestrict, error) {
	values := strings.Split(s, ",")
	if len(values) == 0 {
		return nil, nil
	}

	p := &ParameterRestrict{isEnum: true}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if len(v) == 0 {
			continue
		}
		if typeValue, err := ValueTypeParserMap[valueType](v); err != nil {
			return nil, cfgcore.WrapError(err, "parse failed: [%s] [%s] [%s]\n", s, v, valueType)
		} else {
			AddRestrictValue(p, typeValue)
		}
	}
	if len(p.EnumList) == 0 {
		return nil, nil
	}
	return p, nil
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
	buffer.WriteString(prefixString + "  ")
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
	switch w.Type {
	case IntegerType:
		buffer.WriteString("int")
	case BooleanTYpe:
		buffer.WriteString("bool")
	default:
		buffer.WriteString(string(w.Type))
	}
}

func (w *CueWrapper) generateCueDocument(buffer *bytes.Buffer) {
	if w.Document != "" {
		buffer.WriteString(prefixString + "  ")
		buffer.WriteString("// ")
		buffer.WriteString(w.Document)
		buffer.WriteByte('\n')
	}
}
