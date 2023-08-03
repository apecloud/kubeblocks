package util

import (
	"log"
	"os"
	"path/filepath"
)

func AnalyzeReport() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Println(err)
		return
	}
	parentDir := filepath.Dir(cwd)
	parentPath := filepath.Join(parentDir, "/report.json")
	log.Println("parentPath: " + parentPath)
}
