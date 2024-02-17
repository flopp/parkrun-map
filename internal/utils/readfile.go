package utils

import (
	"compress/gzip"
	"io"
	"os"
	"strings"
)

func ReadFile(filePath string) ([]byte, error) {
	if strings.HasSuffix(filePath, ".gz") {
		f, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		zipReader, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer zipReader.Close()

		return io.ReadAll(zipReader)
	} else {
		buf, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
		return buf, nil
	}
}
