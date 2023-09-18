/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"os"
	executil "os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"k8s.io/utils/exec"
)

const label = "app.kubernetes.io/instance"

func GetFiles(path string) ([]string, error) {
	var result []string
	e := filepath.Walk(path, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Println(err)
			return err
		}
		if !fi.IsDir() {
			if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
				result = append(result, path)
			}
		}
		return nil
	})
	if e != nil {
		log.Println(e)
		return result, e
	}
	return result, nil
}

func GetFolders(path string) ([]string, error) {
	var result []string
	e := filepath.Walk(path, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			log.Println(err)
			return err
		}
		if fi.IsDir() {
			result = append(result, path)
		}
		return nil
	})
	if e != nil {
		log.Println(e)
		return result, e
	}
	return result, nil
}

func CheckClusterStatus(name, ns string, status string) bool {
	cmd := "kubectl get cluster " + name + " -n " + ns + " | grep " + name + " | awk '{print $5}'"
	log.Println(cmd)
	clusterStatus := ExecCommand(cmd)
	log.Println("clusterStatus is " + clusterStatus)
	return strings.TrimSpace(clusterStatus) == status
}

func CheckPodStatus(name, ns string) map[string]bool {
	var podStatusResult = make(map[string]bool)
	cmd := "kubectl get pod -n " + ns + " -l '" + label + "=" + name + "'| grep " + name + " | awk '{print $1}'"
	log.Println(cmd)
	arr := ExecCommandReadline(cmd)
	log.Println(arr)
	if len(arr) > 0 {
		for _, podName := range arr {
			command := "kubectl get pod " + podName + " -n " + ns + " | grep " + podName + " | awk '{print $3}'"
			log.Println(command)
			podStatus := ExecCommand(command)
			log.Println("podStatus is " + podStatus)
			if strings.TrimSpace(podStatus) == "Running" {
				podStatusResult[podName] = true
			} else {
				podStatusResult[podName] = false
			}
		}
	}
	return podStatusResult
}

func OpsYaml(file string, ops string) bool {
	cmd := "kubectl " + ops + " -f " + file
	log.Println(cmd)
	b := ExecuteCommand(cmd)
	return b
}

func ExecuteCommand(command string) bool {
	exeFlag := true
	cmd := exec.New().Command("bash", "-c", command)
	// Create a fetch command output pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Error:can not obtain stdout pipe for command:%s\n", err)
		exeFlag = false
	}
	// Create a fetch command err output pipe
	stderr, err1 := cmd.StderrPipe()
	if err1 != nil {
		log.Printf("Error:can not obtain stderr pipe for command:%s\n", err1)
		exeFlag = false
	}
	// Run command
	if err := cmd.Start(); err != nil {
		log.Printf("Error:The command is err:%s\n", err)
		exeFlag = false
	}
	// Use buffered readers
	go asyncLog(stdout)
	go asyncLog(stderr)
	if err := cmd.Wait(); err != nil {
		log.Printf("wait:%s\n", err.Error())
		exeFlag = false
	}
	return exeFlag
}

func asyncLog(reader io.Reader) {
	cache := ""
	buf := make([]byte, 1024)
	for {
		num, err := reader.Read(buf)
		if err != nil && errors.Is(err, io.EOF) {
			return
		}
		if num > 0 {
			b := buf[:num]
			s := strings.Split(string(b), "\n")
			line := strings.Join(s[:len(s)-1], "\n")
			log.Printf("%s%s\n", cache, line)
			cache = s[len(s)-1]
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
}

func ExecCommandReadline(strCommand string) []string {
	var arr []string
	cmd := exec.New().Command("/bin/bash", "-c", strCommand)
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		log.Printf("Execute failed when Start:%s\n\n", err.Error())
	}
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if err != nil || io.EOF == err {
			break
		}
		arr = append(arr, line)
	}
	if err := cmd.Wait(); err != nil {
		log.Printf("wait:%s\n\n", err.Error())
	}
	return arr
}

func ExecCommand(strCommand string) string {
	cmd := exec.New().Command("/bin/bash", "-c", strCommand)
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		log.Printf("failed to Start:%s\n", err.Error())
		return ""
	}
	outBytes, _ := io.ReadAll(stdout)
	err := stdout.Close()
	if err != nil {
		return ""
	}
	if err := cmd.Wait(); err != nil {
		log.Printf("failed to Wait:%s\n", err.Error())
		return ""
	}
	return string(outBytes)
}

func WaitTime(num int) {
	wg := sync.WaitGroup{}
	wg.Add(num)
	for i := 0; i < num; i++ {
		go func(i int) {
			defer wg.Done()
		}(i)
	}
	wg.Wait()
}

func GetClusterCreateYaml(files []string) string {
	for _, file := range files {
		if strings.Contains(file, "00") {
			return file
		}
	}
	return ""
}

func GetClusterVersion(folder string) (result []string) {
	dbType := GetPrefix(folder, "/")
	WaitTime(3000000)
	cmd := "kubectl get ClusterVersion | grep " + dbType + " | awk '{print $1}'"
	log.Println("cmd: " + cmd)
	result = ExecCommandReadline(cmd)
	log.Println(result)
	return result
}

func GetPrefix(str string, sub string) (s string) {
	index := strings.LastIndex(str, sub)
	s = str[index+1:]
	return
}

func ReplaceClusterVersionRef(fileName string, clusterVersionRef string) {
	file, err := os.OpenFile(fileName, os.O_RDWR, 0666)
	if err != nil {
		log.Println("open file filed.", err)
		return
	}
	reader := bufio.NewReader(file)
	pos := int64(0)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				log.Println("file read ok!")
				break
			} else {
				log.Println("file read error ", err)
				return
			}
		}
		if strings.Contains(line, "app.kubernetes.io/version") {
			version := GetPrefix(clusterVersionRef, "-")
			bytes := []byte("    app.kubernetes.io/version: \"" + version + "\"\n")
			_, err := file.WriteAt(bytes, pos)
			if err != nil {
				log.Println("file open failed ", err)
			}
		}
		if strings.Contains(line, "clusterVersionRef") {
			bytes := []byte("  clusterVersionRef: " + clusterVersionRef + "\n")
			_, err := file.WriteAt(bytes, pos)
			if err != nil {
				log.Println("file open failed ", err)
			}
		}
		pos += int64(len(line))
	}
}

func StringStrip(str string) string {
	str = strings.ReplaceAll(str, " ", "")
	str = strings.ReplaceAll(str, "\n", "")
	return str
}

func CheckKbcliExists() error {
	_, err := exec.New().LookPath("kbcli")
	return err
}

func Check(command string, input string) (string, error) {
	cmd := executil.Command("bash", "-c", command)

	var output bytes.Buffer
	cmd.Stdout = &output

	inPipe, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}

	err = cmd.Start()
	if err != nil {
		return "", err
	}

	_, e := io.WriteString(inPipe, input)
	if e != nil {
		return "", e
	}

	err = cmd.Wait()
	if err != nil {
		return "", err
	}

	return output.String(), nil
}

func GetName(fileName string) (name, ns string) {
	file, err := os.Open(fileName)
	if err != nil {
		log.Println(err)
	}
	br := bufio.NewReader(file)
	for {
		line, err := br.ReadString('\n')
		if err == io.EOF {
			break
		}
		if strings.Contains(line, "cluster.yaml") {
			for {
				line, err := br.ReadString('\n')
				if err == io.EOF {
					break
				}
				if strings.Contains(line, "---") {
					break
				}
				if strings.Contains(line, "  name:") {
					name = StringSplit(line)
				}
				if strings.Contains(line, "  namespace:") {
					ns = StringSplit(line)
				} else {
					ns = "default"
				}
			}
			break
		}
	}

	return name, ns
}

func ReadLineLast(fileName string, name string) string {
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatalln(err)
	}
	scanner := bufio.NewScanner(file)
	var last string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, name) {
			last = line
		}
	}
	return last
}

func StringSplit(str string) string {
	s := strings.Split(str, ":")[1]
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

func ReadLine(fileName string, name string) string {
	file, _ := os.Open(fileName)
	fileScanner := bufio.NewScanner(file)
	for fileScanner.Scan() {
		line := fileScanner.Text()
		if strings.Contains(line, name) {
			return line
		}
	}
	return ""
}

func Count(filename, substring string) (int, error) {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return 0, err
	}
	content := string(bytes)
	return countSubstring(content, substring), nil
}

func countSubstring(str, sub string) int {
	count := 0
	index := strings.Index(str, sub)
	for index >= 0 {
		count++
		index = strings.Index(str[index+1:], sub)
	}
	return count
}

func CheckCommand(command string, path string) bool {
	cmdPath := path + "/" + command
	if fi, err := os.Stat(cmdPath); err == nil && fi.Mode()&0111 != 0 {
		return true
	}
	if _, err := exec.New().LookPath(cmdPath); err == nil {
		return true
	}

	return false
}

func RemoveElements(arr []string, elemsToRemove []string) []string {
	var result []string
	m := make(map[string]bool)
	for _, e := range elemsToRemove {
		m[e] = true
	}
	for _, e := range arr {
		if !m[e] {
			result = append(result, e)
		}
	}
	return result
}
