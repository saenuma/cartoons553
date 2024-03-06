package main

import (
	"time"
	"os/exec"
)


func main() {
	time.Sleep(4 * time.Hour)
	exec.Command("sudo", "shutdown", "-h", "now").Run()
}