/*
Download Action

Download a single file from Internet and unpack it in place if needed.

	# Yaml syntax:
	- action: download
	  url: https://example.org/path/filename.ext
	  name: firmware
	  filename: output_name
	  unpack: bool
	  compression: gz

Mandatory properties:

- url -- URL to an object for download

- name -- string which allow to use downloaded object in other actions
via 'origin' property. If 'unpack' property is set to 'true' name will
refer to temporary directory with extracted content.

Optional properties:

- filename -- use this property as the name for saved file. Useful if URL does not
contain file name in path, for example it is possible to download files from URLs without path part.

- unpack -- hint for action to extract all files from downloaded archive.
See the 'Unpack' action for more information.

- compression -- optional hint for unpack allowing to use proper compression method.
See the 'Unpack' action for more information.

- sha256sum -- optional expected SHA256 sum of the downloaded file; provided directly as a 64 characters hexadecimal string
*/
package actions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path"

	"github.com/go-debos/debos"
)

type DownloadAction struct {
	debos.BaseAction `yaml:",inline"`
	URL              string `yaml:"url"` // URL for downloading
	Filename         string // File name, overrides the name from URL.
	Unpack           bool   // Unpack downloaded file to directory dedicated for download
	Compression      string // compression type
	Sha256sum        string // Expected SHA256 sum of the downloaded file
	Name             string // exporting path to file or directory(in case of unpack)
}

// validateURL checks if supported URL is passed from recipe
// Return:
// - parsed URL
// - nil in case of success
func (d *DownloadAction) validateURL() (*url.URL, error) {
	url, err := url.Parse(d.URL)
	if err != nil {
		return url, err
	}

	switch url.Scheme {
	case "http", "https":
		// Supported scheme
	default:
		return url, fmt.Errorf("unsupported URL provided: '%s'", url.String())
	}

	return url, nil
}

func (d *DownloadAction) validateFilename(context *debos.Context, url *url.URL) (filename string, err error) {
	if len(d.Filename) == 0 {
		// Trying to guess the name from URL Path
		filename = path.Base(url.Path)
	} else {
		filename = path.Base(d.Filename)
	}
	if len(filename) == 0 || filename == "." || filename == "/" {
		return "", fmt.Errorf("incorrect filename provided for '%s'", d.URL)
	}
	filename = path.Join(context.Scratchdir, filename)
	return filename, nil
}

func (d *DownloadAction) archive(filename string) (debos.Archive, error) {
	archive, err := debos.NewArchive(filename)
	if err != nil {
		return archive, err
	}
	switch archive.Type() {
	case debos.Tar:
		if len(d.Compression) > 0 {
			if err := archive.AddOption("tarcompression", d.Compression); err != nil {
				return archive, err
			}
		}
	default:
	}
	return archive, nil
}

func (d *DownloadAction) Verify(context *debos.Context) error {
	var filename string

	if len(d.Name) == 0 {
		return fmt.Errorf("property 'name' is mandatory for download action")
	}

	url, err := d.validateURL()
	if err != nil {
		return err
	}
	filename, err = d.validateFilename(context, url)
	if err != nil {
		return err
	}
	if d.Unpack {
		if _, err := d.archive(filename); err != nil {
			return err
		}
	}
	if len(d.Sha256sum) > 0 {
		if len(d.Sha256sum) != 64 {
			return fmt.Errorf("invalid length for property 'sha256sum'; expected 64 characters, got %d", len(d.Sha256sum))
		}
		_, err := hex.DecodeString(d.Sha256sum)
		if err != nil {
			return fmt.Errorf("invalid characters in 'sha256sum' property: %w", err)
		}
	}
	return nil
}

func (d *DownloadAction) Run(context *debos.Context) error {
	var filename string

	url, err := d.validateURL()
	if err != nil {
		return err
	}

	filename, err = d.validateFilename(context, url)
	if err != nil {
		return err
	}
	originPath := filename

	switch url.Scheme {
	case "http", "https":
		err := debos.DownloadHTTPURL(url.String(), filename)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported URL provided: '%s'", url.String())
	}

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open downloaded file %s: %w", filename, err)
	}
	defer file.Close()
	hasher := sha256.New()
	_, err = io.Copy(hasher, file)
	if err != nil {
		return fmt.Errorf("failed to hash file %s: %w", filename, err)
	}

	actualSha256sum := hex.EncodeToString(hasher.Sum(nil))
	log.Printf("Downloaded file '%s': sha256sum = %s", filename, actualSha256sum)

	if len(d.Sha256sum) > 0 {
		if actualSha256sum != d.Sha256sum {
			os.Remove(filename)
			return fmt.Errorf("SHA256 sum mismatch for %s. Expected %s but got %s", filename, d.Sha256sum, actualSha256sum)
		}
	}

	if d.Unpack {
		archive, err := d.archive(filename)
		if err != nil {
			return err
		}

		targetdir := filename + ".d"
		err = archive.RelaxedUnpack(targetdir)
		if err != nil {
			return err
		}
		originPath = targetdir
	}

	context.Origins[d.Name] = originPath

	return nil
}
