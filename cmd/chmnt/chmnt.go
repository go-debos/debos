package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/jessevdk/go-flags"
)

func main() {
	var options struct {
		Mounts      map[string]string `short:"m" long:"mount" description:"A remapped path (use SOURCE:TARGET syntax)"`
		EnvironVars map[string]string `short:"e" long:"env-var" description:"An additional environment variable for the command (use VARIABLE:VALUE syntax)"`
	}

	log.SetPrefix(os.Args[0] + ": ")
	log.SetOutput(os.Stderr)
	log.SetFlags(0)

	exitcode := 1
	defer func() {
		os.Exit(exitcode)
	}()

	parser := flags.NewParser(&options, flags.Default)

	args, err := parser.Parse()
	if err != nil {
		flagsErr, ok := err.(*flags.Error)
		if ok && flagsErr.Type == flags.ErrHelp {
			exitcode = 0
		} else {
			log.Printf("Failed to parse arguments: %v\n", err)
		}

		return
	}

	if len(args) == 0 {
		log.Printf("No target program given!")
		return
	}

	if len(options.Mounts) > 0 {
		// We need to stay on the same thread, because the syscalls
		// operate only on one thread
		runtime.LockOSThread()

		if err := syscall.Unshare(syscall.CLONE_NEWNS); err != nil {
			log.Printf("Failed to unshare mount namespace: %v\n", err)
			return
		}

		if err := syscall.Mount("none", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
			log.Printf("Failed to set mount sharing to private: %v", err)
			return
		}

		for source, target := range options.Mounts {
			s, err := filepath.Abs(source)
			if err != nil {
				log.Printf("Cannot process source directory %s: %v\n", source, err)
				return
			}
			t, err := filepath.Abs(target)
			if err != nil {
				log.Printf("Cannot process target directory %s: %v\n", target, err)
				return
			}
			if err := syscall.Mount(s, t, "", syscall.MS_BIND, ""); err != nil {
				log.Printf("Could not bind %s to %s: %v\n", source, target, err)
				return
			}
		}
	}

	envmap := make(map[string]string)
	for _, v := range os.Environ() {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 {
			envmap[parts[0]] = parts[1]
		}
	}

	for k, v := range options.EnvironVars {
		if len(v) == 0 {
			delete(envmap, k)
		} else {
			envmap[k] = v
		}
	}

	env := make([]string, 0, len(envmap))
	for k, v := range envmap {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	exe := exec.Command(args[0], args[1:]...)

	if err := syscall.Exec(exe.Path, exe.Args, env); err != nil {
		log.Printf("Unable to execute %s: %v\n", strings.Join(args, " "), err)
		return
	}

	// We will never reach this point, because the previous Exec will have replaced us
	// with the target program
}
