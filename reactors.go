package main

import (
	"bytes"
	"context"
	"fmt"
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

func doPrep(serverConfigPath string) {
	rootPath, _ := GetRootPath()

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

	instanceName := fmt.Sprintf("c553-%s", strings.ToLower(UntestedRandomString(10)))

	var startupScript = `
	#! /bin/bash

sudo apt update
sudo apt upgrade
sudo apt install -y libx11-dev libxxf86vm-dev libxcursor-dev libxi-dev libxrandr-dev libxinerama-dev libegl-dev
sudo apt install -y libwayland-dev wayland-protocols libxkbcommon-dev libdbus-1-dev linux-libc-dev
sudo apt install -y libsm6
sudo snap install blender --classic

# install ops-agent
curl -sSO https://dl.google.com/cloudagents/add-google-cloud-ops-agent-repo.sh
sudo bash add-google-cloud-ops-agent-repo.sh --also-install

sudo mkdir -p /tmp/c553_in/
sudo rm -rf /tmp/t1/ # clean the output folder incase of reuse.

gcloud compute firewall-rules create c553rules --direction ingress \
--source-ranges 0.0.0.0/0 --rules tcp:8089 --action allow

# download needed files
wget https://sae.ng/static/c553/c553_mover
wget https://sae.ng/static/c553/c553_mover.service
wget https://sae.ng/static/c553/c553_shutdown
wget https://sae.ng/static/c553/c553_shutdown.service
wget https://sae.ng/static/c553/c553_render
wget https://sae.ng/static/c553/c553_render.service

# put the files in place
sudo mkdir -p /opt/cartoons553/
sudo cp c553_mover /opt/cartoons553/c553_mover
sudo cp c553_shutdown /opt/cartoons553/c553_shutdown
sudo cp c553_render /opt/cartoons553/c553_render
sudo chmod +x /opt/cartoons553/c553_mover
sudo chmod +x /opt/cartoons553/c553_shutdown
sudo chmod +x /opt/cartoons553/c553_render
sudo cp c553_mover.service /etc/systemd/system/c553_mover.service
sudo cp c553_shutdown.service /etc/systemd/system/c553_shutdown.service
sudo cp c553_render.service /etc/systemd/system/c553_render.service

# start the programs
sudo systemctl daemon-reload
sudo systemctl start c553_shutdown
sudo systemctl start c553_render
sudo systemctl start c553_mover
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

	fmt.Println("Started render server")

	launchedInstance, err := computeService.Instances.Get(conf.Get("project"), conf.Get("zone"), instanceName).Context(ctx).Do()
	if err != nil {
		panic(err)
	}
	instanceIP := launchedInstance.NetworkInterfaces[0].AccessConfigs[0].NatIP

	for {
		_, err := http.Get("http://" + instanceIP + ":8089/ready")
		if err != nil {
			time.Sleep(10 * time.Second)
			continue
		}
		break
	}

	// stops the server
	op, err = computeService.Instances.Stop(conf.Get("project"), conf.Get("zone"), instanceName).Context(ctx).Do()
	if err != nil {
		panic(err)
	}
	err = waitForOperationZone(conf.Get("project"), conf.Get("zone"), computeService, op)
	if err != nil {
		panic(err)
	}

	fmt.Println("Finished configuring render server.")
	raw, _ := os.ReadFile(serverConfigPath)
	newRaw := string(raw) + "\n\n" + "name: " + instanceName
	os.WriteFile(serverConfigPath, []byte(newRaw), 0777)
	fmt.Println("Server config path: ", serverConfigPath)
}

func doRender(blenderPath, serverConfigPath string) {
	rootPath, _ := GetRootPath()

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

	ctx := context.Background()
	computeService, err := compute.NewService(ctx, option.WithCredentialsFile(credentialsFilePath),
		option.WithScopes(compute.ComputeScope))
	if err != nil {
		panic(err)
	}
	instanceName := conf.Get("name")

	// starts the server
	op, err := computeService.Instances.Start(conf.Get("project"), conf.Get("zone"), instanceName).Context(ctx).Do()
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

	for {
		_, err := http.Get("http://" + instanceIP + ":8089/ready")
		if err != nil {
			time.Sleep(10 * time.Second)
			continue
		}
		break
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
	fmt.Printf("Click %s to download a preview of your render while it renders.\n",
		"http://"+instanceIP+":8089/dlv/")

	for {
		err := downloadFile("http://"+instanceIP+":8089/dl/?p="+"/tmp/c553_in/done.txt",
			filepath.Join(rootPath, "done.txt"))
		if err == nil {
			break
		}

		time.Sleep(10 * time.Second)
		fmt.Printf("\rBeen rendering for: %s  ", time.Since(startTime).Round(time.Second).String())
	}

	fmt.Println("\nRendered now dowloading.")
	os.RemoveAll(filepath.Join(rootPath, "done.txt"))

	dlPath := filepath.Join(rootPath, time.Now().Format(VersionFormat)+".mpeg")
	err = downloadFile("http://"+instanceIP+":8089/dlv/", dlPath)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Printf("Output: %s\n", dlPath)

	// stops the server
	op, err = computeService.Instances.Stop(conf.Get("project"), conf.Get("zone"), instanceName).Context(ctx).Do()
	if err != nil {
		panic(err)
	}
	err = waitForOperationZone(conf.Get("project"), conf.Get("zone"), computeService, op)
	if err != nil {
		panic(err)
	}

	fmt.Println("Server stopped.")
}

func doDelete(serverConfigPath string) {
	rootPath, _ := GetRootPath()

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

	ctx := context.Background()
	computeService, err := compute.NewService(ctx, option.WithCredentialsFile(credentialsFilePath),
		option.WithScopes(compute.ComputeScope))
	if err != nil {
		panic(err)
	}
	instanceName := conf.Get("name")

	// delete the server
	op, err := computeService.Instances.Delete(conf.Get("project"), conf.Get("zone"), instanceName).Context(ctx).Do()
	if err != nil {
		panic(err)
	}
	err = waitForOperationZone(conf.Get("project"), conf.Get("zone"), computeService, op)
	if err != nil {
		panic(err)
	}

	os.RemoveAll(serverConfigPath)
	fmt.Println("All Done. Server Deleted.")
}
