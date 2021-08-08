package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
)

const bucketName = "cookndx-dev-testing-2021-02"

var Photos BucketStorage

//// BEGIN Abstraction over storage.Writer
type PhotoWriter interface {
	Write(p []byte) (n int, err error)
	Close() error
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
	http.HandleFunc("/", handler)

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
		log.Fatal(fmt.Errorf("Cannot create Cloud Storage client; %v+", err))
	}

	bucket := store.Bucket(bucketName)
	Photos = &GCPBucket{
		ctx:    &ctx,
		bucket: bucket,
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	name := os.Getenv("NAME")
	if name == "" {
		name = "World"
	}

	// Write the content
	writer := Photos.NewWriter("ignored")
	(*writer).SetContentType("text/plain")

	if _, err := fmt.Fprintf(*writer, "Hello %s!\n", name); err != nil {
		log.Fatalf("Cannot write object; %v+", err)
	}
	// Close the writer to finish out
	// if err := writer.Close(); err != nil {
	// 	log.Fatalf("Cannot close object; %v+", err)
	// }
	fmt.Fprintf(w, "Hello %s!\n", name)
}
