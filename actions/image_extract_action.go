/*
ImageExtract Action

Extract allows to pick out parts of the created image from the ImagePartition
and FilesystemDeploy actions, and save those parts as raw image files. In a way,
this is the Raw action in reverse.

	# Yaml syntax:
	- action: image-extract
	  parts:
	  	<list of parts>

Mandatory properties:
- parts -- list of parts of the image to extract.

Optional properties:

	# Yaml syntax for parts:
	  parts:
	    - partname: partition name
		  destination: filename

Mandatory properties:
- destination -- where to store the extracted part

- partname -- to extract a full partition, matching on partname is the easiest
*/
package actions

import (
	"errors"
	"fmt"
	"github.com/go-debos/debos"
	"io"
	"os"
	"path"
)

type ImagePart struct {
	PartName    string
	Destination string
}

type ImageExtractAction struct {
	debos.BaseAction `yaml:",inline"`
	Parts            []ImagePart
}

func (ext *ImageExtractAction) Verify(_ *debos.Context) error {
	for _, part := range ext.Parts {
		if len(part.PartName) == 0 {
			return fmt.Errorf("partname is a mandatory property")
		}

		if len(part.Destination) == 0 {
			return fmt.Errorf("destination is a mandatory property")
		}
	}

	// Can't verify partname existence at this point as ImagePartition metadata
	// isn't added to the Context until its Run part
	return nil
}

func (ext *ImageExtractAction) Run(context *debos.Context) error {
	if context.ImageFSTab.Len() == 0 {
		return errors.New("fstab not generated, missing image-partition action?")
	}

	origin := context.RecipeDir
	for _, part := range ext.Parts {
		// TODO: This lookup logic is copied from raw action, put in a shared
		// 		 helper util instead?
		var devicePath string
		for _, existingPart := range context.ImagePartitions {
			if existingPart.Name == part.PartName {
				devicePath = existingPart.DevicePath
				break
			}
		}

		if devicePath == "" {
			return fmt.Errorf("failed to find partition named %s", part.PartName)
		}

		// TODO: This copy logic is ALSO copied from raw action. Should do a
		// 		 generic copyFromTo(source, offset, dest, offset)
		source, err := os.Open(devicePath)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", devicePath, err)
		}
		defer source.Close()

		fi, err := source.Stat()
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", devicePath, err)
		}

		destPath := path.Join(origin, part.Destination)
		dest, err := os.OpenFile(destPath, os.O_WRONLY, 0)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", part.Destination, err)
		}
		defer dest.Close()

		bytesCopied, err := io.Copy(dest, source)
		if err != nil || bytesCopied < fi.Size() {
			return fmt.Errorf("couldn't write complete data: %w", err)
		}

		err = dest.Sync()
		if err != nil {
			return fmt.Errorf("couldn't sync content: %w", err)
		}
	}

	return nil
}
