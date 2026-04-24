package debos

import (
	"errors"
	"fmt"
	"os"
	"path"
)

const debianPolicyHelper = "/usr/sbin/policy-rc.d"

/*
ServiceHelper is used to manage services.
Currently supports only debian-based family.
*/

type ServiceHelper struct {
	Rootdir string
}

type ServicesManager interface {
	Allow() error
	Deny() error
}

/*
Allow() allows to start/stop services on OS level.
*/
func (s *ServiceHelper) Allow() error {
	helperFile := path.Join(s.Rootdir, debianPolicyHelper)

	if _, err := os.Stat(helperFile); os.IsNotExist(err) {
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", helperFile, err)
	}
	if err := os.Remove(helperFile); err != nil {
		return fmt.Errorf("remove %s: %w", helperFile, err)
	}
	return nil
}

/*
Deny() prohibits to start/stop services on OS level.
*/
func (s *ServiceHelper) Deny() (err error) {
	helperFile := path.Join(s.Rootdir, debianPolicyHelper)
	helper := []byte(`#!/bin/sh

exit 101
`)

	if _, err := os.Stat(helperFile); err == nil {
		return fmt.Errorf("policy helper file '%s' exists already", debianPolicyHelper)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", helperFile, err)
	}

	if _, err := os.Stat(path.Dir(helperFile)); os.IsNotExist(err) {
		// do not try to do something if ".../usr/sbin" does not exist
		return nil
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", path.Dir(helperFile), err)
	}

	pf, err := os.Create(helperFile)
	if err != nil {
		return fmt.Errorf("create %s: %w", helperFile, err)
	}

	defer func() {
		if closeErr := pf.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close %s: %w", helperFile, closeErr))
		}
	}()

	if _, err := pf.Write(helper); err != nil {
		return fmt.Errorf("write %s: %w", helperFile, err)
	}

	if err := pf.Chmod(0755); err != nil {
		return fmt.Errorf("chmod %s: %w", helperFile, err)
	}

	return nil
}
