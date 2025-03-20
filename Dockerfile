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

version: '3.8'

services:
  postgres:
    image: ankane/pgvector:latest
    container_name: vision-postgres
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: vision_analysis
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  postgres_data: