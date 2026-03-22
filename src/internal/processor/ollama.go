package processor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/mem"
)

const (
	primaryModel     = "llama2-uncensored"
	altPrimaryModel  = "mistral:7b-q4_K_M"
	fallbackModel    = "phi:2.7b"
	primaryRAMNeeded = 5.0 // GB
)

// OllamaClient holds the selected model and host configuration.
type OllamaClient struct {
	Model  string
	Host   string
	Reason string
}

func ollamaHost() string {
	if h := os.Getenv("OLLAMA_HOST"); h != "" {
		return h
	}
	return "host.docker.internal:11434"
}

// checkRAMAvailable returns available RAM in MB.
func checkRAMAvailable() float64 {
	v, err := mem.VirtualMemory()
	if err != nil {
		slog.Warn("could not check RAM", "error", err)
		return 0
	}
	mb := float64(v.Available) / (1024 * 1024)
	slog.Info("available RAM", "mb", int(mb))
	return mb
}

// checkModelAvailable queries Ollama API to see if a model is loaded.
func checkModelAvailable(model, host string) bool {
	url := fmt.Sprintf("http://%s/api/tags", host)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if json.NewDecoder(resp.Body).Decode(&result) != nil {
		return false
	}
	for _, m := range result.Models {
		if m.Name == model {
			return true
		}
	}
	return false
}

// SelectModel chooses the best available LLM model based on RAM and availability.
func SelectModel() (*OllamaClient, error) {
	host := ollamaHost()
	ramMB := checkRAMAvailable()
	ramGB := ramMB / 1024

	for _, model := range []string{primaryModel, altPrimaryModel} {
		if checkModelAvailable(model, host) {
			if ramGB >= primaryRAMNeeded {
				slog.Info("selected primary model", "model", model, "ram_gb", ramGB)
				return &OllamaClient{
					Model:  model,
					Host:   host,
					Reason: fmt.Sprintf("Primary model with sufficient RAM (%.1fGB)", ramGB),
				}, nil
			}
			slog.Warn("insufficient RAM for primary model", "model", model, "ram_gb", ramGB)
			break
		}
	}

	if checkModelAvailable(fallbackModel, host) {
		return &OllamaClient{
			Model:  fallbackModel,
			Host:   host,
			Reason: "Fallback model (RAM pressure or primary unavailable)",
		}, nil
	}

	return nil, fmt.Errorf("no Ollama models available (%s/%s/%s). Install with: ollama pull %s",
		primaryModel, altPrimaryModel, fallbackModel, primaryModel)
}

// GenerateEmbeddings calls the Ollama API to generate embeddings for text chunks.
// This replaces the Python sentence-transformers approach.
func GenerateEmbeddings(chunks []CVChunkWithEmbedding, model, host string) error {
	url := fmt.Sprintf("http://%s/api/embeddings", host)
	client := &http.Client{Timeout: 60 * time.Second}

	slog.Info("generating embeddings", "chunks", len(chunks), "model", model)

	for i := range chunks {
		if err := func() error {
			body, _ := json.Marshal(map[string]string{
				"model":  model,
				"prompt": chunks[i].Text,
			})

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				return fmt.Errorf("embedding request %d: %w", i, err)
			}
			defer resp.Body.Close()

			var result struct {
				Embedding []float32 `json:"embedding"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return fmt.Errorf("embedding decode %d: %w", i, err)
			}

			chunks[i].Embedding = result.Embedding
			return nil
		}(); err != nil {
			return err
		}
	}

	slog.Info("embeddings generated", "count", len(chunks))
	return nil
}

// CVChunkWithEmbedding extends CVChunk with embedding data for processing.
type CVChunkWithEmbedding struct {
	Text      string    `json:"text"`
	Index     int       `json:"index"`
	StartChar int       `json:"start_char"`
	EndChar   int       `json:"end_char"`
	Embedding []float32 `json:"embedding,omitempty"`
}

const profileExtractionPrompt = `Extract the following information from this CV/resume:

1. **Skills**: List all technical skills, programming languages, frameworks, tools
2. **Domains**: List all industry domains/areas of expertise (e.g., Transport, Ticketing, Payments, Hardware Integration)
3. **Experience**: Summarize years of experience and key roles

Return the response as valid JSON with this structure:
{
  "skills": ["skill1", "skill2", ...],
  "domains": ["domain1", "domain2", ...],
  "experience": "summary text"
}

CV Text:
%s`

// ExtractProfile calls Ollama to extract skills, domains, and experience from CV text.
func ExtractProfile(cvText string, client *OllamaClient) (*CVProfile, error) {
	// Limit text length
	if len(cvText) > 5000 {
		cvText = cvText[:5000]
	}

	prompt := fmt.Sprintf(profileExtractionPrompt, cvText)
	url := fmt.Sprintf("http://%s/api/generate", client.Host)

	body, _ := json.Marshal(map[string]interface{}{
		"model":  client.Model,
		"prompt": prompt,
		"options": map[string]interface{}{
			"temperature": 0.1,
			"num_predict": 500,
		},
		"stream": false,
	})

	httpClient := &http.Client{Timeout: 120 * time.Second}
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama decode: %w", err)
	}

	// Extract JSON from response (may have markdown code blocks)
	text := result.Response
	if idx := strings.Index(text, "```json"); idx >= 0 {
		text = text[idx+7:]
		if end := strings.Index(text, "```"); end >= 0 {
			text = text[:end]
		}
	} else if idx := strings.Index(text, "```"); idx >= 0 {
		text = text[idx+3:]
		if end := strings.Index(text, "```"); end >= 0 {
			text = text[:end]
		}
	}
	text = strings.TrimSpace(text)

	var profile CVProfile
	if err := json.Unmarshal([]byte(text), &profile); err != nil {
		return nil, fmt.Errorf("parse profile JSON: %w", err)
	}

	if profile.Skills == nil {
		profile.Skills = []string{}
	}
	if profile.Domains == nil {
		profile.Domains = []string{}
	}

	slog.Info("extracted CV profile", "skills", len(profile.Skills), "domains", len(profile.Domains))
	return &profile, nil
}

// CVProfile holds the extracted profile from a CV.
type CVProfile struct {
	Skills     []string `json:"skills"`
	Domains    []string `json:"domains"`
	Experience string   `json:"experience"`
}

// SaveProfile writes the profile to a JSON file.
func SaveProfile(profile *CVProfile, path string) error {
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}
