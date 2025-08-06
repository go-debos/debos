package debos_test

import (
	_ "fmt"
	"github.com/go-debos/debos"
	"github.com/stretchr/testify/assert"
	_ "reflect"
	_ "strings"
	"testing"
)

func TestBase(t *testing.T) {

	// New archive
	// Expect Tar by default
	_, err := debos.NewArchive("test.base", 0)
	assert.EqualError(t, err, "Unsupported archive 'test.base'")

	// Test base
	archive := debos.ArchiveBase{}
	arcType := archive.Type()
	assert.Equal(t, 0, int(arcType))

	// Add  option
	err = archive.AddOption("someoption", "somevalue")
	assert.Empty(t, err)

	err = archive.Unpack("/tmp/test")
	assert.EqualError(t, err, "Unpack is not supported for ''")
	err = archive.RelaxedUnpack("/tmp/test")
	assert.EqualError(t, err, "Unpack is not supported for ''")
}

func TestTar_default(t *testing.T) {

	// New archive
	// Expect Tar by default
	archive, err := debos.NewArchive("test.tar.gz")
	assert.NotEmpty(t, archive)
	assert.Empty(t, err)

	// Type must be Tar by default
	arcType := archive.Type()
	assert.Equal(t, debos.Tar, arcType)

	// Test unpack
	err = archive.Unpack("/tmp/test")
	// Expect unpack failure
	assert.EqualError(t, err, "exit status 2")

	// Expect failure for RelaxedUnpack
	err = archive.RelaxedUnpack("/tmp/test")
	assert.EqualError(t, err, "exit status 2")

	// Check options
	err = archive.AddOption("taroptions", []string{"--option1"})
	assert.Empty(t, err)
	err = archive.Unpack("/tmp/test")
	assert.EqualError(t, err, "exit status 64")
	err = archive.Unpack("/proc/debostest")
	assert.EqualError(t, err, "mkdir /proc/debostest: no such file or directory")
	err = archive.RelaxedUnpack("/tmp/test")
	assert.EqualError(t, err, "exit status 64")

	// Add wrong option
	err = archive.AddOption("someoption", "somevalue")
	assert.EqualError(t, err, "Option 'someoption' is not supported for tar archive type")
}

// Check supported compression types
func TestTar_compression(t *testing.T) {
	compressions := map[string]string{
		"gz":    "tar -C test -x -z -f test.tar.gz",
		"bzip2": "tar -C test -x -j -f test.tar.gz",
		"xz":    "tar -C test -x -J -f test.tar.gz",
	}

	// Force type
	archive, err := debos.NewArchive("test.tar.gz", debos.Tar)
	assert.NotEmpty(t, archive)
	assert.Empty(t, err)
	// Type must be Tar
	arcType := archive.Type()
	assert.Equal(t, debos.Tar, arcType)

	for compression := range compressions {
		err = archive.AddOption("tarcompression", compression)
		assert.Empty(t, err)
		err := archive.Unpack("test")
		assert.EqualError(t, err, "exit status 2")
	}
	// Check of unsupported compression type
	err = archive.AddOption("tarcompression", "fake")
	assert.EqualError(t, err, "Compression 'fake' is not supported")

	// Pass incorrect type
	err = archive.AddOption("taroptions", nil)
	assert.EqualError(t, err, "Wrong type for value")
	err = archive.AddOption("tarcompression", nil)
	assert.EqualError(t, err, "Wrong type for value")
}

func TestDeb(t *testing.T) {

	// Guess Deb
	archive, err := debos.NewArchive("test.deb")
	assert.NotEmpty(t, archive)
	assert.Empty(t, err)

	// Type must be guessed as Deb
	arcType := archive.Type()
	assert.Equal(t, debos.Deb, arcType)

	// Force Deb type
	archive, err = debos.NewArchive("test.deb", debos.Deb)
	assert.NotEmpty(t, archive)
	assert.Empty(t, err)

	// Type must be Deb
	arcType = archive.Type()
	assert.Equal(t, debos.Deb, arcType)

	// Expect unpack failure
	err = archive.Unpack("/tmp/test")
	assert.EqualError(t, err, "exit status 2")
	err = archive.Unpack("/proc/debostest")
	assert.EqualError(t, err, "mkdir /proc/debostest: no such file or directory")
	err = archive.RelaxedUnpack("/tmp/test")
	assert.EqualError(t, err, "exit status 2")
}

func TestZip(t *testing.T) {
	// Guess zip
	archive, err := debos.NewArchive("test.ZiP")
	assert.NotEmpty(t, archive)
	assert.Empty(t, err)
	// Type must be guessed as Zip
	arcType := archive.Type()
	assert.Equal(t, debos.Zip, arcType)

	// Force Zip type
	archive, err = debos.NewArchive("test.zip", debos.Zip)
	assert.NotEmpty(t, archive)
	assert.Empty(t, err)

	// Type must be Zip
	arcType = archive.Type()
	assert.Equal(t, debos.Zip, arcType)

	// Expect unpack failure
	err = archive.Unpack("/tmp/test")
	assert.EqualError(t, err, "exit status 9")
	err = archive.Unpack("/proc/debostest")
	assert.EqualError(t, err, "mkdir /proc/debostest: no such file or directory")
	err = archive.RelaxedUnpack("/tmp/test")
	assert.EqualError(t, err, "exit status 9")
}
