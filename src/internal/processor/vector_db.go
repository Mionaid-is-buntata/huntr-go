package processor

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	chromem "github.com/philippgille/chromem-go"
)

const (
	vectorDBPath   = "/data/chromadb"
	maxCollections = 2
)

// VectorDB wraps chromem-go operations.
type VectorDB struct {
	db   *chromem.DB
	path string
}

// NewVectorDB creates or opens a persistent chromem-go database.
func NewVectorDB(path string) (*VectorDB, error) {
	if path == "" {
		path = vectorDBPath
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("vector_db: mkdir: %w", err)
	}

	db, err := chromem.NewPersistentDB(path, false)
	if err != nil {
		return nil, fmt.Errorf("vector_db: open: %w", err)
	}
	return &VectorDB{db: db, path: path}, nil
}

// CreateCollection creates a new collection with timestamp metadata.
func (v *VectorDB) CreateCollection(name string, embeddingDim int) (*chromem.Collection, error) {
	// chromem-go uses an embedding function; for pre-computed embeddings we pass nil
	col, err := v.db.CreateCollection(name, map[string]string{
		"created_at": time.Now().Format(time.RFC3339),
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("create collection %s: %w", name, err)
	}
	slog.Info("created collection", "name", name)
	return col, nil
}

// GetCollection retrieves an existing collection.
func (v *VectorDB) GetCollection(name string) *chromem.Collection {
	return v.db.GetCollection(name, nil)
}

// ListCollections returns all collection names.
func (v *VectorDB) ListCollections() []string {
	collections := v.db.ListCollections()
	names := make([]string, 0, len(collections))
	for name := range collections {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// AutoRotateCollections removes the oldest collections when exceeding maxCollections.
func (v *VectorDB) AutoRotateCollections() {
	names := v.ListCollections()
	if len(names) <= maxCollections {
		return
	}

	// Collections are named cv_YYYYMMDD_HHMMSS so sorting alphabetically = chronological
	sort.Strings(names)
	toRemove := len(names) - maxCollections
	for i := 0; i < toRemove; i++ {
		if err := v.db.DeleteCollection(names[i]); err != nil {
			slog.Error("error deleting collection", "name", names[i], "error", err)
		} else {
			slog.Info("auto-rotated collection", "removed", names[i])
		}
	}
}

// StoreCVChunks stores pre-embedded CV chunks in a collection.
func (v *VectorDB) StoreCVChunks(collectionName string, chunks []CVChunkWithEmbedding, metadata map[string]string) error {
	v.AutoRotateCollections()

	col, err := v.CreateCollection(collectionName, 0)
	if err != nil {
		// Try getting existing
		col = v.GetCollection(collectionName)
		if col == nil {
			return fmt.Errorf("could not create or get collection %s: %w", collectionName, err)
		}
	}

	docs := make([]chromem.Document, len(chunks))
	for i, chunk := range chunks {
		meta := map[string]string{
			"chunk_index": fmt.Sprintf("%d", chunk.Index),
			"start_char":  fmt.Sprintf("%d", chunk.StartChar),
			"end_char":    fmt.Sprintf("%d", chunk.EndChar),
		}
		for k, v := range metadata {
			meta[k] = v
		}
		docs[i] = chromem.Document{
			ID:        fmt.Sprintf("chunk_%d", i),
			Content:   chunk.Text,
			Metadata:  meta,
			Embedding: chunk.Embedding,
		}
	}

	if err := col.AddDocuments(nil, docs, 1); err != nil {
		return fmt.Errorf("store chunks: %w", err)
	}

	slog.Info("stored CV chunks", "collection", collectionName, "count", len(chunks))
	return nil
}
