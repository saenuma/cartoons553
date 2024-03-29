package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/radovskyb/watcher"
)

func main() {

	for {
		if DoesPathExists("/tmp/c553_in/") {
			fmt.Println("In path found.")
			break
		} else {
			fmt.Println("Trying to connect to in path")
			time.Sleep(10 * time.Second)
			continue
		}
	}

	// watch for new files
	w := watcher.New()

	go func() {
		for {
			select {
			case event := <-w.Event:
				if strings.HasSuffix(event.Path, ".blend") {
					go doRender(event.Path)
				}

			case err := <-w.Error:
				fmt.Println(err)
			case <-w.Closed:
				return
			}
		}
	}()

	if err := w.AddRecursive("/tmp/c553_in/"); err != nil {
		panic(err)
	}

	if err := w.Start(time.Millisecond * 100); err != nil {
		panic(err)
	}

}

func doRender(path string) {
	fmt.Println("found: " + path)
	raw, _ := os.ReadFile("/tmp/render_quality.txt")
	if string(raw) == "low" {
		exec.Command("blender", "-b", path, "-o", "/tmp/t1/", "-E", "BLENDER_EEVEE", "-F", "AVIJPEG", "-a").Run()
	} else {
		exec.Command("blender", "-b", path, "-o", "/tmp/t1/", "-E", "CYCLES", "-F", "AVIJPEG", "-a").Run()
	}
	time.Sleep(30 * time.Second)
	os.WriteFile("/tmp/c553_in/done.txt", []byte("done"), 0777)
}

func DoesPathExists(p string) bool {
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return false
	}
	return true
}
