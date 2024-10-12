package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"time"

	"cloud.google.com/go/storage"
)

// BlobFile implements multipart.File interface
type BlobFile struct {
	*bytes.Reader
}

// ClientUploader manages uploads to GCS
type ClientUploader struct {
	cl         *storage.Client
	ProjectID  string
	BucketName string
	UploadPath string
}

func NewClientUploader() (*ClientUploader, error) {
	projectID := os.Getenv("GCP_PROJECT_ID")
	bucketName := os.Getenv("GCS_BUCKET_NAME")
	log.Printf("prj and buck are %v and %v", projectID, bucketName)

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage.NewClient: %w", err)
	}

	return &ClientUploader{
		cl:         client,
		ProjectID:  projectID,
		BucketName: bucketName,
		UploadPath: "",
	}, nil
}

// UploadFile uploads an object to GCS
func (c *ClientUploader) UploadFile(file multipart.File, object string) error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	log.Printf("Uploading file to bucket...")

	wc := c.cl.Bucket(c.BucketName).Object(object).NewWriter(ctx)
	if _, err := io.Copy(wc, file); err != nil {
		return fmt.Errorf("io.Copy: %v", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("Writer.Close: %v", err)
	}

	log.Printf("File %v uploaded to %v\n", object, c.BucketName)
	return nil
}

// GetFile downloads an object from the bucket and writes it to a destination file
func (c *ClientUploader) GetFile(object, destFileName string) error {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	// Open the destination file for writing
	destFilePath := "./static/images/" + destFileName
	f, err := os.Create(destFilePath)
	if err != nil {
		return fmt.Errorf("os.Create: %v", err)
	}
	defer f.Close()

	// Create a new reader for the object in the bucket
	rc, err := c.cl.Bucket(c.BucketName).Object(c.UploadPath + object).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("Object(%q).NewReader: %v", object, err)
	}
	defer rc.Close()

	// Copy the object data to the local file
	if _, err := io.Copy(f, rc); err != nil {
		return fmt.Errorf("io.Copy: %v", err)
	}

	fmt.Printf("File %v downloaded to %v\n", object, destFileName)
	return nil
}
