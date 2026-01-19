# X-Extract API Documentation

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

Currently, the API does not require authentication. This may be added in future versions.

## Endpoints

### Health Check

#### GET /health

Returns the health status of the application.

**Response:**
```json
{
  "status": "ok",
  "version": "1.0.0",
  "queue": {
    "running": true
  }
}
```

#### GET /ready

Returns readiness status for load balancers.

**Response:**
```json
{
  "status": "ready"
}
```

### Downloads

#### POST /api/v1/downloads

Add a new download to the queue.

**Request Body:**
```json
{
  "url": "https://x.com/user/status/123456789",
  "platform": "x",
  "mode": "default"
}
```

**Parameters:**
- `url` (required): The URL to download
- `platform` (optional): Platform type (`x` or `telegram`). Auto-detected if not provided.
- `mode` (optional): Download mode (`default`, `single`, `group`). Default: `default`

**Response:** `201 Created`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "url": "https://x.com/user/status/123456789",
  "platform": "x",
  "status": "queued",
  "mode": "default",
  "priority": 0,
  "retry_count": 0,
  "created_at": "2024-01-14T10:30:00Z",
  "updated_at": "2024-01-14T10:30:00Z"
}
```

#### GET /api/v1/downloads

List all downloads with optional filtering.

**Query Parameters:**
- `status` (optional): Filter by status (`queued`, `processing`, `completed`, `failed`, `cancelled`)
- `platform` (optional): Filter by platform (`x`, `telegram`)

**Response:** `200 OK`
```json
[
  {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "url": "https://x.com/user/status/123456789",
    "platform": "x",
    "status": "completed",
    "mode": "default",
    "file_path": "/path/to/downloaded/file.mp4",
    "created_at": "2024-01-14T10:30:00Z",
    "completed_at": "2024-01-14T10:31:00Z"
  }
]
```

#### GET /api/v1/downloads/:id

Get details of a specific download.

**Response:** `200 OK`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "url": "https://x.com/user/status/123456789",
  "platform": "x",
  "status": "completed",
  "mode": "default",
  "priority": 0,
  "retry_count": 0,
  "file_path": "/path/to/downloaded/file.mp4",
  "metadata": "{\"files\":[\"file.mp4\"]}",
  "created_at": "2024-01-14T10:30:00Z",
  "started_at": "2024-01-14T10:30:05Z",
  "completed_at": "2024-01-14T10:31:00Z"
}
```

#### GET /api/v1/downloads/stats

Get download statistics.

**Response:** `200 OK`
```json
{
  "total": 100,
  "queued": 5,
  "processing": 2,
  "completed": 85,
  "failed": 7,
  "cancelled": 1
}
```

#### POST /api/v1/downloads/:id/cancel

Cancel a queued or processing download.

**Response:** `200 OK`
```json
{
  "message": "download cancelled"
}
```

#### POST /api/v1/downloads/:id/retry

Retry a failed download.

**Response:** `200 OK`
```json
{
  "message": "download queued for retry"
}
```

## Error Responses

All endpoints may return the following error responses:

### 400 Bad Request
```json
{
  "error": "invalid request parameters"
}
```

### 404 Not Found
```json
{
  "error": "download not found"
}
```

### 500 Internal Server Error
```json
{
  "error": "internal server error"
}
```

## Rate Limiting

Currently, there is no rate limiting. This may be added in future versions.

## Webhooks

Webhook support is planned for future versions to notify external systems of download events.

## Examples

### cURL Examples

```bash
# Add a download
curl -X POST http://localhost:8080/api/v1/downloads \
  -H "Content-Type: application/json" \
  -d '{"url": "https://x.com/user/status/123"}'

# List all downloads
curl http://localhost:8080/api/v1/downloads

# List completed downloads
curl http://localhost:8080/api/v1/downloads?status=completed

# Get download details
curl http://localhost:8080/api/v1/downloads/550e8400-e29b-41d4-a716-446655440000

# Get statistics
curl http://localhost:8080/api/v1/downloads/stats

# Cancel download
curl -X POST http://localhost:8080/api/v1/downloads/550e8400-e29b-41d4-a716-446655440000/cancel

# Retry download
curl -X POST http://localhost:8080/api/v1/downloads/550e8400-e29b-41d4-a716-446655440000/retry
```

