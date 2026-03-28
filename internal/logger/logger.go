package logger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/RuanhoR/vsi/internal/utils"
)

var LoggerFile = ""

func checkLoggerFile() (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, errors.New("Can't load home")
	}
	if LoggerFile == "" {
		LoggerFile = filepath.Join(home, ".cache", "vsi.log")
	}
	if !utils.FileExists(LoggerFile) {
		file, err := os.Create(LoggerFile)
		if err != nil {
			return false, errors.New("Can't create log file")
		}
		file.Close()
	}
	return true, nil
}
func writeLog(message string) (bool, error) {
	checkLoggerFile()
	file, err := os.OpenFile(LoggerFile, os.O_APPEND, 0664)
	if err != nil {
		return false, errors.New("Can't Open log file")
	}
	if _, err := file.WriteString(message); err != nil {
		return false, errors.New("Can't append file")
	}
	defer file.Close()
	return true, err
}
func baseMsg(level string, tagName string, message string) string {
	return "[" + level + " " + time.Now().Format(time.RFC3339) + "]" + " [" + tagName + "] " + message
}
func E(tagName string, message string) {
	message = baseMsg("error", tagName, message)
	fmt.Println(message)
	go writeLog(message)
}
func W(tagName string, message string) {
	message = baseMsg("warning", tagName, message)
	go writeLog(message)
}
func I(tagName string, message string) {
	message = baseMsg("info", tagName, message)
	go writeLog(message)
}
