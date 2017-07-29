package main

type AptAction struct {
	*BaseAction
	Recommends bool
	Packages   []string
}

func (apt *AptAction) Run(context *YaibContext) {
	aptOptions := []string{"apt-get", "-y"}

	if !apt.Recommends {
		aptOptions = append(aptOptions, "--no-install-recommends")
	}

	aptOptions = append(aptOptions, "install")
	aptOptions = append(aptOptions, apt.Packages...)

	c := NewChrootCommand(context.rootdir, context.Architecture)
	c.AddEnv("DEBIAN_FRONTEND=noninteractive")

	c.Run("apt", "apt-get", "update")
	c.Run("apt", aptOptions...)
	c.Run("apt", "apt-get", "clean")
}
