# Use the official Go image as the base image
FROM golang:1.24 as builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application files
COPY . .

# Build the Go binary
RUN go build -o vision-frame-analyzer main.go

# Use a lightweight image for the final container
FROM debian:bookworm-slim

# Install FFmpeg (needed for frame extraction)
RUN apt-get update && apt-get install -y ffmpeg && rm -rf /var/lib/apt/lists/*

# Set the working directory inside the container
WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/vision-frame-analyzer /app/vision-frame-analyzer

# Set execution permissions
RUN chmod +x /app/vision-frame-analyzer

# Set the default command to run the application
ENTRYPOINT ["/app/vision-frame-analyzer"]

# Default arguments (can be overridden at runtime)
CMD ["--video", "input.mp4", "--output", "output_frames"]