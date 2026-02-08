package debos

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// Function for downloading single file object with http(s) protocol
func DownloadHTTPURL(url, filename string) error {
	log.Printf("Download started: '%s' -> '%s'\n", url, filename)

	// TODO: Proxy support?

	// Check if file object already exists.
	fi, err := os.Stat(filename)
	if err == nil {
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("failed to download '%s': '%s' exists and it is not a regular file", url, filename)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", filename, err)
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("http get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("url '%s' returned status code %d (%s)", url, resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	// Output file
	output, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create %s: %w", filename, err)
	}
	defer output.Close()

	if _, err := io.Copy(output, resp.Body); err != nil {
		return fmt.Errorf("write %s: %w", filename, err)
	}

	return nil
}
