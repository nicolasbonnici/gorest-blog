# Importer Migration Guide

## Overview

The importer functionality has been moved from the blog project to the gorest-blog plugin to make it reusable across projects.

## Changes

### Old Structure
```
blog/plugins/importer/
├── cli/
├── engines/
├── http.go
├── plugin.go
├── service.go
└── types.go
```

### New Structure
```
gorest-blog/
├── importer/              # Importer engines and types
│   ├── engines/
│   ├── cli/
│   ├── http.go
│   ├── types.go
│   └── service.go (deprecated)
└── importer_service.go    # Blog-specific service implementation
```

## Architecture

### Service Factory Pattern

To avoid circular dependencies, the importer uses a factory pattern:

1. **importer** package: Contains engines and generic importer logic
2. **blog** package: Contains `ImporterService` that knows about `blog.Post`
3. **Factory**: Connects the two via `importer.SetServiceFactory()`

### Flow

```
CLI/HTTP → importer.ImportService (interface)
                    ↓
           blog.ImporterService (implementation)
                    ↓
              blog.Post model
```

## Migration Steps

### For Plugin Authors

1. Copy importer code to your plugin
2. Update imports to use your package paths
3. Create a service implementation in your main package (like `importer_service.go`)
4. Set the factory in routes.go or initialization code

### For Blog Project

The blog project now uses the plugin's importer:

```go
// cmd/import/main.go
func init() {
    importer.SetServiceFactory(func(db database.Database, reporter importer.ProgressReporter) importer.ImportService {
        return blog.NewImporterService(db, reporter)
    })
}
```

## Benefits

1. **Reusability**: Any project using gorest-blog can enable the importer
2. **Consistency**: Same importer logic across all projects
3. **Extensibility**: Easy to add new engines
4. **Maintainability**: Single source of truth for importer logic

## Backward Compatibility

The old `blog/plugins/importer` directory can be removed. All functionality is now provided by the plugin.
