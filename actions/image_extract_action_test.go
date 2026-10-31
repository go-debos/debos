// Copyright (c) Qualcomm Technologies, Inc. and/or its subsidiaries.

package actions

import (
	"bytes"
	"encoding/binary"
	"os"
	"path"
	"syscall"
	"testing"

	"github.com/go-debos/debos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testExtent struct {
	start, count int64
}

func writeMBREntry(sector []byte, idx int, kind byte, start, count int64) {
	e := sector[446+16*idx:]
	e[4] = kind
	binary.LittleEndian.PutUint32(e[8:12], uint32(start))
	binary.LittleEndian.PutUint32(e[12:16], uint32(count))
	sector[510] = 0x55
	sector[511] = 0xAA
}

func TestGPTPartitionExtent(t *testing.T) {
	for _, sectorSize := range []int64{512, 4096} {
		img := make([]byte, 64*sectorSize)
		header := img[sectorSize:]
		copy(header, "EFI PART")
		binary.LittleEndian.PutUint64(header[72:80], 2)
		binary.LittleEndian.PutUint32(header[80:84], 128)
		binary.LittleEndian.PutUint32(header[84:88], 128)

		parts := []testExtent{{34, 10}, {44, 20}}
		for i, p := range parts {
			e := img[2*sectorSize+int64(i)*128:]
			binary.LittleEndian.PutUint64(e[32:40], uint64(p.start))
			binary.LittleEndian.PutUint64(e[40:48], uint64(p.start+p.count-1))
		}

		for i, p := range parts {
			start, size, err := partitionExtent(bytes.NewReader(img), "gpt", i+1, sectorSize)
			require.NoError(t, err)
			assert.Equal(t, p.start*sectorSize, start)
			assert.Equal(t, p.count*sectorSize, size)
		}

		_, _, err := partitionExtent(bytes.NewReader(img), "gpt", 3, sectorSize)
		assert.Error(t, err)
	}
}

func TestMBRPartitionExtent(t *testing.T) {
	const sectorSize = 512
	img := make([]byte, 256*sectorSize)

	// Three primary partitions and an extended partition holding two
	// logical partitions, as created by image-partition for 5 partitions
	writeMBREntry(img, 0, 0x83, 1, 10)
	writeMBREntry(img, 1, 0x83, 11, 10)
	writeMBREntry(img, 2, 0x83, 21, 10)
	writeMBREntry(img, 3, 0x0F, 40, 100)

	// First EBR at the start of the extended partition, logical partition
	// start is relative to the EBR, next EBR relative to extended partition
	writeMBREntry(img[40*sectorSize:], 0, 0x83, 2, 20)
	writeMBREntry(img[40*sectorSize:], 1, 0x05, 30, 40)
	writeMBREntry(img[70*sectorSize:], 0, 0x83, 2, 30)

	expected := map[int]testExtent{
		1: {1, 10},
		2: {11, 10},
		3: {21, 10},
		5: {42, 20},
		6: {72, 30},
	}

	for number, p := range expected {
		start, size, err := partitionExtent(bytes.NewReader(img), "msdos", number, sectorSize)
		require.NoError(t, err, "partition %d", number)
		assert.Equal(t, p.start*sectorSize, start, "partition %d", number)
		assert.Equal(t, p.count*sectorSize, size, "partition %d", number)
	}

	for _, number := range []int{0, 7} {
		_, _, err := partitionExtent(bytes.NewReader(img), "msdos", number, sectorSize)
		assert.Error(t, err, "partition %d", number)
	}
}

func TestSparseCopy(t *testing.T) {
	const size = 4*1024*1024 + 123
	data := make([]byte, size)
	copy(data, "start")
	copy(data[size-3:], "end")

	dest := path.Join(t.TempDir(), "out.img")
	require.NoError(t, sparseCopy(dest, bytes.NewReader(data), size))

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, data, got)

	var st syscall.Stat_t
	require.NoError(t, syscall.Stat(dest, &st))
	assert.Less(t, st.Blocks*512, int64(size), "output is not sparse")

	assert.Error(t, sparseCopy(dest, bytes.NewReader(data[:10]), size))
}

func TestImageExtractVerify(t *testing.T) {
	tests := []struct {
		name       string
		partitions []ExtractPartition
		err        string
	}{
		{"empty", nil, ""},
		{"default destinations", []ExtractPartition{{Name: "a"}, {Name: "b"}}, ""},
		{"missing name", []ExtractPartition{{Destination: "a.img"}}, "partition name is a mandatory property"},
		{"absolute destination", []ExtractPartition{{Name: "a", Destination: "/a.img"}}, "destination /a.img must be relative to the artifact directory"},
		{"duplicate destination", []ExtractPartition{{Name: "a", Destination: "x.img"}, {Name: "b", Destination: "./x.img"}}, "destination ./x.img is used by multiple partitions"},
		{"duplicate default destination", []ExtractPartition{{Name: "a"}, {Name: "b", Destination: "a.img"}}, "destination a.img is used by multiple partitions"},
	}

	context := debos.Context{CommonContext: &debos.CommonContext{}}
	action := ImageExtractAction{}
	assert.EqualError(t, action.Verify(&context), "image-extract must come after an image-partition action")

	context.HasImagePartition = true
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := ImageExtractAction{Partitions: test.partitions}
			err := action.Verify(&context)
			if test.err == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.err)
			}
		})
	}
}
