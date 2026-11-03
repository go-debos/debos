package debos

import (
	"fmt"
	"log"
	"os"
)

/*
DebugShell function launches an interactive shell for
debug and problems investigation.
*/
func DebugShell(context Context) {
	if len(context.DebugShell) == 0 {
		return
	}

	// When fakemachine is used, the scratch directory only exists inside the
	// machine, so for failures on the host fall back to the artifact directory.
	dir := context.Scratchdir
	if _, err := os.Stat(dir); err != nil {
		dir = context.Artifactdir
	}

	pa := os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Dir:   dir,
	}

	// Start an interactive shell for debug.
	log.Printf(">>> Starting a debug shell")
	if proc, err := os.StartProcess(context.DebugShell, []string{}, &pa); err != nil {
		fmt.Printf("Failed: %s\n", err)
	} else {
		_, _ = proc.Wait()
	}
}
