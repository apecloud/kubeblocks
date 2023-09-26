package monitor

import (
	"cuelang.org/go/cue"
	"embed"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	"github.com/leaanthony/debme"
	"gopkg.in/yaml.v3"
)

var (
	//go:embed cue/*
	cueTemplates embed.FS
	cacheCtx     = map[string]interface{}{}
)

func getCacheCUETplValue(key string, valueCreator func() (*intctrlutil.CUETpl, error)) (*intctrlutil.CUETpl, error) {
	vIf, ok := cacheCtx[key]
	if ok {
		return vIf.(*intctrlutil.CUETpl), nil
	}
	v, err := valueCreator()
	if err != nil {
		return nil, err
	}
	cacheCtx[key] = v
	return v, err
}

func buildFromCUEForOTel(tplName string, fillMap map[string]any, lookupKey string) ([]byte, error) {
	cueFS, _ := debme.FS(cueTemplates, "cue")
	cueTpl, err := getCacheCUETplValue(tplName, func() (*intctrlutil.CUETpl, error) {
		return intctrlutil.NewCUETplFromBytes(cueFS.ReadFile(tplName))
	})
	if err != nil {
		return nil, err
	}

	cueValue := intctrlutil.NewCUEBuilder(*cueTpl)

	for k, v := range fillMap {
		if err := cueValue.FillObj(k, v); err != nil {
			return nil, err
		}
	}

	value := cueValue.Value.LookupPath(cue.ParsePath(lookupKey))
	bytes, err := value.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var jsonObj interface{}
	if err := yaml.Unmarshal(bytes, &jsonObj); err != nil {
		return nil, err
	}
	yamlBytes, err := yaml.Marshal(jsonObj)
	if err != nil {
		return nil, err
	}
	return yamlBytes, nil
}
