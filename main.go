package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/gookit/color"
	"github.com/saenuma/zazabul"
)

const VersionFormat = "20060102T150405MST"

func main() {
	if len(os.Args) < 2 {
		color.Red.Println("Expecting a command. Run with help subcommand to view help.")
		os.Exit(1)
	}

	rootPath, err := GetRootPath()
	if err != nil {
		color.Red.Println(err.Error())
		os.Exit(1)
	}

	switch os.Args[1] {
	case "--help", "help", "h":
		fmt.Printf(`cartoons553 creates a GCP VM, renders a blender project on it, downloads the renders and deletes the VM.

Note: Please try launching your choice server on Google Cloud's website before using it here.

Note: If you quit the program prematurely, go to https://console.cloud.google.com to delete
the created VM.

Working Directory: '%s'
All files must be placed in Working Directory (%s)

Supported Commands:

    prep    Prepares the render server for cartoons553. It would be already configured and kept in
            in a suspended state. It prints a serverConfigFile

    rnd       Renders a project with the config created above. It expects a blender file and a
            serverConfigFile (created in prep command above)

    del     Deletes a render server. It expects a serverConfigFile

`, rootPath, rootPath)

	case "prep":
		var tmpl = `// project is the Google Cloud Project name
// It can be created either from the Google Cloud Console or from the gcloud command
project:

// region is the Google Cloud Region name
// Specify the region you want to launch your Render server in.
region:


// zone is the Google Cloud Zone which must be derived from the region above.
// for instance a region could be 'us-central1' and the zone could be 'us-central1-a'
zone:


// machine_type is the type of machine configuration to use render your blender project.
// Get the machine_type from https://cloud.google.com/compute/all-pricing and its costs.
// It is not necessary it must be an e2 instance.
// If you find a render to be slow, use a bigger machine. Preferably a highcpu machine.
// At times you might need to apply for quota increase to use a bigger machine.
machine_type: e2-highcpu-4


// sak means service account key file.
// sak_file is a key gotten from https://console.cloud.google.com .
// It is necessary to connect to an instance.
// It must be placed in the path where this config is.
sak_file:

// the quality metric here specifies the render engine to use.
// if the quality is high it would use CYCLES render engine.
// if the quality is low it would use the EEVEE render engine.
quality: low

	`
		configFileName := "s" + time.Now().Format(VersionFormat) + ".zconf"
		confPath := filepath.Join(rootPath, configFileName)

		conf, err := zazabul.ParseConfig(tmpl)
		if err != nil {
			panic(err)
		}

		err = conf.Write(confPath)
		if err != nil {
			panic(err)
		}

		cmd := exec.Command("nano", confPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			color.Red.Println(err.Error())
			os.Exit(1)
		}

		doPrep(confPath)

	case "rnd":
		if len(os.Args) != 4 {
			color.Red.Println("The rnd command expects a blender file and a serverConfigFile")
			os.Exit(1)
		}

		blenderPath := filepath.Join(rootPath, os.Args[2])
		if !DoesPathExists(blenderPath) {
			color.Red.Printf("The file '%s' does not exist in '%s'", os.Args[2], rootPath)
			os.Exit(1)
		}

		serverConfigPath := filepath.Join(rootPath, os.Args[3])
		doRender(blenderPath, serverConfigPath)

	case "del":
		if len(os.Args) != 3 {
			color.Red.Println("The del command expects a serverConfigFile")
			os.Exit(1)
		}

		serverConfigPath := filepath.Join(rootPath, os.Args[2])
		doDelete(serverConfigPath)

	default:
		color.Red.Println("Unexpected command. Run the cli with --help to find out the supported commands.")
		os.Exit(1)
	}
}
