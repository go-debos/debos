// Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries.

/*
ImageExtract Action

This action extracts partitions of the image created by the image-partition
action into separate raw image files, e.g. for usage in update bundles. In a
way, this is the raw action in reverse. The image-partition action must come
before this action in the recipe.

The extraction happens on the host once the image has been fully built and
its partitions have been unmounted, so the extracted images are consistent.
The extracted images are sparse.

	# Yaml syntax:
	- action: image-extract
	  partitions:
	    <list of partitions>

Optional properties:

- partitions -- list of partitions to extract. If not set, all partitions are
extracted using the default destination.

	# Yaml syntax for partitions:
	  partitions:
	    - name: partition name
	      destination: filename

Mandatory properties:

- name -- name of the partition to extract, as set in the image-partition action.

Optional properties:

- destination -- name of the file to write the partition to, relative to the
artifact directory. Defaults to the partition name with the suffix ".img".
*/
package actions

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/go-debos/debos"
)

type ExtractPartition struct {
	Name        string
	Destination string
}

type ImageExtractAction struct {
	debos.BaseAction `yaml:",inline"`
	Partitions       []ExtractPartition
}

func (e *ImageExtractAction) Verify(context *debos.Context) error {
	if !context.HasImagePartition {
		return errors.New("image-extract must come after an image-partition action")
	}

	// Can't verify the partitions exist at this point as the image layout
	// isn't known until image-partition has been ran, see PostMachine
	destinations := map[string]bool{}
	for idx := range e.Partitions {
		p := &e.Partitions[idx]
		if p.Name == "" {
			return errors.New("partition name is a mandatory property")
		}

		if p.Destination == "" {
			p.Destination = p.Name + ".img"
		}

		if !filepath.IsLocal(p.Destination) {
			return fmt.Errorf("destination %s must be relative to the artifact directory", p.Destination)
		}

		dest := filepath.Clean(p.Destination)
		if destinations[dest] {
			return fmt.Errorf("destination %s is used by multiple partitions", p.Destination)
		}
		destinations[dest] = true
	}

	return nil
}

func (e *ImageExtractAction) PostMachine(context *debos.Context) error {
	layout := context.ImageLayout
	if layout == nil {
		return errors.New("no image layout found, missing image-partition action?")
	}

	img, err := os.Open(layout.Path)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}
	defer img.Close()

	if len(e.Partitions) == 0 {
		for name := range layout.Partitions {
			if !filepath.IsLocal(name) {
				return fmt.Errorf("partition name %s can't be used as destination", name)
			}
			e.Partitions = append(e.Partitions, ExtractPartition{Name: name, Destination: name + ".img"})
		}
		sort.Slice(e.Partitions, func(a, b int) bool {
			return layout.Partitions[e.Partitions[a].Name] < layout.Partitions[e.Partitions[b].Name]
		})
	}

	for _, p := range e.Partitions {
		number, ok := layout.Partitions[p.Name]
		if !ok {
			return fmt.Errorf("failed to find partition named %s", p.Name)
		}

		start, size, err := partitionExtent(img, layout.PartitionType, number, int64(context.SectorSize))
		if err != nil {
			return fmt.Errorf("failed to locate partition %s: %w", p.Name, err)
		}

		dest := path.Join(context.Artifactdir, p.Destination)
		if err := sparseCopy(dest, io.NewSectionReader(img, start, size), size); err != nil {
			return fmt.Errorf("failed to extract partition %s: %w", p.Name, err)
		}
	}

	return nil
}

func (e *ImageExtractAction) PostMachineCleanup(context *debos.Context) error {
	if context.State != debos.Success {
		for _, p := range e.Partitions {
			os.Remove(path.Join(context.Artifactdir, p.Destination))
		}
	}
	return nil
}

// Look up the offset and size in bytes of a partition from the partition table
func partitionExtent(img io.ReaderAt, partitionType string, number int, sectorSize int64) (int64, int64, error) {
	switch partitionType {
	case "gpt":
		return gptPartitionExtent(img, number, sectorSize)
	case "msdos":
		return mbrPartitionExtent(img, number, sectorSize)
	default:
		return 0, 0, fmt.Errorf("unsupported partition type %s", partitionType)
	}
}

func gptPartitionExtent(img io.ReaderAt, number int, sectorSize int64) (int64, int64, error) {
	header := make([]byte, 92)
	if _, err := img.ReadAt(header, sectorSize); err != nil {
		return 0, 0, fmt.Errorf("failed to read GPT header: %w", err)
	}

	if string(header[0:8]) != "EFI PART" {
		return 0, 0, errors.New("no GPT header found")
	}

	entriesLBA := int64(binary.LittleEndian.Uint64(header[72:80]))
	numEntries := int(binary.LittleEndian.Uint32(header[80:84]))
	entrySize := int64(binary.LittleEndian.Uint32(header[84:88]))

	if number < 1 || number > numEntries || entrySize < 48 {
		return 0, 0, fmt.Errorf("partition %d not found in GPT", number)
	}

	entry := make([]byte, 48)
	if _, err := img.ReadAt(entry, entriesLBA*sectorSize+int64(number-1)*entrySize); err != nil {
		return 0, 0, fmt.Errorf("failed to read GPT entry: %w", err)
	}

	first := int64(binary.LittleEndian.Uint64(entry[32:40]))
	last := int64(binary.LittleEndian.Uint64(entry[40:48]))
	if first == 0 || last < first {
		return 0, 0, fmt.Errorf("partition %d not found in GPT", number)
	}

	return first * sectorSize, (last - first + 1) * sectorSize, nil
}

type mbrEntry struct {
	kind  byte
	start int64
	count int64
}

func readMBR(img io.ReaderAt, offset int64) ([4]mbrEntry, error) {
	var entries [4]mbrEntry

	mbr := make([]byte, 512)
	if _, err := img.ReadAt(mbr, offset); err != nil {
		return entries, fmt.Errorf("failed to read MBR: %w", err)
	}

	if mbr[510] != 0x55 || mbr[511] != 0xAA {
		return entries, errors.New("no MBR signature found")
	}

	for i := range entries {
		e := mbr[446+16*i:]
		entries[i] = mbrEntry{
			kind:  e[4],
			start: int64(binary.LittleEndian.Uint32(e[8:12])),
			count: int64(binary.LittleEndian.Uint32(e[12:16])),
		}
	}

	return entries, nil
}

func isExtended(kind byte) bool {
	return kind == 0x05 || kind == 0x0F || kind == 0x85
}

func mbrPartitionExtent(img io.ReaderAt, number int, sectorSize int64) (int64, int64, error) {
	primary, err := readMBR(img, 0)
	if err != nil {
		return 0, 0, err
	}

	if number >= 1 && number <= 4 {
		e := primary[number-1]
		if e.kind == 0 || e.count == 0 {
			return 0, 0, fmt.Errorf("partition %d not found in MBR", number)
		}
		return e.start * sectorSize, e.count * sectorSize, nil
	}

	// Logical partitions are stored in a chain of extended boot records
	var extStart int64
	for _, e := range primary {
		if isExtended(e.kind) {
			extStart = e.start
			break
		}
	}

	if number < 5 || extStart == 0 {
		return 0, 0, fmt.Errorf("partition %d not found in MBR", number)
	}

	ebr := extStart
	for n := 5; ; n++ {
		entries, err := readMBR(img, ebr*sectorSize)
		if err != nil {
			return 0, 0, err
		}

		if n == number {
			if entries[0].count == 0 {
				break
			}
			return (ebr + entries[0].start) * sectorSize, entries[0].count * sectorSize, nil
		}

		if !isExtended(entries[1].kind) || entries[1].start == 0 {
			break
		}
		ebr = extStart + entries[1].start
	}

	return 0, 0, fmt.Errorf("partition %d not found in MBR", number)
}

// Copy size bytes from source to a new file at dest, leaving holes for blocks
// which only contain zeroes
func sparseCopy(dest string, source io.Reader, size int64) error {
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer out.Close()

	buf := make([]byte, 1024*1024)
	zero := make([]byte, len(buf))
	var offset int64

	for {
		n, err := io.ReadFull(source, buf)
		if n > 0 && !bytes.Equal(buf[:n], zero[:n]) {
			if _, err := out.WriteAt(buf[:n], offset); err != nil {
				return err
			}
		}
		offset += int64(n)

		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			return err
		}
	}

	if offset != size {
		return fmt.Errorf("short read, got %d of %d bytes", offset, size)
	}

	if err := out.Truncate(size); err != nil {
		return err
	}

	return out.Sync()
}
