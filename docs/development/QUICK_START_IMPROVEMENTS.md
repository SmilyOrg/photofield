# Quick Start Improvements - Implementation Examples

This document provides specific, actionable examples for the highest-priority improvements identified in the roadmap.

## 1. Immediate Linting Setup

### Add to `.github/workflows/release.yml`

```yaml
# Add this step before the build steps
- name: Lint and Format Check
  run: |
    # Install golangci-lint
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.54.2
    
    # Run comprehensive linting
    golangci-lint run --timeout=5m
    
    # Check formatting
    gofmt -l . | tee /tmp/gofmt-output
    if [ -s /tmp/gofmt-output ]; then
      echo "Code is not properly formatted"
      exit 1
    fi
    
    # Run go vet
    go vet ./...
    
    # Check for common issues
    go run honnef.co/go/tools/cmd/staticcheck@latest ./...

- name: Frontend Lint
  run: |
    cd ui
    npm run lint
```

### Create `.golangci.yml` configuration:

```yaml
run:
  timeout: 5m
  skip-dirs:
    - vendor

linters-config:
  govet:
    check-shadowing: true
  golint:
    min-confidence: 0.8
  gocyclo:
    min-complexity: 15
  dupl:
    threshold: 100

linters:
  enable:
    - gofmt
    - golint
    - govet
    - ineffassign
    - misspell
    - staticcheck
    - unused
    - gocyclo
    - dupl
```

## 2. Fix Critical Context.TODO() Issues

### Before (current code):
```go
// internal/layout/flex.go:363
location, err := source.Geo.ReverseGeocode(context.TODO(), info.LatLng)
```

### After (improved):
```go
// internal/layout/flex.go:363
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
location, err := source.Geo.ReverseGeocode(ctx, info.LatLng)
```

### Implementation Plan:
1. **Add context parameter to layout functions**
2. **Implement proper timeout handling**
3. **Add cancellation support for long-running operations**

```go
// Example: Update function signatures
func (l *FlexLayout) LayoutPhotos(ctx context.Context, photos []Photo) error {
    // Use ctx instead of context.TODO()
    return l.processPhotos(ctx, photos)
}
```

## 3. Add Basic Test Infrastructure

### Create `internal/test/helpers/database_test.go`:

```go
package helpers

import (
    "context"
    "database/sql"
    "testing"
    
    _ "modernc.org/sqlite"
)

func SetupTestDB(t *testing.T) *sql.DB {
    db, err := sql.Open("sqlite", ":memory:")
    if err != nil {
        t.Fatalf("Failed to create test database: %v", err)
    }
    
    // Run migrations
    if err := runMigrations(db); err != nil {
        t.Fatalf("Failed to run migrations: %v", err)
    }
    
    t.Cleanup(func() {
        db.Close()
    })
    
    return db
}

func runMigrations(db *sql.DB) error {
    // Add migration logic here
    return nil
}
```

### Create `internal/layout/flex_test.go`:

```go
package layout

import (
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestFlexLayout_LayoutPhotos(t *testing.T) {
    tests := []struct {
        name     string
        photos   []Photo
        expected int
        wantErr  bool
    }{
        {
            name:     "empty photos",
            photos:   []Photo{},
            expected: 0,
            wantErr:  false,
        },
        {
            name: "single photo",
            photos: []Photo{
                {ID: "1", Width: 100, Height: 100},
            },
            expected: 1,
            wantErr:  false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            layout := NewFlexLayout()
            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()
            
            result, err := layout.LayoutPhotos(ctx, tt.photos)
            
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            
            require.NoError(t, err)
            assert.Equal(t, tt.expected, len(result.Items))
        })
    }
}
```

## 4. Performance Monitoring Setup

### Add to `main.go`:

```go
// Add performance monitoring middleware
func performanceMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            
            // Wrap response writer to capture status code
            wrapper := &responseWrapper{ResponseWriter: w, statusCode: 200}
            
            next.ServeHTTP(wrapper, r)
            
            duration := time.Since(start)
            
            // Log slow requests
            if duration > 1*time.Second {
                log.Printf("Slow request: %s %s took %v", r.Method, r.URL.Path, duration)
            }
            
            // Update metrics
            updateRequestMetrics(r.Method, r.URL.Path, wrapper.statusCode, duration)
        })
    }
}

type responseWrapper struct {
    http.ResponseWriter
    statusCode int
}

func (w *responseWrapper) WriteHeader(statusCode int) {
    w.statusCode = statusCode
    w.ResponseWriter.WriteHeader(statusCode)
}
```

## 5. Improve Error Handling

### Create `internal/errors/errors.go`:

```go
package errors

import (
    "fmt"
    "log"
)

type PhotofieldError struct {
    Code    string
    Message string
    Cause   error
}

func (e *PhotofieldError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *PhotofieldError) Unwrap() error {
    return e.Cause
}

// Common error constructors
func NewImageProcessingError(operation string, cause error) *PhotofieldError {
    return &PhotofieldError{
        Code:    "IMAGE_PROCESSING_ERROR",
        Message: fmt.Sprintf("Failed to %s", operation),
        Cause:   cause,
    }
}

func NewDatabaseError(operation string, cause error) *PhotofieldError {
    return &PhotofieldError{
        Code:    "DATABASE_ERROR", 
        Message: fmt.Sprintf("Database operation failed: %s", operation),
        Cause:   cause,
    }
}

// Error middleware for HTTP handlers
func ErrorHandler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Printf("Panic in %s %s: %v", r.Method, r.URL.Path, err)
                http.Error(w, "Internal server error", http.StatusInternalServerError)
            }
        }()
        
        next.ServeHTTP(w, r)
    })
}
```

## 6. Add Configuration Validation

### Create `config_validation.go`:

```go
package main

import (
    "fmt"
    "os"
    "path/filepath"
)

func (c *Config) Validate() error {
    if len(c.Collections) == 0 {
        return fmt.Errorf("at least one collection must be configured")
    }
    
    for i, collection := range c.Collections {
        if err := c.validateCollection(i, collection); err != nil {
            return err
        }
    }
    
    return nil
}

func (c *Config) validateCollection(index int, collection Collection) error {
    if collection.Name == "" {
        return fmt.Errorf("collection[%d]: name is required", index)
    }
    
    if len(collection.Dirs) == 0 {
        return fmt.Errorf("collection[%d]: at least one directory is required", index)
    }
    
    for j, dir := range collection.Dirs {
        if _, err := os.Stat(dir); os.IsNotExist(err) {
            return fmt.Errorf("collection[%d].dirs[%d]: directory %s does not exist", index, j, dir)
        }
        
        if !filepath.IsAbs(dir) {
            return fmt.Errorf("collection[%d].dirs[%d]: directory %s must be absolute path", index, j, dir)
        }
    }
    
    return nil
}
```

## Implementation Priority

1. **Week 1**: Linting setup + Context fixes
2. **Week 2**: Basic test infrastructure + Error handling
3. **Week 3**: Performance monitoring + Configuration validation
4. **Week 4**: Expand test coverage for core modules

Each of these changes can be implemented incrementally and will provide immediate value to the development workflow and code quality.