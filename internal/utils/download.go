package utils

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var downloadDelay = 0 * time.Second

func SetDownloadDelay(t time.Duration) {
	downloadDelay = t
}

func AlwaysDownload(url string, filePath string) error {
	fmt.Printf("-- downloading %s to %s\n", url, filePath)

	time.Sleep(downloadDelay)

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.0.0 Safari/537.36")
	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	statusOK := response.StatusCode >= 200 && response.StatusCode < 300
	if !statusOK {
		return fmt.Errorf("Non-OK HTTP status: %d", response.StatusCode)
	}

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, response.Body); err != nil {
		return err
	}

	return WriteFile(filePath, buf.Bytes())
}

func MustDownloadFile(url string, filePath string) {
	if err := AlwaysDownload(url, filePath); err != nil {
		panic(fmt.Errorf("while downloading '%s' to '%s': %v", url, filePath, err))
	}
}

func DownloadFileIfOlder(url string, filePath string, maxAge time.Time) error {
	if mtime, err := GetMtime(filePath); err == nil && mtime.After(maxAge) {
		return nil
	}

	return AlwaysDownload(url, filePath)
}

func MustDownloadFileIfOlder(url string, filePath string, maxAge time.Time) {
	if err := DownloadFileIfOlder(url, filePath, maxAge); err != nil {
		panic(fmt.Errorf("while downloading '%s' to '%s': %v", url, filePath, err))
	}
}

func DownloadFileIfNotExists(url string, filePath string) error {
	if _, err := GetMtime(filePath); err == nil {
		return nil
	}

	return AlwaysDownload(url, filePath)
}

func DownloadHash(url string, dst, dstDir string) (string, error) {
	if strings.Contains(dst, "HASH") {
		tmpfile, err := os.CreateTemp("", "")
		if err != nil {
			return "", err
		}
		defer os.Remove(tmpfile.Name())

		err = AlwaysDownload(url, tmpfile.Name())
		if err != nil {
			return "", err
		}

		return CopyHash(tmpfile.Name(), dst, dstDir)
	} else {
		dst2 := filepath.Join(dstDir, dst)

		err := AlwaysDownload(url, dst2)
		if err != nil {
			return "", err
		}

		return dst, nil
	}
}

func MustDownloadHash(url string, dst, dstDir string) string {
	res, err := DownloadHash(url, dst, dstDir)
	if err != nil {
		panic(err)
	}
	return res
}
