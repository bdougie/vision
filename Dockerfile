# Use the official Go image as the base image
FROM golang:1.21-alpine

# Install ffmpeg and build dependencies
RUN apk add --no-cache ffmpeg gcc musl-dev

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application files
COPY . .

# Build the Go binary
RUN go build -o vision-analyzer

# Set the entrypoint
ENTRYPOINT ["./vision-analyzer"]