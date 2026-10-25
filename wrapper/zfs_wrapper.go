/* Describes ZFS pools, datasets and swap zvols, validates the
 * configuration and abstracts the zpool and zfs commands. */

package wrapper

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/go-debos/debos"
)

type ZFSDataset struct {
	Name       string
	Properties map[string]string
}

type ZFSSwap struct {
	Name       string
	Size       string
	Properties map[string]string
}

type ZFSEncryption struct {
	Enabled     bool
	Keyformat   string `yaml:"keyformat"`
	Keyfile     string `yaml:"keyfile"`
	Keylocation string `yaml:"keylocation"`
}

/*
ZFSConfig describes the ZFS pool hosted on a partition: the pool,
datasets, an optional swap zvol and optional encryption. It
validates that the configuration is coherent and translates it into
zpool and zfs commands.

A pool which sets BootFS provides the root filesystem of the image,
all other pools are data pools mounted at their datasets
mountpoints. Only structurally required properties are set by debos
(altroot, cachefile=none, mountability of the root filesystem). The
dataset layout and all other properties are recipe policy, see the
OpenZFS "Root on ZFS" documentation for recommendations:
https://openzfs.github.io/openzfs-docs/Getting%20Started/
*/
type ZFSConfig struct {
	Pool              string
	Ashift            int
	Compatibility     string
	BootFS            string
	PoolProperties    map[string]string `yaml:"pool-properties"`
	DatasetProperties map[string]string `yaml:"dataset-properties"`
	Datasets          []ZFSDataset
	Swap              *ZFSSwap
	Encryption        *ZFSEncryption
}

var zfsNameRegexp = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.:-]*$`)

func (z *ZFSConfig) IsRootPool() bool {
	return z.BootFS != ""
}

func (z *ZFSConfig) SetDefaults(partitionName string) {
	if z.Pool == "" {
		z.Pool = partitionName
	}

	if z.Encryption != nil && z.Encryption.Enabled {
		if z.Encryption.Keyformat == "" {
			z.Encryption.Keyformat = "passphrase"
		}

		if z.Encryption.Keylocation == "" {
			z.Encryption.Keylocation = "prompt"
		}
	}
}

func (z *ZFSConfig) Validate() error {
	if !zfsNameRegexp.MatchString(z.Pool) {
		return fmt.Errorf("invalid zfs pool name '%s'", z.Pool)
	}

	if z.IsRootPool() {
		if !strings.HasPrefix(z.BootFS, z.Pool+"/") {
			return fmt.Errorf("zfs bootfs '%s' must be a dataset inside pool '%s'", z.BootFS, z.Pool)
		}

		if len(z.Datasets) == 0 {
			return fmt.Errorf("zfs pool '%s' sets bootfs and therefore requires an explicit 'datasets' list containing the boot environment", z.Pool)
		}

		found := false
		for _, d := range z.Datasets {
			if z.Pool+"/"+d.Name != z.BootFS {
				continue
			}
			found = true

			/* the boot environment must be mountable at / */
			if d.Properties["mountpoint"] != "/" {
				return fmt.Errorf("zfs bootfs dataset '%s' must set the property mountpoint '/'", z.BootFS)
			}
			if d.Properties["canmount"] == "off" {
				return fmt.Errorf("zfs bootfs dataset '%s' may not set canmount 'off'", z.BootFS)
			}
		}

		if !found {
			return fmt.Errorf("zfs bootfs '%s' is not part of the dataset list", z.BootFS)
		}
	}

	if z.Encryption != nil && z.Encryption.Enabled {
		switch z.Encryption.Keyformat {
		case "passphrase", "hex", "raw":
		default:
			return fmt.Errorf("unsupported zfs encryption keyformat '%s'", z.Encryption.Keyformat)
		}

		if z.Encryption.Keyfile == "" {
			return fmt.Errorf("zfs encryption requires a 'keyfile' at image build time")
		}
	}

	if z.Swap != nil {
		if z.Swap.Name == "" {
			return fmt.Errorf("zfs swap name cannot be empty")
		}
		if z.Swap.Size == "" {
			return fmt.Errorf("zfs swap size cannot be empty")
		}
	}

	return nil
}

/* sorting ZFS pool and dataset properties to keep a consistent order and thereby comparability and testability throughout builds */
func sortProperties(flag string, properties map[string]string) []string {
	keys := make([]string, 0, len(properties))
	for k := range properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	args := []string{}
	for _, k := range keys {
		args = append(args, flag, fmt.Sprintf("%s=%s", k, properties[k]))
	}

	return args
}

func (z *ZFSConfig) CreatePool(device string, altroot string, recipeDir string) error {
	/* do not register build pools in the build host zpool.cache */
	poolProperties := map[string]string{
		"cachefile": "none",
	}

	if z.Ashift != 0 {
		poolProperties["ashift"] = fmt.Sprintf("%d", z.Ashift)
	}

	if z.Compatibility != "" {
		poolProperties["compatibility"] = z.Compatibility
	}

	for k, v := range z.PoolProperties {
		poolProperties[k] = v
	}

	datasetProperties := map[string]string{}

	if z.IsRootPool() {
		datasetProperties["canmount"] = "off"
		datasetProperties["mountpoint"] = "/"
	}

	if z.Encryption != nil && z.Encryption.Enabled {
		keyfile := z.Encryption.Keyfile

		if !path.IsAbs(keyfile) {
			keyfile = path.Join(recipeDir, keyfile)
		}

		if _, err := os.Stat(keyfile); err != nil {
			return fmt.Errorf("zfs encryption keyfile: %w", err)
		}

		datasetProperties["encryption"] = "on"
		datasetProperties["keyformat"] = z.Encryption.Keyformat
		datasetProperties["keylocation"] = "file://" + keyfile
	}
	for k, v := range z.DatasetProperties {
		datasetProperties[k] = v
	}

	cmdline := []string{"zpool", "create", "-f"}
	cmdline = append(cmdline, sortProperties("-o", poolProperties)...)
	cmdline = append(cmdline, sortProperties("-O", datasetProperties)...)
	cmdline = append(cmdline, "-R", altroot, z.Pool, device)

	err := debos.Command{}.Run("zpool", cmdline...)
	if err != nil {
		return fmt.Errorf("failed to create zpool '%s': %w", z.Pool, err)
	}

	return nil
}

func (z *ZFSConfig) CreateDatasets() error {
	for _, d := range z.Datasets {
		full := z.Pool + "/" + d.Name

		cmdline := []string{"zfs", "create"}
		cmdline = append(cmdline, sortProperties("-o", d.Properties)...)
		cmdline = append(cmdline, full)

		err := debos.Command{}.Run("zfs", cmdline...)
		if err != nil {
			return fmt.Errorf("failed to create dataset '%s': %w", full, err)
		}

		/* mount boot environment */
		if z.IsRootPool() && full == z.BootFS && d.Properties["canmount"] == "noauto" {
			err := debos.Command{}.Run("zfs", "zfs", "mount", full)
			if err != nil {
				return fmt.Errorf("failed to mount bootfs '%s': %w", full, err)
			}
		}
	}

	if z.IsRootPool() {
		err := debos.Command{}.Run("zpool", "zpool", "set", "bootfs="+z.BootFS, z.Pool)
		if err != nil {
			return fmt.Errorf("failed to set bootfs: %w", err)
		}
	}

	if z.Encryption != nil && z.Encryption.Enabled {
		err := debos.Command{}.Run("zfs", "zfs", "set",
			"keylocation="+z.Encryption.Keylocation, z.Pool)
		if err != nil {
			return fmt.Errorf("failed to set keylocation: %w", err)
		}
	}

	return nil
}

func (z *ZFSConfig) CreateSwap() error {
	if z.Swap == nil {
		return nil
	}

	full := z.Pool + "/" + z.Swap.Name

	cmdline := []string{"zfs", "create", "-V", z.Swap.Size}
	cmdline = append(cmdline, sortProperties("-o", z.Swap.Properties)...)
	cmdline = append(cmdline, full)
	err := debos.Command{}.Run("zfs", cmdline...)
	if err != nil {
		return fmt.Errorf("failed to create swap zvol '%s': %w", full, err)
	}

	/* zvol device links are created asynchronously by udev and zvol_wait blocks until they exist */
	err = debos.Command{}.Run("zvol_wait", "zvol_wait")
	if err != nil {
		return fmt.Errorf("waiting for zvol device links failed: %w", err)
	}

	dev := "/dev/zvol/" + full
	err = debos.Command{}.Run("mkswap", "mkswap", "-f", dev)
	if err != nil {
		return fmt.Errorf("mkswap on '%s' failed: %w", dev, err)
	}

	return nil
}

func (z *ZFSConfig) Export() error {
	return debos.Command{}.Run("zpool", "zpool", "export", z.Pool)
}
