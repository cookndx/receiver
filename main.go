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

//// BEGIN Abstraction over storage.Writer
type PhotoWriter interface {
	io.WriteCloser
	SetContentType(contentType string)
}

type ObjectWriter struct {
	ContentType   string
	wrappedWriter *storage.Writer
}

func (o *ObjectWriter) Write(p []byte) (n int, err error) {
	return o.wrappedWriter.Write(p)
}

func (o *ObjectWriter) Close() error {
	return o.wrappedWriter.Close()
}

func (o *ObjectWriter) SetContentType(contentType string) {
	o.ContentType = contentType
}

//// END Abstraction over storage.Writer

//// BEGIN Abstraction over storage.BucketHandle
type BucketStorage interface {
	NewWriter(objectKey string) *PhotoWriter
}

type GCPBucket struct {
	ctx    *context.Context
	bucket *storage.BucketHandle
}

func (b *GCPBucket) NewWriter(objectKey string) *PhotoWriter {
	objectPath := uuid.New().String()
	obj := b.bucket.Object(objectPath)

	// Write the content
	var writer PhotoWriter
	writer = &ObjectWriter{
		wrappedWriter: obj.NewWriter(*b.ctx),
	}
	return &writer
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

	// Write the content
	writer := photos.NewWriter("ignored")
	(*writer).SetContentType("text/plain")

	if _, err := fmt.Fprintf(*writer, "Hello %s!\n", name); err != nil {
		log.Fatalf("Cannot write object; %+v", err)
	}
	//Close the writer to finish out
	if err := (*writer).Close(); err != nil {
		log.Fatalf("Cannot close object; %+v", err)
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
	log.Printf("Uploaded File: %+v", handler.Filename)
	log.Printf("File Size: %+v", handler.Size)
	log.Printf("MIME Header: %+v", handler.Header)

	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Cannot read data; %+v", err)
		return
	}

	// Write the content
	writer := photos.NewWriter("ignored parameter")
	if _, err := (*writer).Write(fileBytes); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Cannot write object; %+v", err)
		return
	}

	//Close the writer to finish out
	if err := (*writer).Close(); err != nil {
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
