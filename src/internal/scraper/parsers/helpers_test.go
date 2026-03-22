package parsers

import (
	"testing"
)

func TestExtractSalaryNumber(t *testing.T) {
	tests := []struct {
		input string
		want  *int
	}{
		{"£60,000", intPtr(60000)},
		{"£60k", intPtr(60000)},
		{"60000", intPtr(60000)},
		{"£50,000 - £60,000", intPtr(50000)},
		{"£75k - £90k", intPtr(75000)},
		{"", nil},
		{"Competitive", nil},
		{"Not specified", nil},
	}

	for _, tt := range tests {
		got := ExtractSalaryNumber(tt.input)
		if tt.want == nil {
			if got != nil {
				t.Errorf("ExtractSalaryNumber(%q) = %d, want nil", tt.input, *got)
			}
		} else if got == nil {
			t.Errorf("ExtractSalaryNumber(%q) = nil, want %d", tt.input, *tt.want)
		} else if *got != *tt.want {
			t.Errorf("ExtractSalaryNumber(%q) = %d, want %d", tt.input, *got, *tt.want)
		}
	}
}

func TestExtractSkillsFromText(t *testing.T) {
	text := "We need a Python developer with React and Docker experience. Must know AWS and PostgreSQL."
	skills := ExtractSkillsFromText(text)

	for _, want := range []string{"Python", "React", "Docker", "AWS", "PostgreSQL"} {
		if !containsSkill(skills, want) {
			t.Errorf("ExtractSkillsFromText: missing %q in %q", want, skills)
		}
	}
}

func TestExtractSkillsFromTextEmpty(t *testing.T) {
	if got := ExtractSkillsFromText(""); got != "" {
		t.Errorf("ExtractSkillsFromText('') = %q, want empty", got)
	}
}

func TestExtractResponsibilitiesFromText(t *testing.T) {
	text := "About us. We are great. Responsibilities You will build APIs. You will design systems. Requirements Python."
	got := ExtractResponsibilitiesFromText(text)
	if got == "" {
		t.Error("expected non-empty responsibilities")
	}
	if len(got) > 500 {
		t.Errorf("responsibilities too long: %d chars", len(got))
	}
}

func TestExtractResponsibilitiesFallback(t *testing.T) {
	text := "You will build scalable systems. You will design APIs."
	got := ExtractResponsibilitiesFromText(text)
	if got == "" {
		t.Error("expected fallback responsibilities extraction")
	}
}

func TestResolveURL(t *testing.T) {
	tests := []struct {
		base, href, want string
	}{
		{"https://example.com/jobs", "/job/123", "https://example.com/job/123"},
		{"https://example.com/jobs", "https://other.com/job", "https://other.com/job"},
		{"https://example.com", "", ""},
	}
	for _, tt := range tests {
		got := ResolveURL(tt.base, tt.href)
		if got != tt.want {
			t.Errorf("ResolveURL(%q, %q) = %q, want %q", tt.base, tt.href, got, tt.want)
		}
	}
}

func TestDetectWorkType(t *testing.T) {
	tests := []struct {
		text, want string
	}{
		{"This is a remote position", "Remote"},
		{"Hybrid working available", "Hybrid"},
		{"Remote and hybrid options", "Hybrid"},
		{"Office based in London", ""},
	}
	for _, tt := range tests {
		got := DetectWorkType(tt.text)
		if got != tt.want {
			t.Errorf("DetectWorkType(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}

func containsSkill(skills, want string) bool {
	for _, s := range splitSkills(skills) {
		if s == want {
			return true
		}
	}
	return false
}

func splitSkills(s string) []string {
	if s == "" {
		return nil
	}
	parts := make([]string, 0)
	for _, p := range splitByComma(s) {
		p = trimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func splitByComma(s string) []string {
	result := make([]string, 0)
	start := 0
	for i, c := range s {
		if c == ',' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && s[i] == ' ' {
		i++
	}
	for j > i && s[j-1] == ' ' {
		j--
	}
	return s[i:j]
}

func intPtr(v int) *int { return &v }
