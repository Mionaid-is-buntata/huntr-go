package web

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"time"

	"github.com/campbell/huntr-ai/internal/config"
	chromem "github.com/philippgille/chromem-go"
)

const vectorDBPath = "/data/chromadb"

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

// createdAtFromName extracts a formatted timestamp from a collection name.
func createdAtFromName(name string) string {
	matches := cvTimestampRe.FindStringSubmatch(name)
	if matches == nil {
		return ""
	}
	t, err := time.Parse("20060102150405",
		matches[1]+matches[2]+matches[3]+matches[4]+matches[5]+matches[6])
	if err != nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// openVectorDB opens the chromem-go persistent database.
// Caller should use the returned DB briefly and not hold it long-term.
func openVectorDB() (*chromem.DB, error) {
	if _, err := os.Stat(vectorDBPath); err != nil {
		return nil, fmt.Errorf("vector DB path not found: %s", vectorDBPath)
	}
	db, err := chromem.NewPersistentDB(vectorDBPath, false)
	if err != nil {
		return nil, fmt.Errorf("open vector DB: %w", err)
	}
	return db, nil
}

// GetActiveCollection reads the active collection name from config.
// If none is set, defaults to the latest collection name.
func GetActiveCollection(cfg *config.Config) string {
	if cfg.CV.VectorDB.ActiveCollection != "" {
		return cfg.CV.VectorDB.ActiveCollection
	}
	// Default to latest collection
	db, err := openVectorDB()
	if err != nil {
		return ""
	}
	collections := db.ListCollections()
	if len(collections) == 0 {
		return ""
	}
	names := make([]string, 0, len(collections))
	for name := range collections {
		names = append(names, name)
	}
	sort.Strings(names)
	return names[len(names)-1]
}

// SetActiveCollection updates the active collection in config.
func SetActiveCollection(cfg *config.Config, name string, configPath string) bool {
	cfg.CV.VectorDB.ActiveCollection = name
	slog.Info("set active collection", "name", name)
	return config.Save(configPath, cfg) == nil
}

// DeleteCollection deletes a ChromaDB collection.
func DeleteCollection(name string) bool {
	db, err := openVectorDB()
	if err != nil {
		slog.Error("failed to open vector DB for deletion", "error", err)
		return false
	}
	if err := db.DeleteCollection(name); err != nil {
		slog.Error("failed to delete collection", "collection", name, "error", err)
		return false
	}
	slog.Info("deleted collection", "collection", name)
	return true
}

// GetCollections lists all collections with metadata.
func GetCollections(cfg *config.Config) []CollectionInfo {
	db, err := openVectorDB()
	if err != nil {
		slog.Debug("could not open vector DB", "error", err)
		return nil
	}

	collections := db.ListCollections()
	if len(collections) == 0 {
		return nil
	}

	active := cfg.CV.VectorDB.ActiveCollection

	var result []CollectionInfo
	for name, col := range collections {
		info := CollectionInfo{
			Name:         name,
			FriendlyName: GenerateFriendlyName(name),
			CreatedAt:    createdAtFromName(name),
			Count:        col.Count(),
			Active:       name == active,
		}
		result = append(result, info)
	}

	// Sort by name (chronological due to timestamp format)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	// If no active collection is set, mark the latest as active
	if active == "" && len(result) > 0 {
		result[len(result)-1].Active = true
	}

	return result
}
