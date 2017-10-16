package debos

import (
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
	}
	if err := os.Remove(helperFile); err != nil {
		return err
	}
	return nil
}

/*
Deny() prohibits to start/stop services on OS level.
*/
func (s *ServiceHelper) Deny() error {

	helperFile := path.Join(s.Rootdir, debianPolicyHelper)
	var helper = []byte(`#!/bin/sh

exit 101
`)

	if _, err := os.Stat(helperFile); os.IsExist(err) {
		return fmt.Errorf("Policy helper file '%s' exists already", debianPolicyHelper)
	}
	if _, err := os.Stat(path.Dir(helperFile)); os.IsNotExist(err) {
		// do not try to do something if ".../usr/sbin" is not exists
		return nil
	}
	pf, err := os.Create(helperFile)
	if err != nil {
		return err
	}
	defer pf.Close()

	if _, err := pf.Write(helper); err != nil {
		return err
	}

	if err := pf.Chmod(0755); err != nil {
		return err
	}

	return nil
}
