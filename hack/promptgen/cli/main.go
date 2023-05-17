package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/apecloud/kubeblocks/internal/cli/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"io"
	"log"
	"os"
	"path/filepath"
)

type option struct {
	Name      string `json:"name"`
	Alias     string `json:"alias,omitempty"`
	Doc       string `json:"doc"`
	IsBoolean bool   `json:"boolean,omitempty"`
}

type options []option

type command struct {
	Name     string   `json:"name"`
	Doc      string   `json:"doc"`
	Options  options  `json:"options,omitempty"`
	Commands commands `json:"commands,omitempty"`
}

type commands []command
type data struct {
	Cmds    []command `json:"cmds"`
	Options []option  `json:"options"`
}

func genAllCommandForData(cmd *cobra.Command, data *data) error {

	for _, c := range cmd.Commands() {
		var newcmd command
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		newcmd.Name = c.Name()

		switch {
		case c.Long != "":
			newcmd.Doc = c.Long
		case c.Short != "":
			newcmd.Doc = c.Short
		}
		c.Flags().VisitAll(func(flag *pflag.Flag) {
			var newoption option
			newoption.Name = "--" + flag.Name
			newoption.Doc = flag.Usage
			if len(flag.Shorthand) > 0 {
				newoption.Alias = "-" + flag.Shorthand
			}
			if flag.Value.Type() == "bool" {
				newoption.IsBoolean = true
			}
			newcmd.Options = append(newcmd.Options, newoption)
		})

		for _, sub := range c.Commands() {
			if !sub.IsAvailableCommand() || sub.IsAdditionalHelpTopicCommand() {
				continue
			}
			var newsub command
			newsub.Name = sub.Name()
			newsub.Doc = sub.Short
			sub.Flags().VisitAll(func(flag *pflag.Flag) {
				var newoption option
				newoption.Name = "--" + flag.Name
				newoption.Doc = flag.Usage
				if len(flag.Shorthand) > 0 {
					newoption.Alias = "-" + flag.Shorthand
				}
				if flag.Value.Type() == "bool" {
					newoption.IsBoolean = true
				}
				newsub.Options = append(newsub.Options, newoption)
			})
			newcmd.Commands = append(newcmd.Commands, newsub)
		}
		data.Cmds = append(data.Cmds, newcmd)
	}
	err := writeToLocal(*data)
	if err != nil {
		return err
	}
	return nil
}
func writeToLocal(data data) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}
	absPath, err := filepath.Abs(workingDir)
	if err != nil {
		return err
	}
	dir := filepath.Dir(absPath)
	filePath := filepath.Join(dir, "prompt_schema.json")
	jsonData, _ := json.MarshalIndent(data, "", "    ")
	file, _ := os.Create(filePath)
	defer file.Close()
	_, err = file.Write(jsonData)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	var writedata data
	var buff bytes.Buffer

	cli := cmd.NewCliCmd()
	cli.Long = fmt.Sprintf("```\n%s\n```", cli.Long)
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := cli.Execute()
	if err != nil {
		log.Fatal("can't execute kbcli", err)
		return
	}
	w.Close()
	os.Stdout = old
	io.Copy(&buff, r)
	buff.Reset()
	r.Close()
	w.Close()
	cli.Flags().VisitAll(func(flag *pflag.Flag) {
		var newoption option
		newoption.Name = "--" + flag.Name
		newoption.Doc = flag.Usage
		if len(flag.Shorthand) > 0 {
			newoption.Alias = "-" + flag.Shorthand
		}
		if flag.Value.Type() == "bool" {
			newoption.IsBoolean = true
		}
		writedata.Options = append(writedata.Options, newoption)
	})
	if cacheDirFlag := cli.Flag("cache-dir"); cacheDirFlag != nil {
		cacheDirFlag.DefValue = "$HOME/.kube/cache"
	}
	err = genAllCommandForData(cli, &writedata)
	if err != nil {
		log.Fatal("generate json for cli  failed", err)
	}

}
