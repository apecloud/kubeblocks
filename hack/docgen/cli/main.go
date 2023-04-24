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

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"

	"github.com/apecloud/kubeblocks/internal/cli/cmd"
)

func genMarkdownTreeForOverview(cmd *cobra.Command, dir string) error {
	filename := filepath.Join(dir, "cli.md")
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err = io.WriteString(f, `---
title: KubeBlocks CLI Overview
description: KubeBlocks CLI overview
sidebar_position: 1
---

`); err != nil {
		return err
	}

	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}

		// write parent command name
		link := strings.ReplaceAll(cmd.Name()+" "+c.Name(), " ", "_")
		_, err = io.WriteString(f, fmt.Sprintf("## [%s](%s.md)\n\n", c.Name(), link))
		if err != nil {
			return err
		}

		// write command description
		switch {
		case c.Long != "":
			_, err = io.WriteString(f, fmt.Sprintf("%s\n\n", c.Long))
		case c.Short != "":
			_, err = io.WriteString(f, fmt.Sprintf("%s\n\n", c.Short))
		}
		if err != nil {
			return err
		}

		// write subcommands
		for _, sub := range c.Commands() {
			if !sub.IsAvailableCommand() || sub.IsAdditionalHelpTopicCommand() {
				continue
			}
			subName := cmd.Name() + " " + c.Name() + " " + sub.Name()
			link = strings.ReplaceAll(subName, " ", "_")
			_, err = io.WriteString(f, fmt.Sprintf("* [%s](%s.md)\t - %s\n", subName, link, sub.Short))
			if err != nil {
				return err
			}
		}
		_, err = io.WriteString(f, "\n\n")
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	rootPath := "./docs/user_docs/cli"
	if len(os.Args) > 1 {
		rootPath = os.Args[1]
	}

	fmt.Println("Scanning CLI docs rootPath: ", rootPath)
	cli := cmd.NewCliCmd()
	cli.Long = fmt.Sprintf("```\n%s\n```", cli.Long)

	if cacheDirFlag := cli.Flag("cache-dir"); cacheDirFlag != nil {
		cacheDirFlag.DefValue = "$HOME/.kube/cache"
	}

	err := doc.GenMarkdownTree(cli, rootPath)
	if err != nil {
		log.Fatal(err)
	}

	err = genMarkdownTreeForOverview(cli, rootPath)
	if err != nil {
		log.Fatal("generate docs for cli overview failed", err)
	}

	err = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := strings.Split(string(data), "\n")
		if len(lines) == 0 {
			return nil
		}

		firstLine := lines[0]
		if !strings.HasPrefix(firstLine, "## kbcli") {
			return nil
		}

		var lastIdx int
		for idx := len(lines) - 1; idx >= 0; idx-- {
			if strings.Contains(lines[idx], "Auto generated") {
				lastIdx = idx
				break
			}
		}
		if lastIdx == 0 {
			return nil
		}
		lines[lastIdx] = "#### Go Back to [CLI Overview](cli.md) Homepage.\n"

		// update the title
		lines[0] = "---"
		title := strings.TrimPrefix(firstLine, "## ")
		newLines := []string{"---", "title: " + title}
		for idx, line := range lines {
			if strings.Contains(line, "[kbcli](kbcli.md)") {
				lines[idx] = ""
				continue
			}
		}
		newLines = append(newLines, lines...)
		content := strings.Join(newLines, "\n")
		return os.WriteFile(path, []byte(content), info.Mode())
	})
	if err != nil {
		log.Fatal(err)
	}
}
