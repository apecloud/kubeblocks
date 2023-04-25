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

package configmanager

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"text/template/parse"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
	cfgutil "github.com/apecloud/kubeblocks/internal/configuration/util"
	"github.com/apecloud/kubeblocks/internal/gotemplate"
)

type regexFilter = func(fileName string) bool

const (
	builtInExecFunctionName           = "exec"
	builtInUpdateVariableFunctionName = "execSql"
	builtInParamsPatchFunctionName    = "patchParams"

	buildInFilesObjectName = "Files"
)

type DynamicUpdater = func(ctx context.Context, updatedParams map[string]string) error

func OnlineUpdateParamsHandle(tplScriptPath string, formatConfig *appsv1alpha1.FormatterConfig, dataType, dsn string) (DynamicUpdater, error) {
	tplContent, err := os.ReadFile(tplScriptPath)
	if err != nil {
		return nil, err
	}
	if err := checkTPLScript(tplScriptPath, string(tplContent)); err != nil {
		return nil, err
	}
	return func(ctx context.Context, updatedParams map[string]string) error {
		return wrapGoTemplateRun(ctx, tplScriptPath, string(tplContent), updatedParams, formatConfig, dataType, dsn)
	}, nil
}

func checkTPLScript(tplName string, tplContent string) error {
	tr := parse.New(tplName)
	tr.Mode = parse.SkipFuncCheck
	_, err := tr.Parse(tplContent, "", "", make(map[string]*parse.Tree))
	return err
}

func wrapGoTemplateRun(ctx context.Context, tplScriptPath string, tplContent string, updatedParams map[string]string, formatConfig *appsv1alpha1.FormatterConfig, dataType string, dsn string) error {
	var (
		err            error
		commandChannel DynamicParamUpdater
	)

	if commandChannel, err = NewCommandChannel(ctx, dataType, dsn); err != nil {
		return err
	}
	defer commandChannel.Close()

	logger.Info(fmt.Sprintf("update global dynamic params: %v", updatedParams))
	values := gotemplate.ConstructFunctionArgList(updatedParams)
	values[buildInFilesObjectName] = newFiles(filepath.Dir(tplScriptPath))
	engine := gotemplate.NewTplEngine(&values, constructReloadBuiltinFuncs(ctx, commandChannel, formatConfig), tplScriptPath, nil, nil)
	_, err = engine.Render(tplContent)
	return err
}

func constructReloadBuiltinFuncs(ctx context.Context, cc DynamicParamUpdater, formatConfig *appsv1alpha1.FormatterConfig) *gotemplate.BuiltInObjectsFunc {
	return &gotemplate.BuiltInObjectsFunc{
		builtInExecFunctionName: func(command string, args ...string) (string, error) {
			execCommand := exec.CommandContext(ctx, command, args...)
			stdout, err := cfgutil.ExecShellCommand(execCommand)
			logger.V(1).Info(fmt.Sprintf("command: [%s], output: %s, err: %v", execCommand.String(), stdout, err))
			return stdout, err
		},
		builtInUpdateVariableFunctionName: func(sql string, args ...string) error {
			r, err := cc.ExecCommand(ctx, sql, args...)
			logger.V(1).Info(fmt.Sprintf("sql: [%s], result: [%v]", sql, r))
			return err
		},
		builtInParamsPatchFunctionName: func(updatedParams map[string]string, basefile, newfile string) error {
			logger.V(1).Info(fmt.Sprintf("update params: %v, basefile: %s, newfile: %s", updatedParams, basefile, newfile))
			if len(updatedParams) == 0 {
				if basefile == newfile {
					return nil
				}
				return copyFileContents(basefile, newfile)
			}
			b, err := os.ReadFile(basefile)
			if err != nil {
				return err
			}
			newConfig, err := cfgcore.ApplyConfigPatch(b, updatedParams, formatConfig)
			if err != nil {
				return err
			}
			return os.WriteFile(newfile, []byte(newConfig), os.ModePerm)
		},
	}
}

func createUpdatedParamsPatch(newVersion []string, oldVersion []string, formatCfg *appsv1alpha1.FormatterConfig) (map[string]string, error) {
	patchOption := cfgcore.CfgOption{
		Type:    cfgcore.CfgTplType,
		CfgType: formatCfg.Format,
		Log:     logger,
	}

	logger.V(1).Info(fmt.Sprintf("new version files: %v, old version files: %v", newVersion, oldVersion))
	oldData, err := cfgutil.FromConfigFiles(oldVersion)
	if err != nil {
		return nil, err
	}
	newData, err := cfgutil.FromConfigFiles(newVersion)
	if err != nil {
		return nil, err
	}
	patch, err := cfgcore.CreateMergePatch(&cfgcore.ConfigResource{ConfigData: oldData}, &cfgcore.ConfigResource{ConfigData: newData}, patchOption)
	if err != nil {
		return nil, err
	}

	params := cfgcore.GenerateVisualizedParamsList(patch, formatCfg, nil)
	r := make(map[string]string)
	for _, key := range params {
		if key.UpdateType != cfgcore.DeletedType {
			for _, p := range key.Parameters {
				if p.Value != "" {
					r[p.Key] = p.Value
				}
			}
		}
	}
	return r, nil
}

func resolveLink(path string) (string, error) {
	logger.V(1).Info(fmt.Sprintf("resolveLink : %s", path))

	realPath, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(realPath) {
		realPath = filepath.Join(filepath.Dir(path), realPath)
	}
	logger.V(1).Info(fmt.Sprintf("real path: %s", realPath))
	fileInfo, err := os.Stat(realPath)
	if err != nil {
		return "", err
	}
	if fileInfo.IsDir() {
		return "", nil
	}
	if fileInfo.Mode().Type()&fs.ModeSymlink == fs.ModeSymlink {
		return resolveLink(realPath)
	}
	return realPath, nil
}

func readlink(dir string, entry fs.DirEntry, filter regexFilter) (string, error) {
	if !filter(entry.Name()) {
		logger.Info(fmt.Sprintf("ignore file: %s", entry.Name()))
		return "", nil
	}
	fullPath := filepath.Join(dir, entry.Name())
	if entry.Type().IsDir() {
		return "", nil
	}
	if entry.Type() != fs.ModeSymlink {
		return fullPath, nil
	}
	realPath, err := resolveLink(fullPath)
	if err != nil {
		return "", err
	}
	return realPath, nil
}

func createFileRegex(fileRegex string) (regexFilter, error) {
	if fileRegex == "" {
		return func(_ string) bool { return true }, nil
	}

	regxPattern, err := regexp.Compile(fileRegex)
	if err != nil {
		return nil, cfgcore.WrapError(err, "failed to create regexp [%s]", fileRegex)
	}
	return func(s string) bool {
		return regxPattern.MatchString(s)
	}, nil
}

func scanConfigFiles(dirs []string, filter regexFilter) ([]string, error) {
	configs := make([]string, 0)
	for _, dir := range dirs {
		files, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		logger.V(1).Info(fmt.Sprintf("scan watch directory: %s", dir))
		for _, f := range files {
			logger.V(1).Info(fmt.Sprintf("scan file: %s", f.Name()))
			if realPath, err := readlink(dir, f, filter); err == nil && realPath != "" {
				logger.Info(fmt.Sprintf("find valid config file: %s", realPath))
				configs = append(configs, realPath)
			}
		}
	}
	return configs, nil
}

func ScanConfigVolume(mountPoint string) ([]string, error) {
	filter, _ := createFileRegex("")
	return scanConfigFiles([]string{mountPoint}, filter)
}

func backupConfigFiles(dirs []string, filter regexFilter, backupPath string) error {
	fileInfo, err := os.Stat(backupPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if fileInfo == nil {
		if err := os.MkdirAll(backupPath, fs.ModePerm); err != nil {
			return err
		}
	}
	configs, err := scanConfigFiles(dirs, filter)
	if err != nil {
		return err
	}
	return backupLastConfigFiles(configs, backupPath)
}

func backupLastConfigFiles(configs []string, backupPath string) error {
	for _, file := range configs {
		logger.Info(fmt.Sprintf("backup config file: %s", file))
		if err := copyFileContents(file, filepath.Join(backupPath, filepath.Base(file))); err != nil {
			return err
		}
	}
	return nil
}

func copyFileContents(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
