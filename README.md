# YouTube Serverless Platform

A lightweight serverless platform that allows users to upload code in a zip file, which is then containerized and executed on demand.

## Features

- Upload code as a zip file
- Automatic language detection (Python and Go)
- Docker containerization for isolation and security
- RESTful API for function management
- Configurable via environment variables
- Proper error handling and logging

## Requirements

- Go 1.19 or higher
- Docker
- Python 3.9 (for Python functions)

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/youtube_serverless.git
   cd youtube_serverless
   ```

2. Build the application:
   ```bash
   go build -o serverless
   ```

3. Run the application:
   ```bash
   ./serverless
   ```

## Configuration

The application can be configured using environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| SERVER_PORT | Port to listen on | 8080 |
| SERVER_READ_TIMEOUT | Read timeout in seconds | 10s |
| SERVER_WRITE_TIMEOUT | Write timeout in seconds | 10s |
| SERVER_SHUTDOWN_TIMEOUT | Graceful shutdown timeout | 5s |
| DOCKER_IMAGE_PREFIX | Prefix for Docker images | youtube-serverless |
| DOCKER_CONTAINER_LIMIT | Maximum number of containers | 10 |
| DOCKER_RUN_TIMEOUT | Container execution timeout | 30s |
| MAX_FILE_SIZE | Maximum file size in bytes | 10MB |
| TEMP_DIR_BASE | Base directory for temporary files | system default |
| LOG_LEVEL | Logging level (debug, info, warn, error) | info |

## API Endpoints

### Submit a Function

```
POST /api/submit
```

**Request:**
- Content-Type: multipart/form-data
- Form Fields:
  - `code`: Zip file containing the function code
  - `name` (optional): Function name

**Response:**
```json
{
  "functionId": "uuid",
  "imageId": "sha256:...",
  "message": "Function 'name' deployed successfully"
}
```

### Execute a Function

```
GET /api/execute?functionId=uuid
```

or

```
POST /api/execute
```

**Request Body (POST):**
```json
{
  "functionId": "uuid",
  "input": {
    "key1": "value1",
    "key2": "value2"
  }
}
```

**Response:**
```json
{
  "output": "Function output",
  "statusCode": 200,
  "executedAt": 1621234567
}
```

The `input` parameters are passed to the function as environment variables. For example, if you provide `{"name": "John"}` as input, your function will have access to an environment variable named `NAME` with the value `"John"`.

### List Functions

```
GET /api/functions
```

**Response:**
```json
[
  {
    "functionId": "uuid1",
    "imageId": "sha256:...",
    "language": "python",
    "createdAt": 1621234567,
    "lastExecuted": 1621234568,
    "name": "function1"
  },
  {
    "functionId": "uuid2",
    "imageId": "sha256:...",
    "language": "golang",
    "createdAt": 1621234569,
    "name": "function2"
  }
]
```

### Get Function Details

```
GET /api/functions/{functionId}
```

**Response:**
```json
{
  "functionId": "uuid",
  "imageId": "sha256:...",
  "language": "python",
  "createdAt": 1621234567,
  "lastExecuted": 1621234568,
  "name": "function1"
}
```

### Delete Function

```
DELETE /api/functions/{functionId}
```

**Response:**
```json
{
  "message": "Function uuid deleted successfully"
}
```

### Health Check

```
GET /health
```

**Response:**
```json
{
  "status": "ok",
  "time": "2023-01-16T12:34:56Z"
}
```

## Function Structure

### Python Functions

Python functions should have a main handler file that will be executed when the function is invoked.

Example:
```python
def main():
    print("Hello from Python function!")

if __name__ == "__main__":
    main()
```

### Go Functions

Go functions should have a main package with a main function.

Example:
```go
package main

import "fmt"

func main() {
    fmt.Println("Hello from Go function!")
}
```

## Security Considerations

- Functions run in isolated Docker containers with limited resources
- Containers run with read-only filesystem
- All capabilities are dropped
- No network access is provided
- Memory and CPU limits are enforced

## License

MIT
