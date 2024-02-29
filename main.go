package main

import (
	"context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
	"os"
	"github.com/gookit/color"
	"github.com/bankole7782/zazabul"
	"fmt"
	"time"
	"strings"
	"path/filepath"
	// "io"
	"bytes"
	"os/exec"
	"net/http"
	"mime/multipart"
)


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
    fmt.Printf(`Ooldim creates a GCP VM, renders a blender project on it, downloads the renders and deletes the VM.

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
		var	tmpl = `// project is the Google Cloud Project name
// It can be created either from the Google Cloud Console or from the gcloud command
project:

// region is the Google Cloud Region name
// Specify the region you want to launch your flaarum server in.
region:


// zone is the Google Cloud Zone which must be derived from the region above.
// for instance a region could be 'us-central1' and the zone could be 'us-central1-a'
zone:


// machine_type is the type of machine configuration to use to launch your flaarum server.
// Get the machine_type from https://cloud.google.com/compute/all-pricing and its costs.
// It is not necessary it must be an e2 instance.
// If you find a render to be slow, use a bigger machine. Preferably a highcpu machine.
// At times you might need to apply for quota increase to use a bigger machine.
machine_type: e2-highcpu-16


// sak means service account key file.
// sak_file is a key gotten from https://console.cloud.google.com .
// It is necessary to connect to an instance.
// It must be placed in the path gotten from running the command 'ooldim pwd'
sak_file:


	`
		configFileName := "s" + time.Now().Format("20060102") + ".zconf"
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
  	if ! DoesPathExists(blenderPath) {
  		color.Red.Printf("The file '%s' does not exist in '%s'", os.Args[2], rootPath)
  		os.Exit(1)
  	}

  	serverConfigPath := filepath.Join(rootPath, os.Args[3])

  	conf, err := zazabul.LoadConfigFile(serverConfigPath)
  	if err != nil {
  		panic(err)
  		os.Exit(1)
  	}

  	for _, item := range conf.Items {
  		if item.Value == "" {
  			color.Red.Println("Every field in the launch file is compulsory.")
  			os.Exit(1)
  		}
  	}

  	credentialsFilePath := filepath.Join(rootPath, conf.Get("sak_file"))
  	if ! DoesPathExists(credentialsFilePath) {
  		color.Red.Printf("The file '%s' does not exist in '%s'\n", conf.Get("sak_file"), rootPath)
  		os.Exit(1)
  	}

  	instanceName := fmt.Sprintf("ooldim-%s", strings.ToLower(UntestedRandomString(4)))

  	var startupScript = `
  	#! /bin/bash

sudo apt update
sudo apt upgrade
sudo apt install -y libglu1 libxi6 libgconf-2-4 libfontconfig1 libxrender1
sudo snap install blender --classic

sudo mkdir -p /tmp/ooldim_in/
sudo rm -rf /tmp/t1/ # clean the output folder incase of reuse.

gcloud compute firewall-rules create ooldimrules --direction ingress \
 --source-ranges 0.0.0.0/0 --rules tcp:8089 --action allow

# download needed files
wget https://saenuma.com/static/ool_mover
wget https://saenuma.com/static/ool_mover.service
wget https://saenuma.com/static/ool_shutdown
wget https://saenuma.com/static/ool_shutdown.service
wget https://saenuma.com/static/ool_render
wget https://saenuma.com/static/ool_render.service

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

  	data, err := os.ReadFile(credentialsFilePath)
		if err != nil {
			panic(err)
		}
		creds, err := google.CredentialsFromJSON(ctx, data, compute.ComputeScope)
		if err != nil {
			panic(err)
		}

		client := oauth2.NewClient(ctx, creds.TokenSource)

		computeService, err := compute.New(client)
		if err != nil {
			panic(err)
		}
		prefix := "https://www.googleapis.com/compute/v1/projects/" + conf.Get("project")

		image, err := computeService.Images.GetFromFamily("ubuntu-os-cloud", "ubuntu-minimal-2004-lts").Context(ctx).Do()
		if err != nil {
			panic(err)
		}
		imageURL := image.SelfLink

		instance := &compute.Instance{
			Name: instanceName,
			Description: "ooldim instance",
			MachineType: prefix + "/zones/" + conf.Get("zone") + "/machineTypes/" + conf.Get("machine_type"),
			Disks: []*compute.AttachedDisk{
				{
					AutoDelete: true,
					Boot:       true,
					Type:       "PERSISTENT",

					InitializeParams: &compute.AttachedDiskInitializeParams{
						SourceImage: imageURL,
						DiskType: prefix + "/zones/" + conf.Get("zone") + "/diskTypes/pd-ssd",
						DiskSizeGb: 10,
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
			Metadata: &compute.Metadata {
				Items: []*compute.MetadataItems {
					{
						Key: "startup-script",
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

		fmt.Println("Ooldim Server Created. Name: %s, IP: %s", instanceName, instanceIP)

		for {
			_, err := http.Get("http://" + instanceIP + ":8089/ready")
			if err != nil {
				time.Sleep(10 * time.Second)
				continue
			}
			break
		}

		fmt.Println("Tested connection to your Ooldim server.")

		rawBlenderFile, err := os.ReadFile(blenderPath)
		if err != nil {
			panic(err)
		}

		// begin file upload
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
		req, err := http.NewRequest("POST", "http://" + instanceIP + ":8089/upload", body)
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
		outPath := getRenderPath(os.Args[2])

		imageIndex := 1
		for {
			downloadImage(instanceIP, imageIndex, outPath)
			imageIndex += 1

			err := downloadFile("http://" + instanceIP + ":8089/dl/?p=" + "/tmp/ooldim_in/done.txt",
				filepath.Join(rootPath, "done.txt"))
			if err == nil {
				break
			}
		}
		fmt.Println("downloaded render output.")
		os.RemoveAll(filepath.Join(rootPath, "done.txt"))

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


func downloadImage(instanceIP string, number int, outPath string) {
	filename := fmt.Sprintf("%04d", number) + ".png"
	for {
		err := downloadFile("http://" + instanceIP + ":8089/dl/?p=" + "/tmp/t1/" + filename,
			filepath.Join(outPath, filename))
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		break
	}

	if number == 1 {
		exec.Command("xdg-open", outPath).Run()
	}
}


func getRenderPath(filename string) string {
	rootPath, _ := GetRootPath()
	added := 1
	for {
		f := filepath.Join(rootPath, fmt.Sprintf("%s_%d", filename, added))
		if DoesPathExists(f) {
			added += 1
		} else {
			os.MkdirAll(f, 0777)
			return f
		}
	}
}


func waitForOperationRegion(project, region string, service *compute.Service, op *compute.Operation) error {
	ctx := context.Background()
	for {
		result, err := service.RegionOperations.Get(project, region, op.Name).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("Failed retriving operation status: %s", err)
		}

		if result.Status == "DONE" {
			if result.Error != nil {
				var errors []string
				for _, e := range result.Error.Errors {
					errors = append(errors, e.Message)
				}
				return fmt.Errorf("Operation failed with error(s): %s", strings.Join(errors, ", "))
			}
			break
		}
		time.Sleep(time.Second)
	}
	return nil
}


func waitForOperationZone(project, zone string, service *compute.Service, op *compute.Operation) error {
	ctx := context.Background()
	for {
		result, err := service.ZoneOperations.Get(project, zone, op.Name).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("Failed retriving operation status: %s", err)
		}

		if result.Status == "DONE" {
			if result.Error != nil {
				var errors []string
				for _, e := range result.Error.Errors {
					errors = append(errors, e.Message)
				}
				return fmt.Errorf("Operation failed with error(s): %s", strings.Join(errors, ", "))
			}
			break
		}
		time.Sleep(time.Second)
	}
	return nil
}
