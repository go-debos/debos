package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"strings"
)

type DebootstrapAction struct {
	suite      string
	mirror     string
	variant    string
	components []string
}

func (d *DebootstrapAction) RunSecondStage(context YaibContext) {

	q := NewQemuHelper(context)
	q.Setup()
	defer q.Cleanup()

	options := []string{context.rootdir,
		"/debootstrap/debootstrap",
		"--no-check-gpg",
		"--second-stage"}

	if d.components != nil {
		s := strings.Join(d.components, ",")
		options = append(options, fmt.Sprintf("--components=%s", s))
	}

	err := RunCommand("Debootstrap (stage 2)", "chroot", options...)

	if err != nil {
		log.Panic(err)
	}

}

func (d *DebootstrapAction) Run(context YaibContext) {
	options := []string{"--no-check-gpg",
		"--keyring=apertis-archive-keyring",
		"--merged-usr"}

	if d.components != nil {
		s := strings.Join(d.components, ",")
		options = append(options, fmt.Sprintf("--components=%s", s))
	}

	/* FIXME drop the hardcoded amd64 assumption" */
	foreign := context.Architecture != "amd64"

	if foreign {
		options = append(options, "--foreign")
		options = append(options, fmt.Sprintf("--arch=%s", context.Architecture))

	}

	if d.variant != "" {
		options = append(options, "--variant=minbase")
	}

	options = append(options, d.suite)
	options = append(options, context.rootdir)
	options = append(options, d.mirror)
	options = append(options, "/usr/share/debootstrap/scripts/unstable")

	err := RunCommand("Debootstrap", "debootstrap", options...)

	if err != nil {
		panic(err)
	}

	if foreign {
		d.RunSecondStage(context)
	}

	/* HACK */
	srclist, err := os.OpenFile(path.Join(context.rootdir, "etc/apt/sources.list"),
		os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		panic(err)
	}
	_, err = io.WriteString(srclist, fmt.Sprintf("deb %s %s %s\n",
		d.mirror,
		d.suite,
		strings.Join(d.components, " ")))
	if err != nil {
		panic(err)
	}
	srclist.Close()

	err = RunCommandInChroot(context, "apt clean", "/usr/bin/apt-get", "clean")
	if err != nil {
		panic(err)
	}
}

func NewDebootstrapAction(p map[string]interface{}) *DebootstrapAction {
	d := new(DebootstrapAction)
	d.suite = p["suite"].(string)
	d.mirror = p["mirror"].(string)
	if p["variant"] != nil {
		d.variant = p["variant"].(string)
	}

	for _, v := range p["components"].([]interface{}) {
		d.components = append(d.components, v.(string))
	}

	return d
}
