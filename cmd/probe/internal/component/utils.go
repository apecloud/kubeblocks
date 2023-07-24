package component

import (
	"os"

	"github.com/spf13/viper"
)

type Properties map[string]string

type Component struct {
	Name string
	Spec ComponentSpec
}

type ComponentSpec struct {
	Version  string
	Metadata []kv
}

type kv struct {
	Name  string
	Value string
}

var Name2Property = map[string]Properties{}

func readConfig(filename string) (string, Properties, error) {
	viper.SetConfigType("yaml")
	viper.SetConfigFile(filename)
	if err := viper.ReadInConfig(); err != nil {
		return "", nil, err
	}
	component := &Component{}
	if err := viper.Unmarshal(component); err != nil {
		return "", nil, err
	}
	properties := make(Properties)
	properties["version"] = component.Spec.Version
	for _, pair := range component.Spec.Metadata {
		properties[pair.Name] = pair.Value
	}
	return component.Name, properties, nil
}

func GetAllComponent(dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, file := range files {
		name, properties, err := readConfig(dir + "/" + file.Name())
		if err != nil {
			return err
		}
		Name2Property[name] = properties
	}
	return nil
}

func GetProperties(name string) Properties {
	return Name2Property[name]
}
