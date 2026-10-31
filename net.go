package debos

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// DownloadHTTPURL downloads a single file object with the http(s) protocol.
func DownloadHTTPURL(url, filename string) (err error) {
	log.Printf("Download started: '%s' -> '%s'\n", url, filename)

	// TODO: Proxy support?

	// Check if file object already exists.
	fi, err := os.Stat(filename)
	if err == nil {
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("failed to download '%s': '%s' exists and it is not a regular file", url, filename)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat '%s': %w", filename, err)
	}

	//nolint:gosec // Downloads the URL given in the recipe.
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get '%s': %w", url, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to close response body: %w", closeErr))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("url '%s' returned status code %d (%s)", url, resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	// Output file
	output, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", filename, err)
	}
	defer func() {
		if closeErr := output.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to close %s: %w", filename, closeErr))
		}
	}()

	if _, err := io.Copy(output, resp.Body); err != nil {
		return fmt.Errorf("failed to write %s: %w", filename, err)
	}

	return nil
}
