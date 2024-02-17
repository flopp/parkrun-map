package utils

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
)

func WriteFile(filePath string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0770); err != nil {
		return err
	}

	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	if strings.HasSuffix(filePath, ".gz") {
		zipWriter, err := gzip.NewWriterLevel(out, 9)
		if err != nil {
			return err
		}
		defer zipWriter.Close()

		_, err = zipWriter.Write(data)
		return err
	} else {
		_, err := out.Write(data)
		return err
	}
}
