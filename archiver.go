package debos

import (
	"log"
	"os"
)

func TarOptions(compression string) string {
	unpackTarOpts := map[string]string{
		"gz":    "-z",
		"bzip2": "-j",
		"xz":    "-J",
	} // Trying to guess all other supported formats

	return unpackTarOpts[compression]
}

func UnpackTarArchive(infile, destination, compression string, options ...string) error {
	if err := os.MkdirAll(destination, 0755); err != nil {
		return err
	}
	log.Printf("Unpacking %s\n", infile)

	command := []string{"tar"}
	command = append(command, options...)
	command = append(command, "-x")
	if unpackTarOpt := TarOptions(compression); len(unpackTarOpt) > 0 {
		command = append(command, unpackTarOpt)
	}
	command = append(command, "-f", infile, "-C", destination)

	return Command{}.Run("unpack", command...)
}
