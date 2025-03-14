package main

var (
	HelpMessage = `cartoons553 helps in rendering a blender project on Google Cloud.

Note: Please try launching your choice server on Google Cloud's website before using it here.

Note: 100 - 200 CPUs is recommended for rendering blender projects.

Working Directory: '%s'
All files must be placed in Working Directory (%s)

Supported Commands:

    prep    Prepares the render server for cartoons553. It would be already configured and kept in
            in a suspended state. It prints a serverConfigFile

    rnd     Renders a project with the config created above. It expects a blender file and a
            serverConfigFile (created in prep command above)

    del     Deletes a render server. It expects a serverConfigFile

`
)

func hasUpdate() bool {
	return false
}
