package main

import (
	"os/exec"
	"time"
)

func main() {
	time.Sleep(1 * time.Hour)
	exec.Command("sudo", "shutdown", "-h", "now").Run()
}
