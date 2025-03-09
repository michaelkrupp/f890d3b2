# Homecase Image Server

A microservices-based system providing secure image storage with authentication:

- **Authentication Service**: Manages users and authentication tokens
- **Image Service**: Handles secure image storage and retrieval


## Quick Start

The fastest way to run the project is using Docker Compose:

```bash
make docker
```


## Core Features

### Image Management
- Upload multiple images (JPEG, PNG, TIFF)
- Secure access control
- On-demand image resizing with caching
- Automatic image deduplication
- File size limits (default 20MB per file)

### Authentication
- User registration and login
- Token-based authentication
- RSA-signed tokens


## API Reference

### Authentication Service (`localhost:8080`)

#### Register User
```bash
curl -X POST http://localhost:8080/auth/register \
  -d "username=myuser" \
  -d "password=mypassword"
```

#### Login
```bash
curl -X POST http://localhost:8080/auth/login \
  -d "username=myuser" \
  -d "password=mypassword"
```
Returns a token for API access.

### Image Service (`localhost:8081`) 

All endpoints require authentication via Bearer token:
```bash
Authorization: Bearer <your_token>
```

#### Upload Images
```bash
curl -X POST http://localhost:8081/media \
  -H "Authorization: Bearer <your_token>" \
  -F "file=@image.jpg"
```

#### Download Image
```bash
# Original size
curl -X GET http://localhost:8081/media/<media_id> \
  -H "Authorization: Bearer <your_token>"

# Resized version
curl -X GET "http://localhost:8081/media/<media_id>?width=800" \
  -H "Authorization: Bearer <your_token>"
```

#### Delete Image
```bash
curl -X DELETE http://localhost:8081/media/<media_id> \
  -H "Authorization: Bearer <your_token>"
```

## Configuration

Both services use environment variables for configuration. You can set these directly or use a `.env` file.

### Essential Configuration

```bash
# Auth Service (DEMO_AUTHSVC_*)
export DEMO_AUTHSVC_HTTP_SERVER_ADDR=:8080
export DEMO_AUTHSVC_USER_DATABASE_PATH=var/storage/authsvc.db
export DEMO_AUTHSVC_AUTH_SIGNING_KEY_FILE=var/storage/authsvc.key
export DEMO_AUTHSVC_AUTH_TOKEN_DURATION=3600

# Image Service (DEMO_IMAGESVC_*)
export DEMO_IMAGESVC_HTTP_SERVER_ADDR=:8081
export DEMO_IMAGESVC_BLOB_BASEDIR=var/storage/blob
export DEMO_IMAGESVC_AUTH_CLIENT_AUTH_URL=http://localhost:8080/auth/validate
export DEMO_IMAGESVC_MEDIA_MAX_SIZE=20971520
```

See the [Configuration Reference](#configuration-reference) section below for all available options.

## Project Structure

```
.
├── cmd/                # Service entry points
│   ├── authsvc/       # Authentication service
│   └── imagesvc/      # Image service
├── internal/          
│   ├── domain/        # Core domain models
│   ├── infra/         # Infrastructure code
│   ├── repo/          # Data storage
│   ├── svc/           # Service implementations
│   └── util/          # Shared utilities
└── var/               # Runtime data
```

## Building From Source

### Prerequisites
- go 1.21 or later
- docker (optional)
- make (optional)
- gotestsum (optional)
- prox (optional)

### Manual Build

```bash
# Build services
go build -o bin/authsvc ./cmd/authsvc
go build -o bin/imagesvc ./cmd/imagesvc

# Run services
source .env
./bin/authsvc & ./bin/imagesvc
```

### Using Make

```bash
# Show available commands
make help

# Build and run with Docker
make docker

# Run from source using prox
make run

# Build locally
make build

# Run tests
make test
```

## Configuration Reference

### Auth Service (`DEMO_AUTHSVC_*`)

#### Logging
- `LOG_OUTPUT`: Log output destination ("stdout", "stderr", "discard" or file path) [default: "stderr"]
- `LOG_LEVEL`: Minimum log level ("debug", "info", "warn", "error") [default: "info"]
- `LOG_FILTER`: Package-level logging overrides ("pkg:level,pkg:level") [default: ""]
- `LOG_JSON`: Enable JSON-formatted output [default: false]

#### Authentication
- `AUTH_SIGNING_KEY_FILE`: Path to RSA private key file [default: "var/storage/authsvc.key"]
- `AUTH_TOKEN_DURATION`: Auth token validity duration in seconds [default: 3600]

#### HTTP Server
- `HTTP_SERVER_ADDR`: Server listen address [default: ":8080"]
- `HTTP_READ_HEADER_TIMEOUT`: Header read timeout in seconds [default: 5]
- `HTTP_READ_TIMEOUT`: Request read timeout in seconds [default: 5]
- `HTTP_WRITE_TIMEOUT`: Response write timeout in seconds [default: 5]

#### User Storage
- `USER_DATABASE_PATH`: SQLite database file path [default: "var/storage/authsvc.db"]

### Image Service (`DEMO_IMAGESVC_*`)

#### Logging
- `LOG_OUTPUT`: Log output destination ("stdout", "stderr", "discard" or file path) [default: "stderr"]
- `LOG_LEVEL`: Minimum log level ("debug", "info", "warn", "error") [default: "info"]
- `LOG_FILTER`: Package-level logging overrides ("pkg:level,pkg:level") [default: ""]
- `LOG_JSON`: Enable JSON-formatted output [default: false]

#### Media Handling
- `MEDIA_MAX_SIZE`: Maximum allowed file size in bytes [default: 20971520]
- `IMAGE_INTERPOLATOR`: Image scaling algorithm ("nearestneighbor", "catmullrom", "bilinear", "approxbilinear") [default: "catmullrom"]

#### HTTP Server
- `IMAGE_HTTP_SERVER_ADDR`: Server listen address [default: ":8080"]
- `IMAGE_HTTP_READ_HEADER_TIMEOUT`: Header read timeout in seconds [default: 5]
- `IMAGE_HTTP_READ_TIMEOUT`: Request read timeout in seconds [default: 5]
- `IMAGE_HTTP_WRITE_TIMEOUT`: Response write timeout in seconds [default: 5]
- `IMAGE_HTTP_MULTIPART_FILE_NAME`: Form field name for file uploads [default: "upload"]
- `IMAGE_HTTP_URL_FILE_ID_PARAM`: URL parameter name for image IDs [default: "media_id"]
- `IMAGE_HTTP_URL_FILE_DOWNLOAD_PARAM`: URL parameter for triggering downloads [default: "download"]
- `IMAGE_HTTP_URL_WIDTH_PARAM`: URL parameter for specifying image resize width [default: "width"]
- `IMAGE_HTTP_CONTENT_DISPOSITION_DOWNLOAD`: Enable download headers [default: false]
- `IMAGE_HTTP_MULTIPART_FORM_MAX_SIZE`: Maximum allowed memory for multipart form uploads [default: 10485760]

#### Auth Client
- `AUTH_CLIENT_AUTH_URL`: Auth service validation endpoint [default: "http://localhost:8080/auth/validate"]

#### Blob Storage
- `BLOB_BASEDIR`: Root directory for blob storage [default: "var/storage/blob"]
