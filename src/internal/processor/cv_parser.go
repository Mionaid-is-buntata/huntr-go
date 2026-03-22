package processor

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"golang.org/x/net/html/charset"
)

// isValidDocx checks if a file is a valid DOCX by checking the ZIP magic bytes.
func isValidDocx(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	header := make([]byte, 4)
	if _, err := f.Read(header); err != nil {
		return false
	}
	return header[0] == 'P' && header[1] == 'K'
}

// readAsPlainText tries multiple encodings to read a file as text.
func readAsPlainText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Try UTF-8 first
	content := string(data)
	if strings.TrimSpace(content) != "" {
		slog.Info("read CV as plain text", "encoding", "utf-8", "chars", len(content))
		return content, nil
	}

	// Try other encodings via charset package
	for _, enc := range []string{"windows-1252", "iso-8859-1"} {
		reader, err := charset.NewReaderLabel(enc, strings.NewReader(string(data)))
		if err != nil {
			continue
		}
		var buf strings.Builder
		if _, err := io.Copy(&buf, reader); err == nil && strings.TrimSpace(buf.String()) != "" {
			slog.Info("read CV as plain text", "encoding", enc, "chars", buf.Len())
			return buf.String(), nil
		}
	}

	return "", fmt.Errorf("could not read %s with any encoding", path)
}

// docxBody represents the document.xml body structure.
type docxBody struct {
	Paragraphs []docxParagraph `xml:"body>p"`
	Tables     []docxTable     `xml:"body>tbl"`
}

type docxParagraph struct {
	Runs []docxRun `xml:"r"`
}

type docxRun struct {
	Text string `xml:"t"`
}

type docxTable struct {
	Rows []docxRow `xml:"tr"`
}

type docxRow struct {
	Cells []docxCell `xml:"tc"`
}

type docxCell struct {
	Paragraphs []docxParagraph `xml:"p"`
}

// ParseCVDocx parses a DOCX file and extracts text.
// Falls back to plain text if DOCX parsing fails.
func ParseCVDocx(path string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("cv file not found: %s", path)
	}

	if !isValidDocx(path) {
		slog.Warn("file does not appear to be valid DOCX, trying plain text", "path", path)
		return readAsPlainText(path)
	}

	r, err := zip.OpenReader(path)
	if err != nil {
		slog.Warn("DOCX open failed, trying plain text", "error", err)
		return readAsPlainText(path)
	}
	defer r.Close()

	// Find word/document.xml
	var docFile *zip.File
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		slog.Warn("no word/document.xml found, trying plain text")
		return readAsPlainText(path)
	}

	rc, err := docFile.Open()
	if err != nil {
		return "", fmt.Errorf("open document.xml: %w", err)
	}
	defer rc.Close()

	var body docxBody
	if err := xml.NewDecoder(rc).Decode(&body); err != nil {
		slog.Warn("XML decode failed, trying plain text", "error", err)
		return readAsPlainText(path)
	}

	var parts []string

	// Extract paragraph text
	for _, p := range body.Paragraphs {
		var texts []string
		for _, run := range p.Runs {
			if run.Text != "" {
				texts = append(texts, run.Text)
			}
		}
		if line := strings.TrimSpace(strings.Join(texts, "")); line != "" {
			parts = append(parts, line)
		}
	}

	// Extract table text
	for _, tbl := range body.Tables {
		for _, row := range tbl.Rows {
			var cells []string
			for _, cell := range row.Cells {
				var cellTexts []string
				for _, p := range cell.Paragraphs {
					for _, run := range p.Runs {
						if run.Text != "" {
							cellTexts = append(cellTexts, run.Text)
						}
					}
				}
				if ct := strings.TrimSpace(strings.Join(cellTexts, "")); ct != "" {
					cells = append(cells, ct)
				}
			}
			if len(cells) > 0 {
				parts = append(parts, strings.Join(cells, " | "))
			}
		}
	}

	fullText := strings.Join(parts, "\n")
	if strings.TrimSpace(fullText) == "" {
		slog.Warn("DOCX parsing produced empty text, trying plain text")
		return readAsPlainText(path)
	}

	slog.Info("parsed CV as DOCX", "chars", len(fullText))
	return fullText, nil
}
