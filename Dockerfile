# Use the official Go image for building the app
FROM golang:1.22-alpine AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files to download dependencies
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go app
RUN go build -o server .

# Use a minimal base image to reduce the size of the final image
FROM alpine:latest

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/server /app/server

# Copy static files
COPY static/ /app/static/

# Env vars
ENV KAFKA_BROKER_ADDR=""
ENV KAFKA_CONSUME_TOPIC="image-processing"
ENV KAFKA_PRODUCE_TOPIC="test-topic"
ENV GCP_PROJECT_ID="spring-firefly-407617"
ENV GCP_BUCKET_NAME="img-bucket-69"


# Expose port 8080
EXPOSE 8080

# Command to run the executable
CMD ["/app/server"]
