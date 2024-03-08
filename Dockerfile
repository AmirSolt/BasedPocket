# Use a Go base image for building
FROM golang:1.22-alpine AS builder

# Set a working directory
WORKDIR /app

# Copy the Go modules and sum files first (for caching)
COPY go.mod go.sum ./

# Download Go module dependencies
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 go build -o /basedpocket -ldflags="-s -w" .

# Use a fresh alpine base for the runtime
FROM alpine:latest

# Install ca-certificates
RUN apk add --no-cache ca-certificates

# Copy the binary from the builder stage
COPY --from=builder /basedpocket /basedpocket

# Expose port 8080 for the application
EXPOSE 8080

# Command to run the binary
CMD ["/basedpocket", "serve", "--http=0.0.0.0:8080"]