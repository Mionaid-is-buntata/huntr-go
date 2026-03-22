package processor

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/campbell/huntr-ai/internal/models"
)

const (
	DefaultChunkSize    = 600
	DefaultChunkOverlap = 120
)

// ChunkText splits text into overlapping segments with word-boundary breaks.
func ChunkText(text string, chunkSize, overlap int) []models.CVChunk {
	if text == "" {
		return nil
	}
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if overlap <= 0 {
		overlap = DefaultChunkOverlap
	}

	var chunks []models.CVChunk
	start := 0
	index := 0

	for start < len(text) {
		end := start + chunkSize
		if end > len(text) {
			end = len(text)
		}

		segment := text[start:end]

		// Try to break at word boundary if not at end
		if end < len(text) && chunkSize > 100 {
			// Search last 50 chars for space or newline
			searchStart := len(segment) - 50
			if searchStart < 0 {
				searchStart = 0
			}
			lastSpace := strings.LastIndexAny(segment[searchStart:], " \n")
			if lastSpace >= 0 {
				breakPos := searchStart + lastSpace + 1
				if breakPos > chunkSize-100 {
					end = start + breakPos
					segment = text[start:end]
				}
			}
		}

		chunks = append(chunks, models.CVChunk{
			Text:      strings.TrimSpace(segment),
			Index:     index,
			StartChar: start,
			EndChar:   end,
		})

		// Ensure start always advances by at least 1
		next := end - overlap
		if next <= start {
			next = start + 1
		}
		start = next
		index++
		if start >= len(text) {
			break
		}
	}

	slog.Info("chunked text", "chunks", len(chunks), "size", chunkSize, "overlap", overlap)
	return chunks
}

// CreateLockFile creates a lock file to prevent concurrent CV processing.
func CreateLockFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("lock file exists: %s", path)
	}
	return os.WriteFile(path, []byte{}, 0644)
}

// RemoveLockFile removes the lock file.
func RemoveLockFile(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		slog.Warn("error removing lock file", "error", err)
	}
}
