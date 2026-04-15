/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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

package parameters

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	parameterscore "github.com/apecloud/kubeblocks/pkg/parameters/core"
)

var (
	yamlMarkerLinePattern = regexp.MustCompile(`^\s*['"]?([A-Za-z0-9_.-]+)['"]?\s*:\s*.*$`)
	jsonMarkerLinePattern = regexp.MustCompile(`^\s*"([^"]+)"\s*:\s*.*$`)
)

type markerLineEntry struct {
	Marker parametersv1alpha1.ParameterViewContentMarker
	Text   string
}

func (r *ParameterViewReconciler) renderMarkerLineContent(cfgCtx *parameterViewConfigContext,
	view *parametersv1alpha1.ParameterView, rawContent string) (string, error) {
	entries, trailingNewline, err := r.buildMarkerLineEntries(cfgCtx, view, rawContent)
	if err != nil {
		return "", err
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Text == "" {
			lines = append(lines, fmt.Sprintf("[%s]", entry.Marker))
			continue
		}
		lines = append(lines, fmt.Sprintf("[%s] %s", entry.Marker, entry.Text))
	}
	result := strings.Join(lines, "\n")
	if trailingNewline && len(entries) > 0 {
		result += "\n"
	}
	return result, nil
}

func (r *ParameterViewReconciler) buildMarkerLineEntries(cfgCtx *parameterViewConfigContext,
	view *parametersv1alpha1.ParameterView, rawContent string) ([]markerLineEntry, bool, error) {
	lines, trailingNewline := splitContentLines(rawContent)
	if len(lines) == 0 {
		return nil, trailingNewline, nil
	}
	paramsDef := resolveParameterDefinition(cfgCtx.paramsDefs, view.Spec.FileName)
	paramValues, err := parameterscore.TransformConfigFileToKeyValueMap(view.Spec.FileName,
		[]parametersv1alpha1.ComponentConfigDescription{cfgCtx.fileConfig}, []byte(rawContent))
	if err != nil {
		return nil, false, err
	}
	resolvedNames := make(map[string]string, len(paramValues)*2)
	for fullName := range paramValues {
		resolvedNames[fullName] = fullName
		shortName := shortParameterName(fullName)
		if shortName == fullName {
			continue
		}
		if existing, ok := resolvedNames[shortName]; !ok || existing == fullName {
			resolvedNames[shortName] = fullName
		}
	}

	entries := make([]markerLineEntry, 0, len(lines))
	for _, line := range lines {
		paramName, ok := extractMarkerLineParameterName(cfgCtx.fileConfig.FileFormatConfig.Format, line)
		if !ok {
			entries = append(entries, markerLineEntry{
				Marker: parametersv1alpha1.UnmanagedParameterViewContentMarker,
				Text:   line,
			})
			continue
		}
		fullName := paramName
		if resolved, ok := resolvedNames[paramName]; ok {
			fullName = resolved
		}
		entries = append(entries, markerLineEntry{
			Marker: classifyMarkerLine(fullName, paramsDef),
			Text:   line,
		})
	}
	return entries, trailingNewline, nil
}

func parseMarkerLineContent(content string) ([]markerLineEntry, bool, string, error) {
	lines, trailingNewline := splitContentLines(content)
	if len(lines) == 0 {
		return nil, trailingNewline, "", nil
	}
	entries := make([]markerLineEntry, 0, len(lines))
	rawLines := make([]string, 0, len(lines))
	for idx, line := range lines {
		entry, err := parseMarkerLine(line)
		if err != nil {
			return nil, false, "", fmt.Errorf("invalid marker syntax at line %d: %w", idx+1, err)
		}
		entries = append(entries, entry)
		rawLines = append(rawLines, entry.Text)
	}
	rawContent := strings.Join(rawLines, "\n")
	if trailingNewline {
		rawContent += "\n"
	}
	return entries, trailingNewline, rawContent, nil
}

func parseMarkerLine(line string) (markerLineEntry, error) {
	if len(line) < 3 || line[0] != '[' || line[2] != ']' {
		return markerLineEntry{}, fmt.Errorf("line must start with a marker like [D], [S], [I], or [U]")
	}
	marker := parametersv1alpha1.ParameterViewContentMarker(line[1:2])
	switch marker {
	case parametersv1alpha1.DynamicParameterViewContentMarker,
		parametersv1alpha1.StaticParameterViewContentMarker,
		parametersv1alpha1.ImmutableParameterViewContentMarker,
		parametersv1alpha1.UnmanagedParameterViewContentMarker:
	default:
		return markerLineEntry{}, fmt.Errorf("unsupported marker [%s]", marker)
	}
	text := ""
	switch {
	case len(line) == 3:
	case len(line) > 3 && line[3] == ' ':
		text = line[4:]
	default:
		return markerLineEntry{}, fmt.Errorf("marker must be followed by a space or line end")
	}
	return markerLineEntry{Marker: marker, Text: text}, nil
}

func validateMarkerLineContent(edited, source string) error {
	editedEntries, _, _, err := parseMarkerLineContent(edited)
	if err != nil {
		return err
	}
	sourceEntries, _, _, err := parseMarkerLineContent(source)
	if err != nil {
		return err
	}
	if len(editedEntries) != len(sourceEntries) {
		return fmt.Errorf("marker line count does not match source view")
	}
	for i := range sourceEntries {
		if editedEntries[i].Marker != sourceEntries[i].Marker {
			return fmt.Errorf("marker changed at line %d", i+1)
		}
		if sourceEntries[i].Marker == parametersv1alpha1.DynamicParameterViewContentMarker ||
			sourceEntries[i].Marker == parametersv1alpha1.StaticParameterViewContentMarker {
			continue
		}
		if editedEntries[i].Text != sourceEntries[i].Text {
			return fmt.Errorf("line %d with marker [%s] cannot be modified", i+1, sourceEntries[i].Marker)
		}
	}
	return nil
}

func splitContentLines(content string) ([]string, bool) {
	if content == "" {
		return nil, false
	}
	trailingNewline := strings.HasSuffix(content, "\n")
	trimmed := strings.TrimSuffix(content, "\n")
	return strings.Split(trimmed, "\n"), trailingNewline
}

func resolveParameterDefinition(paramsDefs []*parametersv1alpha1.ParametersDefinition, fileName string) *parametersv1alpha1.ParametersDefinition {
	for _, paramsDef := range paramsDefs {
		if paramsDef != nil && paramsDef.Spec.FileName == fileName {
			return paramsDef
		}
	}
	return nil
}

func classifyMarkerLine(paramName string,
	paramsDef *parametersv1alpha1.ParametersDefinition) parametersv1alpha1.ParameterViewContentMarker {
	if paramsDef == nil {
		return parametersv1alpha1.UnmanagedParameterViewContentMarker
	}
	spec := &paramsDef.Spec
	switch {
	case matchesParameterList(spec.ImmutableParameters, paramName):
		return parametersv1alpha1.ImmutableParameterViewContentMarker
	case matchesParameterList(spec.DynamicParameters, paramName):
		return parametersv1alpha1.DynamicParameterViewContentMarker
	case matchesParameterList(spec.StaticParameters, paramName):
		return parametersv1alpha1.StaticParameterViewContentMarker
	default:
		return parametersv1alpha1.UnmanagedParameterViewContentMarker
	}
}

func matchesParameterList(list []string, paramName string) bool {
	if len(list) == 0 {
		return false
	}
	shortName := shortParameterName(paramName)
	return slices.ContainsFunc(list, func(candidate string) bool {
		candidateShortName := shortParameterName(candidate)
		return candidate == paramName ||
			candidate == shortName ||
			candidateShortName == shortName ||
			strings.HasSuffix(paramName, "."+candidate) ||
			strings.HasSuffix(candidate, "."+shortName)
	})
}

func shortParameterName(paramName string) string {
	if idx := strings.LastIndex(paramName, "."); idx >= 0 && idx+1 < len(paramName) {
		return paramName[idx+1:]
	}
	return paramName
}

func extractMarkerLineParameterName(format parametersv1alpha1.CfgFileFormat, line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", false
	}
	switch format {
	case parametersv1alpha1.Ini, parametersv1alpha1.TOML:
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") || strings.HasPrefix(trimmed, "[") {
			return "", false
		}
		return splitConfigKey(line)
	case parametersv1alpha1.Properties, parametersv1alpha1.PropertiesPlus, parametersv1alpha1.PropertiesUltra,
		parametersv1alpha1.Dotenv, parametersv1alpha1.RedisCfg:
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			return "", false
		}
		return splitConfigKey(line)
	case parametersv1alpha1.YAML:
		matches := yamlMarkerLinePattern.FindStringSubmatch(line)
		if len(matches) != 2 {
			return "", false
		}
		return strings.Trim(matches[1], `"'`), true
	case parametersv1alpha1.JSON:
		matches := jsonMarkerLinePattern.FindStringSubmatch(line)
		if len(matches) != 2 {
			return "", false
		}
		return matches[1], true
	default:
		return "", false
	}
}

func splitConfigKey(line string) (string, bool) {
	idx := strings.IndexAny(line, "=:")
	if idx <= 0 {
		return "", false
	}
	key := strings.TrimSpace(line[:idx])
	if key == "" {
		return "", false
	}
	return key, true
}
