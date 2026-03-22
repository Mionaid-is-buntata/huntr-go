package web

import (
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/campbell/huntr-ai/internal/config"
)

var cvTimestampRe = regexp.MustCompile(`^cv_(\d{4})(\d{2})(\d{2})_(\d{2})(\d{2})(\d{2})$`)

// CollectionInfo represents a ChromaDB collection with metadata.
type CollectionInfo struct {
	Name         string `json:"name"`
	FriendlyName string `json:"friendly_name"`
	CreatedAt    string `json:"created_at"`
	Count        int    `json:"count"`
	Active       bool   `json:"active"`
}

// GenerateFriendlyName converts "cv_20260127_143000" to "CV - 27/01/2026 14:30".
func GenerateFriendlyName(name string) string {
	matches := cvTimestampRe.FindStringSubmatch(name)
	if matches == nil {
		return name
	}
	t, err := time.Parse("20060102150405",
		matches[1]+matches[2]+matches[3]+matches[4]+matches[5]+matches[6])
	if err != nil {
		return name
	}
	return fmt.Sprintf("CV - %s", t.Format("02/01/2006 15:04"))
}

// GetActiveCollection reads the active collection name from config.
func GetActiveCollection(cfg *config.Config) string {
	// Active collection stored in cv.vector_db — we access via the VectorConfig struct.
	// The Python code stores it as a separate key "active_collection" in vector_db.
	// For Go we'll add it to VectorConfig when needed. For now, return empty.
	return ""
}

// SetActiveCollection updates the active collection in config.
func SetActiveCollection(cfg *config.Config, name string, configPath string) bool {
	// Will be fully implemented when chromem-go is integrated in Phase 2.
	slog.Info("set active collection", "name", name)
	return config.Save(configPath, cfg) == nil
}

// DeleteCollection deletes a ChromaDB collection.
// Placeholder — full implementation in Phase 2 with chromem-go.
func DeleteCollection(name string) bool {
	slog.Warn("DeleteCollection: chromem-go not yet integrated", "collection", name)
	return false
}

// GetCollections lists all collections.
// Placeholder — full implementation in Phase 2 with chromem-go.
func GetCollections(cfg *config.Config) []CollectionInfo {
	slog.Debug("GetCollections: chromem-go not yet integrated")
	return nil
}
