# Use the official Golang image to create a build artifact.
# This image is based on Debian, so if you need alpine or another base, adjust accordingly.
FROM golang:1.22-alpine as builder

# Set the Current Working Directory inside the container.
WORKDIR /app

# Copy everything from the current directory to the PWD(Present Working Directory) inside the container.
COPY . .

RUN go mod download

# Build the Go app.
RUN mkdir -p ./bin
RUN go build -o ./bin ./...

# Use a Docker multi-stage build to create a lean production image.
# Start from scratch to keep the image size small.
FROM alpine:latest  

# Install ca-certificates for HTTPS requests (optional).
RUN apk --no-cache add ca-certificates

WORKDIR /

# Copy the Pre-built binary file from the previous stage. 
COPY --from=builder /app/bin /usr/local/bin

# Command to run the executable.
CMD ["/usr/local/bin/utxo-indexer"]