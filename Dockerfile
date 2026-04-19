# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod ./

# Download dependencies (none yet, but good practice)
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version injection
ARG VERSION=dev

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o sercha-core \
    ./cmd/sercha-core

# Runtime stage
FROM alpine:3.20

# Add ca-certificates for HTTPS, tzdata for timezones, and poppler-utils for PDF text extraction
RUN apk --no-cache add ca-certificates tzdata poppler-utils

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/sercha-core /app/sercha-core

# Create non-root user
RUN adduser -D -u 1000 sercha
USER sercha

# Expose default port
EXPOSE 8080

ENTRYPOINT ["/app/sercha-core"]
CMD ["all"]
