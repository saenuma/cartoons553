package main

import (
  "os"
  "path/filepath"
  "strings"
  "github.com/pkg/errors"
  "math/rand"
  "io"
  "time"
  "net/http"
  "fmt"
)


func GetRootPath() (string, error) {
	hd, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "os error")
	}
	dd := os.Getenv("SNAP_USER_COMMON")
	if strings.HasPrefix(dd, filepath.Join(hd, "snap", "go")) || dd == "" {
		dd = filepath.Join(hd, "ooldim_data")
    os.MkdirAll(dd, 0777)
	}

	return dd, nil
}


func UntestedRandomString(length int) string {
  var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
  const charset = "abcdefghijklmnopqrstuvwxyz1234567890"

  b := make([]byte, length)
  for i := range b {
    b[i] = charset[seededRand.Intn(len(charset))]
  }
  return string(b)
}


func DoesPathExists(p string) bool {
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return false
	}
	return true
}


func downloadFile(url, outPath string) error {
	if DoesPathExists(outPath) {
		return nil
	}

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return errors.Wrap(err, "http error")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
  if err != nil {
    return errors.Wrap(err, "io error")
  }

	if resp.StatusCode != 200 {
		return errors.New(string(body))
	}

	out, err := os.Create(outPath)
	if err != nil {
		return errors.Wrap(err, "os error")
	}
	defer out.Close()

	// Write the body to file
	_, err = out.Write(body)
	if err != nil {
		return errors.Wrap(err, "io error")
	}

	fmt.Println("Downloaded: " + filepath.Base(url))
	return nil
}