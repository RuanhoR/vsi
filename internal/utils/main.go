package utils

import "os"

func FileExists(dir string) (bool) {
	_, err := os.Stat(dir)
	if err == nil {
		return true
	}
	return false
}
