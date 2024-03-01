package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gookit/color"
	"github.com/saenuma/zazabul"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
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

Note: If you quit the program prematurely, go to https://console.cloud.google.com to delete
the created VM.

Working Directory: '%s'

Supported Commands:

    init    Creates a config file describing your render server. Edit to your own requirements.
            Some of the values can be gotten from Google Cloud's documentation.

    r       Renders a project with the config created above. It expects a blender file and a
            launch file (created from 'init' above)
            All files must be placed in Working Directory (%s)

`, rootPath, rootPath)

	case "init":
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
machine_type: e2-highcpu-16


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
		writePath := filepath.Join(rootPath, configFileName)

		conf, err := zazabul.ParseConfig(tmpl)
		if err != nil {
			panic(err)
		}

		err = conf.Write(writePath)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Edit the file at '%s' before launching.\n", writePath)

	case "r":
		if len(os.Args) != 4 {
			color.Red.Println("The r command expects a blender file and the conf file")
			os.Exit(1)
		}
		blenderPath := filepath.Join(rootPath, os.Args[2])
		if !DoesPathExists(blenderPath) {
			color.Red.Printf("The file '%s' does not exist in '%s'", os.Args[2], rootPath)
			os.Exit(1)
		}

		serverConfigPath := filepath.Join(rootPath, os.Args[3])

		conf, err := zazabul.LoadConfigFile(serverConfigPath)
		if err != nil {
			panic(err)
		}

		for _, item := range conf.Items {
			if item.Value == "" {
				color.Red.Println("Every field in the launch file is compulsory.")
				os.Exit(1)
			}
		}

		credentialsFilePath := filepath.Join(rootPath, conf.Get("sak_file"))
		if !DoesPathExists(credentialsFilePath) {
			color.Red.Printf("The file '%s' does not exist in '%s'\n", conf.Get("sak_file"), rootPath)
			os.Exit(1)
		}

		instanceName := fmt.Sprintf("c553-%s", strings.ToLower(UntestedRandomString(4)))

		var startupScript = `
  	#! /bin/bash

sudo apt update
sudo apt upgrade
sudo apt install -y libx11-dev libxxf86vm-dev libxcursor-dev libxi-dev libxrandr-dev libxinerama-dev libegl-dev
sudo apt install -y libwayland-dev wayland-protocols libxkbcommon-dev libdbus-1-dev linux-libc-dev
sudo apt install -y libsm6
sudo snap install blender --classic

sudo mkdir -p /tmp/ooldim_in/
sudo rm -rf /tmp/t1/ # clean the output folder incase of reuse.

gcloud compute firewall-rules create ooldimrules --direction ingress \
 --source-ranges 0.0.0.0/0 --rules tcp:8089 --action allow

# download needed files
wget https://sae.ng/static/c553/ool_mover
wget https://sae.ng/static/c553/ool_mover.service
wget https://sae.ng/static/c553/ool_shutdown
wget https://sae.ng/static/c553/ool_shutdown.service
wget https://sae.ng/static/c553/ool_render
wget https://sae.ng/static/c553/ool_render.service

# put the files in place
sudo mkdir -p /opt/ooldim/
sudo cp ool_mover /opt/ooldim/ool_mover
sudo cp ool_shutdown /opt/ooldim/ool_shutdown
sudo cp ool_render /opt/ooldim/ool_render
sudo chmod +x /opt/ooldim/ool_mover
sudo chmod +x /opt/ooldim/ool_shutdown
sudo chmod +x /opt/ooldim/ool_render
sudo cp ool_mover.service /etc/systemd/system/ool_mover.service
sudo cp ool_shutdown.service /etc/systemd/system/ool_shutdown.service
sudo cp ool_render.service /etc/systemd/system/ool_render.service

# start the programs
sudo systemctl daemon-reload
sudo systemctl start ool_shutdown
sudo systemctl start ool_render
sudo systemctl start ool_mover
`

		ctx := context.Background()
		computeService, err := compute.NewService(ctx, option.WithCredentialsFile(credentialsFilePath),
			option.WithScopes(compute.ComputeScope))
		if err != nil {
			panic(err)
		}
		prefix := "https://www.googleapis.com/compute/v1/projects/" + conf.Get("project")

		image, err := computeService.Images.GetFromFamily("ubuntu-os-cloud", "ubuntu-minimal-2204-lts").Context(ctx).Do()
		if err != nil {
			panic(err)
		}
		imageURL := image.SelfLink

		instance := &compute.Instance{
			Name:        instanceName,
			Description: "ooldim instance",
			MachineType: prefix + "/zones/" + conf.Get("zone") + "/machineTypes/" + conf.Get("machine_type"),
			Disks: []*compute.AttachedDisk{
				{
					AutoDelete: true,
					Boot:       true,
					Type:       "PERSISTENT",

					InitializeParams: &compute.AttachedDiskInitializeParams{
						SourceImage: imageURL,
						DiskType:    prefix + "/zones/" + conf.Get("zone") + "/diskTypes/pd-ssd",
						DiskSizeGb:  10,
					},
				},
			},
			NetworkInterfaces: []*compute.NetworkInterface{
				{
					AccessConfigs: []*compute.AccessConfig{
						{
							Type: "ONE_TO_ONE_NAT",
							Name: "External NAT",
						},
					},
					Network: prefix + "/global/networks/default",
				},
			},
			ServiceAccounts: []*compute.ServiceAccount{
				{
					Email: "default",
					Scopes: []string{
						compute.DevstorageFullControlScope,
						compute.ComputeScope,
					},
				},
			},
			Metadata: &compute.Metadata{
				Items: []*compute.MetadataItems{
					{
						Key:   "startup-script",
						Value: &startupScript,
					},
				},
			},
		}

		op, err := computeService.Instances.Insert(conf.Get("project"), conf.Get("zone"), instance).Do()
		if err != nil {
			panic(err)
		}
		err = waitForOperationZone(conf.Get("project"), conf.Get("zone"), computeService, op)
		if err != nil {
			panic(err)
		}

		launchedInstance, err := computeService.Instances.Get(conf.Get("project"), conf.Get("zone"), instanceName).Context(ctx).Do()
		if err != nil {
			panic(err)
		}
		instanceIP := launchedInstance.NetworkInterfaces[0].AccessConfigs[0].NatIP

		fmt.Printf("Render Server Created. Name: %s, IP: %s\n", instanceName, instanceIP)

		for {
			_, err := http.Get("http://" + instanceIP + ":8089/ready")
			if err != nil {
				time.Sleep(10 * time.Second)
				continue
			}
			break
		}

		fmt.Println("Tested connection to your Render server.")

		// write the quality of the render
		resp, err := http.Get("http://" + instanceIP + ":8089/set_quality/?q=" + conf.Get("quality"))
		if err != nil {
			fmt.Println(err)
		}
		defer resp.Body.Close()
		body2, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println(err)
		}

		if resp.StatusCode != 200 {
			fmt.Println(string(body2))
		}

		// upload blender file to render
		rawBlenderFile, err := os.ReadFile(blenderPath)
		if err != nil {
			panic(err)
		}

		body := new(bytes.Buffer)
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", filepath.Base(blenderPath))
		if err != nil {
			panic(err)
		}
		part.Write(rawBlenderFile)
		err = writer.Close()
		if err != nil {
			panic(err)
		}
		req, err := http.NewRequest("POST", "http://"+instanceIP+":8089/upload", body)
		if err != nil {
			panic(err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		httpClient := &http.Client{}
		_, err = httpClient.Do(req)
		if err != nil {
			panic(err)
		}
		fmt.Println("Uploaded blend file and beginning render")

		startTime := time.Now()
		fmt.Println()
		for {
			err := downloadFile("http://"+instanceIP+":8089/dl/?p="+"/tmp/ooldim_in/done.txt",
				filepath.Join(rootPath, "done.txt"))
			if err == nil {
				break
			}

			time.Sleep(10 * time.Second)
			fmt.Printf("\rBeen rendering for: %s", time.Since(startTime).String())
		}
		fmt.Println("\nRendered now dowloading.")
		os.RemoveAll(filepath.Join(rootPath, "done.txt"))

		rootPath, _ := GetRootPath()
		dlPath := filepath.Join(rootPath, time.Now().Format(VersionFormat)+".avi")
		err = downloadFile("http://"+instanceIP+":8089/dlv/", dlPath)
		if err != nil {
			fmt.Println(err)
		}

		fmt.Printf("Output: %s\n", dlPath)

		// delete the server
		op, err = computeService.Instances.Delete(conf.Get("project"), conf.Get("zone"), instanceName).Context(ctx).Do()
		if err != nil {
			panic(err)
		}
		err = waitForOperationZone(conf.Get("project"), conf.Get("zone"), computeService, op)
		if err != nil {
			panic(err)
		}
		fmt.Println("All Done. Server Deleted.")

	default:
		color.Red.Println("Unexpected command. Run the cli with --help to find out the supported commands.")
		os.Exit(1)
	}
}

// func waitForOperationRegion(project, region string, service *compute.Service, op *compute.Operation) error {
// 	ctx := context.Background()
// 	for {
// 		result, err := service.RegionOperations.Get(project, region, op.Name).Context(ctx).Do()
// 		if err != nil {
// 			return fmt.Errorf("Failed retriving operation status: %s", err)
// 		}

// 		if result.Status == "DONE" {
// 			if result.Error != nil {
// 				var errors []string
// 				for _, e := range result.Error.Errors {
// 					errors = append(errors, e.Message)
// 				}
// 				return fmt.Errorf("Operation failed with error(s): %s", strings.Join(errors, ", "))
// 			}
// 			break
// 		}
// 		time.Sleep(time.Second)
// 	}
// 	return nil
// }

func waitForOperationZone(project, zone string, service *compute.Service, op *compute.Operation) error {
	ctx := context.Background()
	for {
		result, err := service.ZoneOperations.Get(project, zone, op.Name).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("failed retriving operation status: %s", err)
		}

		if result.Status == "DONE" {
			if result.Error != nil {
				var errors []string
				for _, e := range result.Error.Errors {
					errors = append(errors, e.Message)
				}
				return fmt.Errorf("operation failed with error(s): %s", strings.Join(errors, ", "))
			}
			break
		}
		time.Sleep(time.Second)
	}
	return nil
}
