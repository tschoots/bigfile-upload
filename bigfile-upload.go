package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	//"golang.org/x/net/websocket"
	"fmt"
	"io"
	//"io/ioutil"
	"os"
	"strconv"
	//"net/http/httputil"
	"strings"
)

var m sync.Mutex

// template struct can be used to load template pages
type templateHandler struct {
	once     sync.Once
	filename string
	templ    *template.Template
}

// ServeHTTP handles the HTTP request.
func (t *templateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.once.Do(func() {
		t.templ = template.Must(template.ParseFiles(filepath.Join("templates", t.filename)))
	})
	t.templ.Execute(w, r)
}

type resumeFileInfo struct {
	FilePath string
	Size     int
}

type uploadHandler struct {
	UploadedFiles map[int]resumeFileInfo
	DownloadDir   string
	TargetDir     string
}

func (u *uploadHandler) concatDownloadedFiles(fileName string) {
	fmt.Printf("concat files : %d\n", len(u.UploadedFiles))

	dir, _ := filepath.Abs(u.TargetDir)
	filename := filepath.Join(dir, fileName)

	
	outFile, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR , 0666)
	defer outFile.Close()
	if err != nil {
		fmt.Printf("ERROR creating file %s:\n%s\n\n", fileName, err)
		return
	}

	for i := 1; i < len(u.UploadedFiles)+1; i++ {
		inputFile, err := os.Open(u.UploadedFiles[i].FilePath)
		defer inputFile.Close()
		if err != nil {
			fmt.Printf("ERROR opening file %s:\n%s\n\n", u.UploadedFiles[i], err)
			return
		}

		if _, err := io.Copy(outFile, inputFile); err != nil {
			fmt.Printf("ERROR copying file %s:\n%s\n\n", u.UploadedFiles[i].FilePath, err)
			return
		}
		os.Remove(u.UploadedFiles[i].FilePath)
	}
	u.UploadedFiles = map[int]resumeFileInfo{}

}

func (u *uploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("UploadHandler")

	r.ParseForm()
	for k, v := range r.Form {
		fmt.Printf("key : %s\nvalue: %s\n\n", k, v)
	}
	fmt.Printf("method %s\n", r.Method)

	// get the packet nr
	nr, err := strconv.Atoi(r.FormValue("resumableChunkNumber"))
	if err != nil {
		fmt.Printf("ERROR strconv for resumableChunkNumber went wrong : \n%s\n\n", err)
	}
	packetSize, err := strconv.Atoi(r.FormValue("resumableCurrentChunkSize"))
	if err != nil {
		fmt.Printf("ERROR strconv for resumableCurrentChunkSize went wrong : \n%s\n\n", err)
	}

	// check if it's a get or a post
	if strings.Compare(r.Method, "GET") == 0 {
		if _, ok := u.UploadedFiles[nr]; !ok {
			w.WriteHeader(501)
			return
		}
		if u.UploadedFiles[nr].Size != packetSize {
			w.WriteHeader(501)
			return
		}
		w.WriteHeader(200)
	} else {
		//this is a post
		fmt.Println("--POST--")
		dir, _ := filepath.Abs(u.DownloadDir)
		filename := fmt.Sprintf("%s/%s.%s_%s", dir, r.FormValue("resumableFilename"), r.FormValue("resumableTotalChunks"), r.FormValue("resumableChunkNumber"))
		file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
		defer file.Close()
		if err != nil {
			fmt.Printf("ERROR create file : %s\n%s\n\n", filename, err)
		}

		defer r.Body.Close()
		_, err = io.Copy(file, r.Body)
		if err != nil {
			//fmt.Printf("ERROR io.copy of %s\n%s\n\n", filename, err)
			fmt.Printf("ERROR ioutil.readall of %s\n\n", err)
		}
		fmt.Printf("content length %d\n\n", r.ContentLength)

		total, err := strconv.Atoi(r.FormValue("resumableTotalChunks"))
		if err != nil {
			fmt.Printf("ERROR strconv for resumableTotalChunks went wrong : \n%s\n\n", err)
		}
		w.WriteHeader(200)
		m.Lock()
		u.UploadedFiles[nr] = resumeFileInfo{FilePath: filename, Size: int(r.ContentLength)}
		m.Unlock()
		if len(u.UploadedFiles) == total {
			u.concatDownloadedFiles(r.FormValue("resumableFilename"))
		}
	}
}

func main() {
	var port = flag.String("port", ":8080", "the port the server will be listening on")
	flag.Parse()

	http.Handle("/", &templateHandler{filename: "index.html"})
	http.Handle("/upload", &uploadHandler{DownloadDir: "downloads", TargetDir: "downloads", UploadedFiles: make(map[int]resumeFileInfo)})
	http.HandleFunc("/resumable.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("templates", "resumable.js"))
	})

	// Start the web server
	log.Println("Starting web server on", *port)
	if err := http.ListenAndServe(*port, nil); err != nil {
		log.Fatal("listenAndServe:", err)
	}
}
