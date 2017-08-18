package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
)

type DownloadAction struct {
	BaseAction  `yaml:",inline"`
	Url         string // URL for downloading
	Filename    string // File name, overrides the name from URL.
	Unpack      bool   // Unpack downloaded file to directory dedicated for download
	Compression string // compression type
}

// Function for downloading single file object with http(s) protocol
func DownloadHttpUrl(url, filename string) error {
	log.Printf("Download started: '%s' -> '%s'\n", url, filename)

	// TODO: Proxy support?

	// Check if file object already exists.
	fi, err := os.Stat(filename)
	if !os.IsNotExist(err) && !fi.Mode().IsRegular() {
		return fmt.Errorf("Failed to download '%s': '%s' exists and it is not a regular file\n", url, filename)
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Url '%s' returned status code %d (%s)\n", url, resp.StatusCode, http.StatusText(resp.StatusCode))
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

// validateUrl checks if supported URL is passed from recipe
// Return:
// - parsed URL
// - nil in case of success
func (d *DownloadAction) validateUrl() (*url.URL, error) {

	url, err := url.Parse(d.Url)
	if err != nil {
		return url, err
	}

	switch url.Scheme {
	case "http", "https":
		// Supported scheme
	default:
		return url, fmt.Errorf("Unsupported URL is provided: '%s'", url.String())
	}

	return url, nil
}

func (d *DownloadAction) Verify(context *DebosContext) error {

	_, err := d.validateUrl()
	return err
}

func (d *DownloadAction) Run(context *DebosContext) error {
	var filename string
	d.LogStart()

	url, err := d.validateUrl()
	if err != nil {
		return err
	}

	if len(d.Filename) == 0 {
		// Trying to guess the name from URL Path
		filename = path.Base(url.Path)
	} else {
		filename = path.Base(d.Filename)
	}
	if len(filename) == 0 {
		return fmt.Errorf("Incorrect filename is provided for '%s'", d.Url)
	}
	filename = path.Join(context.scratchdir, filename)

	switch url.Scheme {
	case "http", "https":
		err := DownloadHttpUrl(url.String(), filename)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("Unsupported URL is provided: '%s'", url.String())
	}

	if d.Unpack == true {
		targetdir := filename + ".d"
		return UnpackTarArchive(filename, targetdir, d.Compression, "--no-same-owner", "--no-same-permissions")
	}

	return nil
}
