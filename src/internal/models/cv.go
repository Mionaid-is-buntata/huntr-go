package models

// CVProfile represents the extracted profile from a user's CV.
type CVProfile struct {
	Skills     []string `json:"skills"`
	Domains    []string `json:"domains"`
	Experience string   `json:"experience"`
}

// CVChunk represents a segment of CV text for embedding.
type CVChunk struct {
	Text      string    `json:"text"`
	Index     int       `json:"index"`
	StartChar int       `json:"start_char"`
	EndChar   int       `json:"end_char"`
	Embedding []float32 `json:"embedding,omitempty"`
}
