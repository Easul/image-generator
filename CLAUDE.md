# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a self-hosted AI image generation web application built with Go and vanilla JavaScript. It acts as a frontend/proxy to OpenAI-compatible image generation APIs, providing:
- Image generation from text prompts
- Image editing (inpainting)
- Background removal
- User authentication with admin controls
- Gallery and sharing features
- API key-based programmatic access

## Build & Run Commands

### Development
```bash
# Build the application
go build -o bin/server ./cmd/server

# Run the application
./bin/server

# The server will start on :8080 by default (configurable in config.yaml)
```

### Production Build
```bash
# Build for Alpine Linux (static binary)
# Output will be in dist/ig (overwrites existing binary)
OUTPUT_NAME=ig ./build-alpine.sh

# The binary is placed at dist/ig and can be deployed directly
# This is the standard build process - always use build-alpine.sh for production

# Custom build options (if needed)
GOOS=linux GOARCH=arm64 OUTPUT_NAME=ig ./build-alpine.sh
```

### Testing
```bash
# Run all tests (currently no tests exist)
go test -v ./...

# Run tests for a specific package
go test -v ./internal/service
```

### Dependencies
```bash
# Download dependencies
go mod download

# Update dependencies
go mod tidy
```

## Architecture

### Layered Structure
```
HTTP Layer (Gin Router)
     ↓
Middleware (Session/API Key Auth)
     ↓
Handlers (HTTP Controllers)
     ↓
Services (Business Logic)
     ↓
Database (GORM + SQLite)
```

### Key Packages

- **`cmd/server`** - Application entry point
  - `main.go` - Server initialization, route registration, embedded assets
  - `web/` - Frontend HTML/JS/CSS (embedded into binary via `embed` package)

- **`internal/config`** - Configuration management
  - Loads `config.yaml` at startup
  - Provides AES-256-GCM encryption utilities for sensitive data (API keys)
  - Encryption format: `enc:base64(nonce+ciphertext)`

- **`internal/database`** - Database layer
  - GORM initialization with SQLite
  - Auto-migration for all models
  - Seed function for default settings
  - Backfill function for usage logs

- **`internal/model`** - Data models (GORM entities)
  - `User` - User accounts with admin/ban flags
  - `Image` - Generated/edited images with metadata
  - `ApiKey` - API keys for programmatic access (OpenAI-style: `sk-proj-...`)
  - `Setting` - Dynamic configuration KV store
  - `Share` - Public image sharing tokens
  - `UsageLog` - Audit trail

- **`internal/handler`** - HTTP request handlers
  - All handlers follow pattern: validate input → call service → return JSON
  - Error responses return `{"error": "message"}` with appropriate status codes
  - Success responses vary by endpoint

- **`internal/service`** - Business logic
  - `ImageService` - Core image operations, API calls to OpenAI-compatible endpoints
  - `ApiKeyService` - API key validation and management
  - `ShareService` - Share token generation

- **`internal/middleware`** - HTTP middleware
  - `AuthRequired(db)` - Session-based auth for browser users
  - `AdminRequired(db)` - Admin-only routes
  - `ApiKeyAuth(db)` - API key validation from Authorization header

### Authentication Flow

Two authentication strategies coexist:

1. **Session-based** (browser users):
   - Uses Gorilla sessions with secure cookies
   - Session stored in memory (not persisted)
   - Middleware: `AuthRequired(db)`

2. **API key-based** (programmatic access):
   - Bearer token in `Authorization` header
   - Format: `Authorization: Bearer sk-proj-...`
   - Middleware: `ApiKeyAuth(db)`
   - Updates `last_used_at` timestamp on each request

### Image Processing Flow

```
1. User submits request (generate/edit/remove-bg)
2. Handler creates Image record with status="pending"
3. Service calls external OpenAI-compatible API
4. Response contains either:
   - URL (proxied through /api/image/proxy)
   - Base64 data (saved to disk, served locally)
5. Image record updated with status="success" or "failed"
6. Frontend polls /api/task/:id until completion
```

### Configuration System

**`config.yaml`** structure:
- `server` - Address, session secret, cookie settings
- `database` - SQLite file path
- `storage` - Upload directories, max file size
- `openai` - External API endpoint and key
- `defaults` - Site branding, models, image sizes

**Dynamic settings** (stored in database `settings` table):
- Can be modified via admin UI at runtime
- Encrypted fields (like API keys) are prefixed with `enc:` in database
- Use `config.EncryptString()` / `config.DecryptString()` utilities

## Frontend Structure

### Pages & JS Modules

Each HTML page has a corresponding JS file:
- `index.html` + `app.js` - Main workbench
- `gallery.html` + `gallery.js` - Image gallery
- `admin.html` + `admin.js` - Admin dashboard
- `login.html` / `register.html` + `auth.js` - Authentication
- `password.html` + `password.js` - Password change
- `share.html` + `share.js` - Public shared images

### Communication Pattern

All JS modules use a common `api()` function:
```javascript
async function api(path, options = {}) {
  const response = await fetch(path, {
    credentials: 'include',  // Send cookies for sessions
    headers: { 'Content-Type': 'application/json' },
    ...options
  });
  const data = await response.json().catch(() => ({}));
  if (response.status === 401) window.location.href = '/login.html';
  if (!response.ok) throw new Error(data.error || '请求失败');
  return data;
}
```

### Asset Embedding

Frontend assets are embedded into the Go binary:
```go
//go:embed web/assets/*
var assetsFS embed.FS

//go:embed web/*.html
var htmlFS embed.FS
```

This means changes to web files require a rebuild to take effect.

## API Endpoints

### Public (no auth)
- `POST /api/register` - User registration
- `POST /api/login` - User login
- `GET /api/config` - Client config (site name, models, sizes)
- `GET /api/share/:token` - View shared image

### Protected (session or API key)
- `POST /api/images/generate` - Generate image from prompt
- `POST /api/images/edit` - Edit image (inpainting)
- `POST /api/images/remove-bg` - Remove background
- `GET /api/task/:id` - Poll task status
- `GET /api/gallery` - List user's images
- `DELETE /api/gallery/:id` - Delete image
- `POST /api/share` - Create share link

### Admin only (`/api/admin/*`)
- `GET/POST /admin/settings` - System settings CRUD
- `GET /admin/users` - List all users
- `POST /admin/user/admin` - Grant admin privileges
- `POST /admin/user/ban` - Ban user
- `POST /admin/user/reset-password` - Reset user password
- `GET/POST /admin/models` - Configure available models
- `POST /admin/test-api` - Test OpenAI API connection
- `GET /admin/api-keys` - List API keys
- `POST /admin/api-keys` - Create API key
- `DELETE /admin/api-keys` - Delete API key

### API Key Routes (`/api/v1/*`)
Duplicate endpoints for programmatic access via API keys:
- `POST /api/v1/images/generate`
- `POST /api/v1/images/edit`
- `POST /api/v1/images/remove-bg`

## Important Patterns & Conventions

### Error Handling
- Database errors: Return 500 with generic "操作失败" message
- Not found: Return 404 with specific error
- Unauthorized: Return 401, frontend redirects to login
- Validation errors: Return 400 with descriptive message

### Database Transactions
- Not currently used (consider adding for multi-step operations)
- GORM provides `db.Transaction(func(tx *gorm.DB) error { ... })`

### File Storage
- User uploads: `storage.original_dir` (default: `uploads/original/`)
- Generated images: `storage.generated_dir` (default: `uploads/generated/`)
- File naming: `{userID}_{timestamp}_{random}.{ext}`

### Sensitive Data
- API keys are encrypted in database using AES-256-GCM
- Session secret must be changed in production
- Passwords hashed with bcrypt (handled by auth package)

## Common Development Tasks

### Adding a New Image Operation

1. Add handler method in `internal/handler/image_handler.go`
2. Implement business logic in `internal/service/image_service.go`
3. Add route in `cmd/server/main.go` under protected or apiAuth groups
4. Update frontend in relevant JS file (usually `app.js`)

### Adding a New Model/Provider

1. Update `available_models` in `config.yaml` defaults
2. Admin can also add models via UI at runtime
3. If API format differs from OpenAI, extend `ImageService.callAPI()` logic

### Modifying Database Schema

1. Update model struct in `internal/model/`
2. GORM will auto-migrate on next startup (adds columns, doesn't remove)
3. For breaking changes, write manual migration in `database.Init()`

### Adding Admin Features

1. Create handler in `internal/handler/admin_handler.go`
2. Add route under `admin` group in `main.go`
3. Update `admin.html` and `admin.js` for UI

## Security Considerations

- **Session Secret**: Must be changed from default in production
- **Secure Cookies**: Enable `server.secure_cookies: true` when using HTTPS
- **API Key Format**: OpenAI-style (`sk-proj-{64-char-hex}`) for compatibility
- **Brute Force Protection**: Implemented in auth handler (tracks failed attempts)
- **User Banning**: Banned users cannot authenticate via session or API key
- **Admin Actions**: All admin operations require `is_admin=true` flag

## Deployment Notes

- Single binary deployment (all assets embedded)
- Requires writable directories for database and uploads
- SQLite database is portable (copy `data/app.db` to migrate)
- Default port :8080 (configurable)
- No external dependencies at runtime (pure Go + SQLite)

## Git Workflow

- Main branch: `main`
- Feature branches: `feature/{name}`
- Always create new commits (don't amend published commits)
- Include `Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>` in commit messages
