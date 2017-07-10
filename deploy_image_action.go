package main

type DeployImageAction struct {
	*BaseAction
}

func (overlay *DeployImageAction) Run(context *YaibContext) {
	/* Copying files is actually silly hard, one has to keep permissions, ACL's
	 * extended attribute, misc, other. Leave it to cp...
	 */
	RunCommand("Deploy to image", "cp", "-va", context.rootdir+"/.", context.imageMntDir)
	context.rootdir = context.imageMntDir
}
