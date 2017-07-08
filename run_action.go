package main

type RunAction struct {
  chroot bool;
  script string;
}

func NewRunAction(p map[string]interface{}) *RunAction {
  run := new(RunAction)
  run.chroot = p["chroot"].(bool)
  run.script = p["script"].(string)
  return run
}

func (run *RunAction) Run(context YaibContext) {
  err := RunCommandInChroot(context, run.script, "sh", "-c", run.script)
  if err != nil {
    panic(err)
  }
}
