package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	HelpMessage = `cartoons553 helps in rendering a blender project on Google Cloud.

Note: Please try launching your choice server on Google Cloud's website before using it here.

Note: 100 - 200 CPUs is recommended for rendering blender projects.

Working Directory: '%s'
All files must be placed in Working Directory (%s)

Supported Commands:

    init    Creates a serverConfigFile for editing. Fill it to your own requirements.

    prep    Prepares the render server for cartoons553 described in the serverConfigFile. 
            It would be already configured and kept in a suspended state. 
						It expects a serverConfigFile gotten from above.

    rnd     Renders a project with the config created above. It expects a blender file and a
            serverConfigFile (created in prep command above)

    del     Deletes a render server. It expects a serverConfigFile

`
)

func hasUpdate() bool {
	outPath := filepath.Join(os.TempDir(), "c553.txt")
	err := downloadFile(UpdateURLCheck, outPath)
	if err != nil {
		fmt.Println(err)
		return false
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		fmt.Println(err)
		return false
	}

	if strings.TrimSpace(string(raw)) != AppVersion {
		return true
	}

	return false
}
