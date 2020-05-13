/*
Simple file utilities.
*/
package utilities

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Copy the src file to dst. Any existing file will be overwritten.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	if err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// Download a file via http(s) and store it locally.
func DownloadFile(source, destination string) error {
	resp, err := http.Get(source)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New(fmt.Sprintf("cannot get %s, error: %d", source, resp.StatusCode))
	}

	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// Helper to create a file and write the given text to it. If the file exists, it will be overwritten.
func StringToFile(s string, file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	_, err = fmt.Fprintln(w, s)
	if err != nil {
		_ = f.Close()
		return err
	}
	err = w.Flush()
	if err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// FileDoesNotExists returns 'true' if we can safely assume that the given file does not exist.
func FileDoesNotExists(file string) bool {
	_, err := os.Stat(file)
	return os.IsNotExist(err)
}
