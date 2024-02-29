package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	// Upload route
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/set_quality/", registerQuality)
	http.HandleFunc("/dl/", downloadHandler)
	http.HandleFunc("/dlv/", downloadVid)
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "yeah")
	})

	//Listen on port 8080
	err := http.ListenAndServe(":8089", nil)
	if err != nil {
		panic(err)
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	// Maximum upload of 10 MB files
	r.ParseMultipartForm(10000 << 20)

	file, handler, err := r.FormFile("file")
	if err != nil {
		fmt.Fprintf(w, "not_ok")
		fmt.Println(err)
		return
	}
	defer file.Close()

	rawFile, err := io.ReadAll(file)
	if err != nil {
		fmt.Fprintf(w, "not_ok")
		fmt.Println(err)
		return
	}
	os.MkdirAll("/tmp/ooldim_in/", 0777)
	err = os.WriteFile("/tmp/ooldim_in/"+handler.Filename, rawFile, 0777)
	if err != nil {
		fmt.Fprintf(w, "not_ok")
		fmt.Println(err)
		return
	}
	fmt.Fprintf(w, "ok")
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, r.FormValue("p"))
}

func downloadVid(w http.ResponseWriter, r *http.Request) {
	rawFIs, _ := os.ReadDir("/tmp/t1")
	toDlPath := filepath.Join("/tmp/t1/", rawFIs[0].Name())
	fmt.Println(toDlPath)
	http.ServeFile(w, r, toDlPath)
}

func registerQuality(w http.ResponseWriter, r *http.Request) {
	quality := r.FormValue("q")
	if quality != "" {
		os.WriteFile("/tmp/render_quality.txt", []byte(quality), 0777)
	}
	fmt.Fprintf(w, "ok")
}
