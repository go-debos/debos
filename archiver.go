package debos

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ArchiveType int

// Supported types
const (
	_ ArchiveType = iota // Guess archive type from file extension
	Tar
	Zip
	Deb
)

type ArchiveBase struct {
	file    string // Path to archive file
	atype   ArchiveType
	options map[interface{}]interface{} // Archiver-depending map with additional hints
}
type ArchiveTar struct {
	ArchiveBase
}
type ArchiveZip struct {
	ArchiveBase
}
type ArchiveDeb struct {
	ArchiveBase
}

type Unpacker interface {
	Unpack(destination string) error
	RelaxedUnpack(destination string) error
}

type Archiver interface {
	Type() ArchiveType
	AddOption(key, value interface{}) error
	Unpacker
}

type Archive struct {
	Archiver
}

// Unpack archive as is
func (arc *ArchiveBase) Unpack(destination string) error {
	return fmt.Errorf("Unpack is not supported for '%s'", arc.file)
}

/*
RelaxedUnpack unpack archive in relaxed mode allowing to ignore or
avoid minor issues with unpacker tool or framework.
*/
func (arc *ArchiveBase) RelaxedUnpack(destination string) error {
	return arc.Unpack(destination)
}

func (arc *ArchiveBase) AddOption(key, value interface{}) error {
	if arc.options == nil {
		arc.options = make(map[interface{}]interface{})
	}
	arc.options[key] = value
	return nil
}

func (arc *ArchiveBase) Type() ArchiveType { return arc.atype }

// Helper function for unpacking with external tool
func unpack(command []string, destination string) error {
	if err := os.MkdirAll(destination, 0755); err != nil {
		return err
	}
	return Command{}.Run("unpack", command...)
}

// Helper function for checking allowed compression types
// Returns empty string for unknown
func tarOptions(compression string) string {
	unpackTarOpts := map[string]string{
		"gz":    "-z",
		"bzip2": "-j",
		"xz":    "-J",
	} // Trying to guess all other supported compression types

	return unpackTarOpts[compression]
}

func (tar *ArchiveTar) Unpack(destination string) error {
	command := []string{"tar"}
	if options, ok := tar.options["taroptions"].([]string); ok {
		for _, option := range options {
			command = append(command, option)
		}
	}
	command = append(command, "-C", destination)
	command = append(command, "-x")
	command = append(command, "--xattrs")
	command = append(command, "--xattrs-include=*.*")

	if compression, ok := tar.options["tarcompression"]; ok {
		if unpackTarOpt := tarOptions(compression.(string)); len(unpackTarOpt) > 0 {
			command = append(command, unpackTarOpt)
		}
	}
	command = append(command, "-f", tar.file)

	return unpack(command, destination)
}

func (tar *ArchiveTar) RelaxedUnpack(destination string) error {

	taroptions := []string{"--no-same-owner", "--no-same-permissions"}
	options, ok := tar.options["taroptions"].([]string)
	defer func() { tar.options["taroptions"] = options }()

	if ok {
		for _, option := range options {
			taroptions = append(taroptions, option)
		}
	}
	tar.options["taroptions"] = taroptions

	return tar.Unpack(destination)
}

func (tar *ArchiveTar) AddOption(key, value interface{}) error {

	switch key {
	case "taroptions":
		// expect a slice
		options, ok := value.([]string)
		if !ok {
			return fmt.Errorf("Wrong type for value")
		}
		tar.options["taroptions"] = options

	case "tarcompression":
		compression, ok := value.(string)
		if !ok {
			return fmt.Errorf("Wrong type for value")
		}
		option := tarOptions(compression)
		if len(option) == 0 {
			return fmt.Errorf("Compression '%s' is not supported", compression)
		}
		tar.options["tarcompression"] = compression

	default:
		return fmt.Errorf("Option '%v' is not supported for tar archive type", key)
	}
	return nil
}

func (zip *ArchiveZip) Unpack(destination string) error {
	command := []string{"unzip", zip.file, "-d", destination}
	return unpack(command, destination)
}

func (zip *ArchiveZip) RelaxedUnpack(destination string) error {
	return zip.Unpack(destination)
}

func (deb *ArchiveDeb) Unpack(destination string) error {
	command := []string{"dpkg", "-x", deb.file, destination}
	return unpack(command, destination)
}

func (deb *ArchiveDeb) RelaxedUnpack(destination string) error {
	return deb.Unpack(destination)
}

/*
NewArchive associate correct structure and methods according to
archive type. If ArchiveType is omitted -- trying to guess the type.
Return ArchiveType or nil in case of error.
*/
func NewArchive(file string, arcType ...ArchiveType) (Archive, error) {
	var archive Archive
	var atype ArchiveType

	if len(arcType) == 0 {
		ext := filepath.Ext(file)
		ext = strings.ToLower(ext)

		switch ext {
		case ".deb":
			atype = Deb
		case ".zip":
			atype = Zip
		default:
			//FIXME: guess Tar maybe?
			atype = Tar
		}
	} else {
		atype = arcType[0]
	}

	common := ArchiveBase{}
	common.file = file
	common.atype = atype
	common.options = make(map[interface{}]interface{})

	switch atype {
	case Tar:
		archive = Archive{&ArchiveTar{ArchiveBase: common}}
	case Zip:
		archive = Archive{&ArchiveZip{ArchiveBase: common}}
	case Deb:
		archive = Archive{&ArchiveDeb{ArchiveBase: common}}
	default:
		return archive, fmt.Errorf("Unsupported archive '%s'", file)
	}
	return archive, nil
}
