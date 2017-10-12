/*
Download Action

Download a single file from Internet and unpack it in place if needed.

Yaml syntax:
 - action: download
   url: http://example.domain/path/filename.ext
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
*/
package actions

import (
	"fmt"
	"github.com/go-debos/debos"
	"net/url"
	"path"
)

type DownloadAction struct {
	debos.BaseAction `yaml:",inline"`
	Url              string // URL for downloading
	Filename         string // File name, overrides the name from URL.
	Unpack           bool   // Unpack downloaded file to directory dedicated for download
	Compression      string // compression type
	Name             string // exporting path to file or directory(in case of unpack)
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

func (d *DownloadAction) validateFilename(context *debos.DebosContext, url *url.URL) (filename string, err error) {
	if len(d.Filename) == 0 {
		// Trying to guess the name from URL Path
		filename = path.Base(url.Path)
	} else {
		filename = path.Base(d.Filename)
	}
	if len(filename) == 0 {
		return "", fmt.Errorf("Incorrect filename is provided for '%s'", d.Url)
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

func (d *DownloadAction) Verify(context *debos.DebosContext) error {
	var filename string

	if len(d.Name) == 0 {
		return fmt.Errorf("Property 'name' is mandatory for download action\n")
	}

	url, err := d.validateUrl()
	if err != nil {
		return err
	}
	filename, err = d.validateFilename(context, url)
	if err != nil {
		return err
	}
	if d.Unpack == true {
		if _, err := d.archive(filename); err != nil {
			return err
		}
	}
	return nil
}

func (d *DownloadAction) Run(context *debos.DebosContext) error {
	var filename string
	d.LogStart()

	url, err := d.validateUrl()
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
		err := debos.DownloadHttpUrl(url.String(), filename)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("Unsupported URL is provided: '%s'", url.String())
	}

	if d.Unpack == true {
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
