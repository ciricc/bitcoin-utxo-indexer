# Use the official Golang image to create a build artifact.
# This image is based on Debian, so if you need alpine or another base, adjust accordingly.
FROM golang:1.22 as builder

# Set the Current Working Directory inside the container.
WORKDIR /app

# Copy everything from the current directory to the PWD(Present Working Directory) inside the container.
COPY . .

# Build the Go app.
RUN go build -o /main ./cmd/main.go

# Use a Docker multi-stage build to create a lean production image.
# Start from scratch to keep the image size small.
FROM alpine:latest  

# Install ca-certificates for HTTPS requests (optional).
RUN apk --no-cache add ca-certificates

WORKDIR /

# Copy the Pre-built binary file from the previous stage. 
COPY --from=builder /main .

# Command to run the executable.
CMD ["./main"]