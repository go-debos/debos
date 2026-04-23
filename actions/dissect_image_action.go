/*
DissectImage Action

Dissect allows to pick out parts of the created image from the ImagePartition
and FilesystemDeploy actions, and save those parts as raw image files. In a way,
this is the Raw action in reverse.

	# Yaml syntax:
	- action: image-dissect
	  parts:
	  	<list of parts>

Mandatory properties:
- parts -- list of parts of the image to dissect. These are processed one by one
and may overlap.

Optional properties:

	# Yaml syntax for parts:
	  parts:
	    - partname: partition name
		  start: offset
		  end: offset
		  destination: filename

Mandatory properties:
- destination -- where to store the extracted part

Optional properties:
- partname -- to extract a full partition, matching on partname is the easiest

- start -- offset in bytes or in sector numbers, e.g. 256s. This becomes the starting
point from where to extract when partname is not given.

- end -- offset in bytes or in sector numbers, e.g. 512s. This becomes the end point
at which the extraction stops, when partname is not given.

The sector size is either the recipe header 'sectorsize' or the default 512 sector
size.
*/
package actions

import (
	"github.com/go-debos/debos"
)

type ImagePart struct {
	PartName	string
	StartOffset string
	EndOffset	string
	// TBD
}

type ImageDissectAction struct {
	debos.BaseAction    `yaml:",inline"`
	Parts			 []ImagePart
	// TBD
}

func (dis *ImageDissectAction) Verify(ctx *debos.Context) error {
	// TODO
	return nil
}

func (dis *ImageDissectAction) PostMachine(ctx *debos.Context) error {
	// Has to run after the images have been unmounted.
	// TODO
	return nil
}
