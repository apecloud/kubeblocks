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

package interactive

import (
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	ui "github.com/replicatedhq/termui/v3"
	"github.com/replicatedhq/termui/v3/widgets"
	"github.com/replicatedhq/troubleshoot/cmd/util"
	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
)

var (
	selectedResult = 0
	table          = widgets.NewTable()
	isShowingSaved = false
)

// ShowInteractiveResults displays the results with interactive mode
func ShowInteractiveResults(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult, outputPath string) error {
	if err := ui.Init(); err != nil {
		return errors.Wrap(err, "failed to create terminal ui")
	}
	defer ui.Close()

	drawUI(preflightName, analyzeResults)

	uiEvents := ui.PollEvents()
	for e := range uiEvents {
		switch e.ID {
		case "<C-c>":
			return nil
		case "q":
			if isShowingSaved {
				isShowingSaved = false
				ui.Clear()
				drawUI(preflightName, analyzeResults)
			} else {
				return nil
			}
		case "s":
			filename, err := save(preflightName, outputPath, analyzeResults)
			if err != nil {
				// show
			} else {
				showSaved(filename)
				go func() {
					time.Sleep(time.Second * 5)
					isShowingSaved = false
					ui.Clear()
					drawUI(preflightName, analyzeResults)
				}()
			}
		case "<Resize>":
			ui.Clear()
			drawUI(preflightName, analyzeResults)
		case "<Down>":
			if selectedResult < len(analyzeResults)-1 {
				selectedResult++
			} else {
				selectedResult = 0
				table.SelectedRow = 0
			}
			table.ScrollDown()
			ui.Clear()
			drawUI(preflightName, analyzeResults)
		case "<Up>":
			if selectedResult > 0 {
				selectedResult--
			} else {
				selectedResult = len(analyzeResults) - 1
				table.SelectedRow = len(analyzeResults)
			}
			table.ScrollUp()
			ui.Clear()
			drawUI(preflightName, analyzeResults)
		}
	}
	return nil
}

func drawUI(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) {
	drawGrid(analyzeResults)
	drawHeader(preflightName)
	drawFooter()
}

func drawHeader(preflightName string) {
	termWidth, _ := ui.TerminalDimensions()

	title := widgets.NewParagraph()
	title.Text = fmt.Sprintf("%s Preflight Checks", util.AppName(preflightName))
	title.TextStyle.Fg = ui.ColorWhite
	title.TextStyle.Bg = ui.ColorClear
	title.TextStyle.Modifier = ui.ModifierBold
	title.Border = false

	left := termWidth/2 - 2*len(title.Text)/3
	right := termWidth/2 + (termWidth/2 - left)

	title.SetRect(left, 0, right, 1)
	ui.Render(title)
}

func drawGrid(analyzeResults []*analyzerunner.AnalyzeResult) {
	drawPreflightTable(analyzeResults)
	drawDetails(analyzeResults[selectedResult])
}

func drawFooter() {
	termWidth, termHeight := ui.TerminalDimensions()

	instructions := widgets.NewParagraph()
	instructions.Text = "[q] quit    [s] save    [↑][↓] scroll"
	instructions.Border = false

	left := 0
	right := termWidth
	top := termHeight - 1
	bottom := termHeight

	instructions.SetRect(left, top, right, bottom)
	ui.Render(instructions)
}

func drawPreflightTable(analyzeResults []*analyzerunner.AnalyzeResult) {
	termWidth, termHeight := ui.TerminalDimensions()

	table.SetRect(0, 3, termWidth/2, termHeight-6)
	table.FillRow = true
	table.Border = true
	table.Rows = [][]string{}
	table.ColumnWidths = []int{termWidth}

	for i, analyzeResult := range analyzeResults {
		title := analyzeResult.Title
		if analyzeResult.Strict {
			title += fmt.Sprintf(" (Strict: %t)", analyzeResult.Strict)
		}
		switch {
		case analyzeResult.IsPass:
			title = fmt.Sprintf("✔  %s", title)
		case analyzeResult.IsWarn:
			title = fmt.Sprintf("⚠️  %s", title)
		case analyzeResult.IsFail:
			title = fmt.Sprintf("✘  %s", title)
		}
		table.Rows = append(table.Rows, []string{
			title,
		})
		switch {
		case analyzeResult.IsPass:
			if i == selectedResult {
				table.RowStyles[i] = ui.NewStyle(ui.ColorGreen, ui.ColorClear, ui.ModifierReverse)
			} else {
				table.RowStyles[i] = ui.NewStyle(ui.ColorGreen, ui.ColorClear)
			}
		case analyzeResult.IsWarn:
			if i == selectedResult {
				table.RowStyles[i] = ui.NewStyle(ui.ColorYellow, ui.ColorClear, ui.ModifierReverse)
			} else {
				table.RowStyles[i] = ui.NewStyle(ui.ColorYellow, ui.ColorClear)
			}
		case analyzeResult.IsFail:
			if i == selectedResult {
				table.RowStyles[i] = ui.NewStyle(ui.ColorRed, ui.ColorClear, ui.ModifierReverse)
			} else {
				table.RowStyles[i] = ui.NewStyle(ui.ColorRed, ui.ColorClear)
			}
		}
	}
	ui.Render(table)
}

func drawDetails(analysisResult *analyzerunner.AnalyzeResult) {
	termWidth, _ := ui.TerminalDimensions()

	currentTop := 4
	title := widgets.NewParagraph()
	title.Text = analysisResult.Title
	title.Border = false
	switch {
	case analysisResult.IsPass:
		title.TextStyle = ui.NewStyle(ui.ColorGreen, ui.ColorClear, ui.ModifierBold)
	case analysisResult.IsWarn:
		title.TextStyle = ui.NewStyle(ui.ColorYellow, ui.ColorClear, ui.ModifierBold)
	case analysisResult.IsFail:
		title.TextStyle = ui.NewStyle(ui.ColorRed, ui.ColorClear, ui.ModifierBold)
	}
	height := estimateNumberOfLines(title.Text, termWidth/2)
	title.SetRect(termWidth/2, currentTop, termWidth, currentTop+height)
	ui.Render(title)
	currentTop = currentTop + height + 1

	message := widgets.NewParagraph()
	message.Text = analysisResult.Message
	message.Border = false
	height = estimateNumberOfLines(message.Text, termWidth/2) + 2
	message.SetRect(termWidth/2, currentTop, termWidth, currentTop+height)
	ui.Render(message)
	currentTop = currentTop + height + 1

	if analysisResult.URI != "" {
		uri := widgets.NewParagraph()
		uri.Text = fmt.Sprintf("For more information: %s", analysisResult.URI)
		uri.Border = false
		height = estimateNumberOfLines(uri.Text, termWidth/2)
		uri.SetRect(termWidth/2, currentTop, termWidth, currentTop+height)
		ui.Render(uri)
		// currentTop = currentTop + height + 1
	}
}

func estimateNumberOfLines(text string, width int) int {
	if width == 0 {
		return 0
	}
	lines := len(text)/width + 1
	return lines
}

// save exports analyzeResults to local file against customize outputPath.
func save(preflightName string, outputPath string, analyzeResults []*analyzerunner.AnalyzeResult) (string, error) {
	filename := ""
	if outputPath != "" {
		// use override output path
		overridePath, err := convert.ValidateOutputPath(outputPath)
		if err != nil {
			return "", errors.Wrap(err, "override output file path")
		}
		filename = overridePath
	} else {
		// use default output path
		filename = fmt.Sprintf("%s-results-%s.txt", preflightName, time.Now().Format("2006-01-02T15_04_05"))
	}

	if _, err := os.Stat(filename); err == nil {
		_ = os.Remove(filename)
	}

	results := fmt.Sprintf("%s Preflight Checks\n\n", util.AppName(preflightName))
	for _, analyzeResult := range analyzeResults {
		result := ""
		switch {
		case analyzeResult.IsPass:
			result = "Check PASS\n"
		case analyzeResult.IsWarn:
			result = "Check WARN\n"
		case analyzeResult.IsFail:
			result = "Check FAIL\n"
		}
		result += fmt.Sprintf("Title: %s\n", analyzeResult.Title)
		result += fmt.Sprintf("Message: %s\n", analyzeResult.Message)

		if analyzeResult.URI != "" {
			result += fmt.Sprintf("URI: %s\n", analyzeResult.URI)
		}

		if analyzeResult.Strict {
			result += fmt.Sprintf("Strict: %t\n", analyzeResult.Strict)
		}
		result += "\n------------\n"
		results += result
	}

	if err := os.WriteFile(filename, []byte(results), 0644); err != nil {
		return "", errors.Wrap(err, "failed to save preflight results")
	}

	return filename, nil
}

func showSaved(filename string) {
	termWidth, termHeight := ui.TerminalDimensions()

	savedMessage := widgets.NewParagraph()
	savedMessage.Text = fmt.Sprintf("Preflight results saved to\n\n%s", filename)
	savedMessage.WrapText = true
	savedMessage.Border = true

	left := termWidth/2 - 20
	right := termWidth/2 + 20
	top := termHeight/2 - 4
	bottom := termHeight/2 + 4

	savedMessage.SetRect(left, top, right, bottom)
	ui.Render(savedMessage)

	isShowingSaved = true
}
