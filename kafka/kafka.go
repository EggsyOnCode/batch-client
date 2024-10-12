package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"scalabale-img-processor/storage"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Filters is a slice of Filter
type Filters []Filter

// Filter represents an imagor endpoint filter
type Filter struct {
	Name string `json:"name,omitempty"`
	Args string `json:"args,omitempty"`
}

// KafkaMessageResponse represents a response from Kafka
type KafkaMessageResponse struct {
	UpdatedImageUrl string `json:"updated_image_url"`
}

type Params struct {
	Params        bool    `json:"-"`
	Path          string  `json:"path,omitempty"`
	Image         string  `json:"image,omitempty"`
	Unsafe        bool    `json:"unsafe,omitempty"`
	Hash          string  `json:"hash,omitempty"`
	Meta          bool    `json:"meta,omitempty"`
	Trim          bool    `json:"trim,omitempty"`
	TrimBy        string  `json:"trim_by,omitempty"`
	TrimTolerance int     `json:"trim_tolerance,omitempty"`
	CropLeft      float64 `json:"crop_left,omitempty"`
	CropTop       float64 `json:"crop_top,omitempty"`
	CropRight     float64 `json:"crop_right,omitempty"`
	CropBottom    float64 `json:"crop_bottom,omitempty"`
	FitIn         bool    `json:"fit_in,omitempty"`
	Stretch       bool    `json:"stretch,omitempty"`
	Width         int     `json:"width,omitempty"`
	Height        int     `json:"height,omitempty"`
	PaddingLeft   int     `json:"padding_left,omitempty"`
	PaddingTop    int     `json:"padding_top,omitempty"`
	PaddingRight  int     `json:"padding_right,omitempty"`
	PaddingBottom int     `json:"padding_bottom,omitempty"`
	HFlip         bool    `json:"h_flip,omitempty"`
	VFlip         bool    `json:"v_flip,omitempty"`
	HAlign        string  `json:"h_align,omitempty"`
	VAlign        string  `json:"v_align,omitempty"`
	Smart         bool    `json:"smart,omitempty"`
	Filters       Filters `json:"filters,omitempty"`
}

// KafkaMessage struct to be sent in Kafka messages
type KafkaMessage struct {
	ImageParams Params `json:"image_params"`
	ImageName   string `json:"image_url"`
}

type KafkaClient struct {
	client *kgo.Client
}

func NewKafkaClient() (*KafkaClient, error) {
	// Set up Kafka client options using environment variables
	opts := []kgo.Opt{
		kgo.SeedBrokers(os.Getenv("KAFKA_BROKER_ADDR")),
		kgo.DefaultProduceTopic(os.Getenv("KAFKA_PRODUCE_TOPIC")),
		kgo.ConsumeTopics(os.Getenv("KAFKA_CONSUME_TOPIC")),
		kgo.ClientID("producer-1"),
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka client: %w", err)
	}

	return &KafkaClient{client: client}, nil
}

// ProduceMessage sends a Kafka message
func (kc *KafkaClient) ProduceMessage(kafkaMessage KafkaMessage) error {
	msgBytes, err := json.Marshal(kafkaMessage)
	if err != nil {
		return fmt.Errorf("failed to serialize Kafka message: %w", err)
	}

	// Prepare the Kafka message
	record := &kgo.Record{
		Topic: os.Getenv("KAFKA_PRODUCE_TOPIC"),
		Value: msgBytes,
	}

	log.Printf("Kafka message JSON: %s", msgBytes)

	// Send the message to Kafka
	if err := kc.client.ProduceSync(context.Background(), record).FirstErr(); err != nil {
		return fmt.Errorf("failed to send message to Kafka: %w", err)
	}
	log.Println("Message sent to Kafka successfully")

	return nil
}

func (kc *KafkaClient) NewMessage(objName string) KafkaMessage {
	filters := Filters{
		{
			Name: "resize",  // Example filter
			Args: "90x1000", // Arguments for resizing the image
		},
		{
			Name: "crop",
			Args: "0.6,0.3,0.9,0.9", // Arguments for cropping (left, top, right, bottom)
		},
		{
			Name: "rotate", // Add the rotate filter
			Args: "90",     // Rotate the image 90 degrees
		},
	}

	// Create an example Params struct
	imageParams := Params{
		Path:          "",
		Image:         objName,
		Trim:          true,
		TrimTolerance: 10,
		CropLeft:      0.1,
		CropTop:       0.1,
		CropRight:     0.9,
		CropBottom:    0.9,
		Width:         500,
		Height:        500,
		HFlip:         true,
		HAlign:        "center",
		VAlign:        "middle",
		Smart:         true,
		Filters:       filters,
	}

	// Create the KafkaMessage
	kafkaMessage := KafkaMessage{
		ImageParams: imageParams,
		ImageName:   objName,
	}

	return kafkaMessage

}

// ListenToResponses listens for messages from the response topic
func (kc *KafkaClient) ListenToResponses() {
	for {
		fetches := kc.client.PollFetches(context.Background())
		fetches.EachRecord(func(record *kgo.Record) {
			var kafkaMessage KafkaMessageResponse
			err := json.Unmarshal(record.Value, &kafkaMessage)
			if err != nil {
				log.Printf("Error parsing Kafka record: %v", err)
				return
			}

			log.Printf("Updated image URL from Kafka topic: %s", kafkaMessage.UpdatedImageUrl)
		})

		time.Sleep(100 * time.Millisecond)
	}
}

// ListenAndRespond listens for messages on the response topic and sends the response to the provided channel.
func (kc *KafkaClient) ListenAndRespond(responseChan chan KafkaMessageResponse, storageClient *storage.ClientUploader) {
	for {
		fetches := kc.client.PollFetches(context.Background())
		fetches.EachRecord(func(record *kgo.Record) {
			var kafkaMessage KafkaMessageResponse
			err := json.Unmarshal(record.Value, &kafkaMessage)
			if err != nil {
				log.Printf("Error parsing Kafka record: %v", err)
				return
			}

			// Get the image URL from the Kafka message
			imageUrl := kafkaMessage.UpdatedImageUrl

			// Download the image using the storage client
			imageFileName := "processed_image_" + time.Now().Format("20060102150405") + ".png" // Unique name for the downloaded image
			if err := storageClient.GetFile(imageUrl, imageFileName); err != nil {
				log.Printf("Error downloading image: %v", err)
				return
			}

			// Send the downloaded image file name back to the response channel
			responseChan <- KafkaMessageResponse{UpdatedImageUrl: imageFileName}
		})

		time.Sleep(100 * time.Millisecond)
	}
}

func (kc *KafkaClient) Close() {
	kc.client.Close()
}
