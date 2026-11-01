package debos

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ArchiveType identifies the format of an archive.
type ArchiveType int

// Supported types.
const (
	_ ArchiveType = iota // Guess archive type from file extension
	Tar
	Zip
	Deb
)

// ArchiveBase holds the state common to all archive formats and rejects
// unpacking; concrete archive types embed it and override Unpack.
type ArchiveBase struct {
	file    string // Path to archive file
	atype   ArchiveType
	options map[any]any // Archiver-depending map with additional hints
}

// ArchiveTar is a tarball, optionally compressed.
type ArchiveTar struct {
	ArchiveBase
}

// ArchiveZip is a zip archive.
type ArchiveZip struct {
	ArchiveBase
}

// ArchiveDeb is a Debian binary package.
type ArchiveDeb struct {
	ArchiveBase
}

// Unpacker extracts the contents of an archive.
type Unpacker interface {
	Unpack(destination string) error
	RelaxedUnpack(destination string) error
}

// Archiver is an archive that can be configured and unpacked.
type Archiver interface {
	Type() ArchiveType
	AddOption(key, value any) error
	Unpacker
}

// Archive wraps the Archiver matching the format of an archive file.
type Archive struct {
	Archiver
}

// Unpack archive as is.
func (arc *ArchiveBase) Unpack(_ string) error {
	return fmt.Errorf("Unpack is not supported for '%s'", arc.file)
}

/*
RelaxedUnpack unpack archive in relaxed mode allowing to ignore or
avoid minor issues with unpacker tool or framework.
*/
func (arc *ArchiveBase) RelaxedUnpack(destination string) error {
	return arc.Unpack(destination)
}

// AddOption stores a format-specific hint for the unpacker.
func (arc *ArchiveBase) AddOption(key, value any) error {
	if arc.options == nil {
		arc.options = make(map[any]any)
	}
	arc.options[key] = value

	return nil
}

// Type returns the format of the archive.
func (arc *ArchiveBase) Type() ArchiveType { return arc.atype }

// Helper function for unpacking with external tool.
func unpack(command []string, destination string) error {
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destination, err)
	}
	err := Command{}.Run("unpack", command...)
	if err != nil {
		return fmt.Errorf("failed to unpack to %s: %w", destination, err)
	}

	return nil
}

// Helper function for checking allowed compression types
// Returns the tar option for the compression type and whether it is supported.
func tarOptions(compression string) (string, bool) {
	unpackTarOpts := map[string]string{
		"bzip2": "--bzip2",
		"gz":    "--gzip",
		"lzip":  "--lzip",
		"lzma":  "--lzma",
		"lzop":  "--lzop",
		"xz":    "--xz",
		"zstd":  "--zstd",
		"none":  "",
	} // Trying to guess all other supported compression types

	option, ok := unpackTarOpts[compression]

	return option, ok
}

// Unpack extracts the tarball into destination.
func (tar *ArchiveTar) Unpack(destination string) error {
	command := []string{"tar"}
	usePigz := false
	if compression, ok := tar.options["tarcompression"]; ok && compression == "gz" {
		if _, err := exec.LookPath("pigz"); err == nil {
			usePigz = true
		}
	}
	if options, ok := tar.options["taroptions"].([]string); ok {
		command = append(command, options...)
	}
	command = append(command, "-C", destination)
	command = append(command, "-x")
	command = append(command, "--xattrs")
	command = append(command, "--xattrs-include=*.*")

	if compression, ok := tar.options["tarcompression"].(string); ok {
		if unpackTarOpt, _ := tarOptions(compression); len(unpackTarOpt) > 0 {
			if usePigz {
				command = append(command, "--use-compress-program=pigz")
			} else {
				command = append(command, unpackTarOpt)
			}
		}
	}
	command = append(command, "-f", tar.file)

	return unpack(command, destination)
}

// RelaxedUnpack extracts the tarball into destination, ignoring ownership
// and permission errors that an unprivileged tar cannot honour.
func (tar *ArchiveTar) RelaxedUnpack(destination string) error {
	taroptions := []string{"--no-same-owner", "--no-same-permissions"}
	options, ok := tar.options["taroptions"].([]string)
	defer func() { tar.options["taroptions"] = options }()

	if ok {
		taroptions = append(taroptions, options...)
	}
	tar.options["taroptions"] = taroptions

	return tar.Unpack(destination)
}

// AddOption accepts the "taroptions" ([]string of extra tar arguments) and
// "tarcompression" (compression name) options, and rejects any other.
func (tar *ArchiveTar) AddOption(key, value any) error {
	switch key {
	case "taroptions":
		// expect a slice
		options, ok := value.([]string)
		if !ok {
			return errors.New("wrong type for value")
		}
		tar.options["taroptions"] = options

	case "tarcompression":
		compression, ok := value.(string)
		if !ok {
			return errors.New("wrong type for value")
		}
		if _, ok := tarOptions(compression); !ok {
			return fmt.Errorf("compression '%s' is not supported", compression)
		}
		tar.options["tarcompression"] = compression

	default:
		return fmt.Errorf("option '%v' is not supported for tar archive type", key)
	}

	return nil
}

// Unpack extracts the zip archive into destination.
func (zip *ArchiveZip) Unpack(destination string) error {
	command := []string{"unzip", zip.file, "-d", destination}

	return unpack(command, destination)
}

// RelaxedUnpack is the same as Unpack, as unzip has no relaxed mode.
func (zip *ArchiveZip) RelaxedUnpack(destination string) error {
	return zip.Unpack(destination)
}

// Unpack extracts the contents of the package into destination.
func (deb *ArchiveDeb) Unpack(destination string) error {
	command := []string{"dpkg", "-x", deb.file, destination}

	return unpack(command, destination)
}

// RelaxedUnpack is the same as Unpack, as dpkg-deb has no relaxed mode.
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
			// FIXME: guess Tar maybe?
			atype = Tar
		}
	} else {
		atype = arcType[0]
	}

	common := ArchiveBase{}
	common.file = file
	common.atype = atype
	common.options = make(map[any]any)

	switch atype {
	case Tar:
		archive = Archive{&ArchiveTar{ArchiveBase: common}}
	case Zip:
		archive = Archive{&ArchiveZip{ArchiveBase: common}}
	case Deb:
		archive = Archive{&ArchiveDeb{ArchiveBase: common}}
	default:
		return archive, fmt.Errorf("unsupported archive '%s'", file)
	}

	return archive, nil
}
