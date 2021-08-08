package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
)

const bucketName = "cookndx-dev-testing-2021-02"

var photos BucketStorage

//// BEGIN Abstraction over storage.BucketHandle
type BucketStorage interface {
	NewWriter(objectKey string) io.WriteCloser
}

type GCPBucket struct {
	ctx    *context.Context
	bucket *storage.BucketHandle
}

func (b *GCPBucket) NewWriter(objectKey string) io.WriteCloser {
	objectPath := uuid.New().String()
	obj := b.bucket.Object(objectPath)
	return obj.NewWriter(*b.ctx)
}

//// END Abstraction over storage.BucketHandle

func main() {
	log.Print("starting server...")
	http.HandleFunc("/", helloHandler)
	http.HandleFunc("/upload", uploadHandler)

	initStorage()

	// Determine port for HTTP service.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("defaulting to port %s", port)
	}

	// Start HTTP server.
	log.Printf("listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func initStorage() {
	// Connect to the Cloud Storage bucket
	ctx := context.Background()
	store, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatal(fmt.Errorf("Cannot create Cloud Storage client; %+v", err))
	}

	bucket := store.Bucket(bucketName)
	photos = &GCPBucket{
		ctx:    &ctx,
		bucket: bucket,
	}
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	name := os.Getenv("NAME")
	if name == "" {
		name = "World"
	}

	fmt.Fprintf(w, "Hello %s!\n", name)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		processUpload(w, r)
	} else {
		serveForm(w, r)
	}
}

func processUpload(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(2 << 23) // 8 MB
	file, handler, err := r.FormFile("sourceFile")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Printf("Cannot find FormFile; %+v", err)
		return
	}
	defer file.Close()
	log.Printf("File uploaded. %+v", map[string]interface{}{
		"size":                handler.Size,
		"content_disposition": handler.Header["Content-Disposition"],
		"content_type":        handler.Header["Content-Type"],
	})

	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Cannot read data; %+v", err)
		return
	}

	// Write the content
	writer := photos.NewWriter("ignored parameter")
	if _, err := writer.Write(fileBytes); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Cannot write object; %+v", err)
		return
	}

	//Close the writer to finish out
	if err := writer.Close(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Cannot close object; %+v", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "Successfully Uploaded File\n")
}

func serveForm(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(uploadForm))
}

const uploadForm = `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <meta http-equiv="X-UA-Compatible" content="ie=edge" />
    <title>Document</title>
  </head>
  <body>
    <form
      enctype="multipart/form-data"
      action="http://localhost:8080/upload"
      method="post">
      <input type="file" name="sourceFile" />
      <input type="submit" value="upload" />
    </form>
  </body>
</html>`
