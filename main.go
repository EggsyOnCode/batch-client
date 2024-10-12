package main

import (
	"fmt"
	"log"
	"net/http"
	"scalabale-img-processor/kafka"
	"scalabale-img-processor/storage"
	"time"
)

const (
	MAX_UPLOAD_SIZE = 5 * 1024 * 1024 // 5 MB
	MAX_FILES       = 10              // Maximum number of files

)

func main() {
	// Initialize the Kafka client

	kafkaClient, err := kafka.NewKafkaClient()
	if err != nil {
		log.Fatalf("Error initializing Kafka client: %v", err)
	}
	defer kafkaClient.Close()

	// Initialize the GCS client
	storageClient, err := storage.NewClientUploader()
	if err != nil {
		log.Fatalf("Error initializing GCS client: %v", err)
	}

	// Channel for receiving processed image URLs
	responseChan := make(chan kafka.KafkaMessageResponse)

	// Start listening for Kafka responses in a separate goroutine
	go kafkaClient.ListenAndRespond(responseChan, storageClient)

	// Serve static files, including index.html
	http.Handle("/", http.FileServer(http.Dir("./static")))

	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		// Check if the request method is POST
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		// Parse the multipart form
		err := r.ParseMultipartForm(10 << 20) // Limit to 10 MB per file
		if err != nil {
			http.Error(w, fmt.Sprintf("Error parsing multipart form: %v", err), http.StatusBadRequest)
			return
		}

		files := r.MultipartForm.File["images"]
		if files == nil {
			http.Error(w, "No files uploaded", http.StatusBadRequest)
			return
		}

		// Process each file
		for _, fileHeader := range files {
			log.Printf("Uploaded file: %s", fileHeader.Filename) // Log file names
			file, err := fileHeader.Open()
			if err != nil {
				http.Error(w, "Error opening file", http.StatusInternalServerError)
				return
			}
			defer file.Close()

			// Upload the file to GCS
			log.Printf("uploding img to GCS %v", fileHeader.Filename)
			objectName := fileHeader.Filename
			if err := storageClient.UploadFile(file, objectName); err != nil {
				log.Fatalf("Error uploading file to GCS: %v", err)
				http.Error(w, "Error uploading file to GCS", http.StatusInternalServerError)
				return
			}

			// Prepare the Kafka message

			kafkaMsg := kafkaClient.NewMessage(objectName)

			log.Printf("sending msg to kafka")
			// Send the Kafka message
			if err := kafkaClient.ProduceMessage(kafkaMsg); err != nil {
				http.Error(w, "Error sending message to Kafka", http.StatusInternalServerError)
				return
			}

			// Wait for a response from Kafka
			select {
			case response := <-responseChan:
				log.Printf("received res from kafka")
				// Serve the response HTML to update the frontend
				fmt.Fprintf(w, "<div><img src='/images/%s' class='uploaded-image' alt='Processed Image'></div>", response.UpdatedImageUrl)
			case <-time.After(30 * time.Second): // Timeout after 30 seconds
				http.Error(w, "Timed out waiting for Kafka response", http.StatusRequestTimeout)
				return
			}
		}
	})

	// Start the server
	log.Fatal(http.ListenAndServe(":8080", nil))
}
