package main

import (
	"net/http"
	"fmt"
	"io"
	"os"
)


func main() {
	// Upload route
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/dl/", downloadHandler)
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
	err = os.WriteFile("/tmp/ooldim_in/" + handler.Filename, rawFile, 0777)
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