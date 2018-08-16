package debos

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// Function for downloading single file object with http(s) protocol
func DownloadHttpUrl(downloadUrl, filename string) error {
	log.Printf("Download started: '%s' -> '%s'\n", downloadUrl, filename)

	// TODO: Proxy support?

	// Check if file object already exists.
	fi, err := os.Stat(filename)
	if !os.IsNotExist(err) && !fi.Mode().IsRegular() {
		return fmt.Errorf("Failed to download '%s': '%s' exists and it is not a regular file\n", downloadUrl, filename)
	}

	resp, err := http.Get(downloadUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Url '%s' returned status code %d (%s)\n", downloadUrl, resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	// Output file
	output, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer output.Close()

	if _, err := io.Copy(output, resp.Body); err != nil {
		return err
	}

	return nil
}
