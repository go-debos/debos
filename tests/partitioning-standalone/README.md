# Standalone Partitioning Integration Test

This test verifies the `standalone: true` mode of the `image-partition` action, which creates separate filesystem image files instead of a traditional partitioned disk image.

## Test Coverage

The test verifies:

1. **Separate Sparse Files** - Confirms that `test.img1` and `test.img2` are created as separate standalone partition files instead of a single partitioned image file.

2. **Filesystem Formatting** - Tests multiple filesystem types:
   - ext2 on boot partition (256M)
   - ext4 on root partition (512M)

3. **Filesystem Integrity** - Validates filesystem consistency using fsck in read-only mode, confirming both filesystems are properly formatted and valid.

4. **Partition File Sizes** - Confirms each partition file is created with the correct size, matching the configured values (with tolerance for filesystem overhead).

## Partition Configuration

- **boot**: ext2, 256M
- **root**: ext4, 512M

## Important Notes on Standalone Mode

- **Explicit sizes required**: Standalone mode uses explicit partition sizes (e.g., `256M`), not percentages like `0%` to `256M`.
- **Start position**: The `start` value must be `0` for standalone partitions (disk position doesn't apply to separate files).
- **End value**: The `end` value specifies the partition size directly.
- **No partition table**: Standalone mode creates separate files, not a disk image with a partition table.
- **File naming**: Partition images are named `imagename1`, `imagename2`, etc.
- **Build-time mounting**: Standalone partition files are mounted during the build process so downstream actions can populate them; they remain separate deployment artifacts.
- **Filesystem validation**: Filesystems are validated using fsck in read-only mode, not through mount operations.

## Running the Test

### With Fakemachine (Recommended for CI)

```bash
docker run --rm --device /dev/kvm \
  -v $(pwd)/tests:/tests -w /tests \
  --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v partitioning-standalone/test.yaml
```

### Without Fakemachine

```bash
docker run --rm --device /dev/kvm \
  -v $(pwd)/tests:/tests -w /tests \
  --tmpfs /scratch:exec --tmpfs /run -e TMP=/scratch \
  debos -v --disable-fakemachine partitioning-standalone/test.yaml
```

Both execution paths should produce identical results.

## Expected Output

The test should:
1. Create two separate sparse image files (`test.img1` for boot, `test.img2` for root)
2. Format boot with ext2 and root with ext4
3. Verify filesystem integrity with fsck for both partitions
4. Confirm partition file sizes are within expected ranges
5. Complete with all verification checks passing

## Validation Steps

The test includes several validation stages:
- **File existence and size**: Confirms partition image files were created with correct sizes (within 10% tolerance)
- **Filesystem detection**: Uses `file -s` to confirm correct filesystem types are present
- **Filesystem integrity**: Runs `fsck -n` (read-only mode) to validate filesystem structure and consistency
- **Summary verification**: Final confirmation that all standalone partition tests passed
