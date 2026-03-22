package parsers

import (
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ExtractText returns trimmed text content from a goquery selection.
func ExtractText(s *goquery.Selection) string {
	if s == nil || s.Length() == 0 {
		return ""
	}
	return strings.TrimSpace(s.Text())
}

// ResolveURL resolves a potentially relative href against a base URL.
func ResolveURL(baseURL, href string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(ref).String()
}

var salaryRe = regexp.MustCompile(`£?\s*(\d+(?:\.\d+)?)\s*k?`)

// ExtractSalaryNumber extracts a numeric salary from text like "£50,000 - £60,000" or "£50k".
// Returns the lower bound if a range is given.
func ExtractSalaryNumber(salaryText string) *int {
	if salaryText == "" {
		return nil
	}

	text := strings.ToLower(strings.ReplaceAll(salaryText, ",", ""))

	matches := salaryRe.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}

	val, err := strconv.ParseFloat(matches[0][1], 64)
	if err != nil {
		return nil
	}

	// Check if value is in thousands (has 'k' or value < 1000)
	if strings.Contains(text, "k") || val < 1000 {
		val *= 1000
	}

	result := int(val)
	return &result
}

// techSkills is the list of technical skills to match, with their display names and regex patterns.
var techSkills = []struct {
	Display string
	Pattern *regexp.Regexp
}{
	{"Python", regexp.MustCompile(`(?i)\bpython\b`)},
	{"JavaScript", regexp.MustCompile(`(?i)\bjavascript\b`)},
	{"TypeScript", regexp.MustCompile(`(?i)\btypescript\b`)},
	{"Java", regexp.MustCompile(`(?i)\bjava\b`)},
	{"C#", regexp.MustCompile(`(?i)\bc#\b`)},
	{"C++", regexp.MustCompile(`(?i)\bc\+\+\b`)},
	{"Go", regexp.MustCompile(`(?i)\bgo\b`)},
	{"Golang", regexp.MustCompile(`(?i)\bgolang\b`)},
	{"Rust", regexp.MustCompile(`(?i)\brust\b`)},
	{"Ruby", regexp.MustCompile(`(?i)\bruby\b`)},
	{"PHP", regexp.MustCompile(`(?i)\bphp\b`)},
	{"Swift", regexp.MustCompile(`(?i)\bswift\b`)},
	{"Kotlin", regexp.MustCompile(`(?i)\bkotlin\b`)},
	{"Scala", regexp.MustCompile(`(?i)\bscala\b`)},
	{"React", regexp.MustCompile(`(?i)\breact\b`)},
	{"Angular", regexp.MustCompile(`(?i)\bangular\b`)},
	{"Vue", regexp.MustCompile(`(?i)\bvue\b`)},
	{"Node.js", regexp.MustCompile(`(?i)\bnode\.?js\b`)},
	{"Express", regexp.MustCompile(`(?i)\bexpress\b`)},
	{"Django", regexp.MustCompile(`(?i)\bdjango\b`)},
	{"Flask", regexp.MustCompile(`(?i)\bflask\b`)},
	{"FastAPI", regexp.MustCompile(`(?i)\bfastapi\b`)},
	{"Spring", regexp.MustCompile(`(?i)\bspring\b`)},
	{"Rails", regexp.MustCompile(`(?i)\brails\b`)},
	{"ASP.NET", regexp.MustCompile(`(?i)\basp\.?net\b`)},
	{"Next.js", regexp.MustCompile(`(?i)\bnext\.?js\b`)},
	{"AWS", regexp.MustCompile(`(?i)\baws\b`)},
	{"Azure", regexp.MustCompile(`(?i)\bazure\b`)},
	{"GCP", regexp.MustCompile(`(?i)\bgcp\b`)},
	{"Google Cloud", regexp.MustCompile(`(?i)\bgoogle cloud\b`)},
	{"Kubernetes", regexp.MustCompile(`(?i)\bkubernetes\b`)},
	{"Docker", regexp.MustCompile(`(?i)\bdocker\b`)},
	{"Terraform", regexp.MustCompile(`(?i)\bterraform\b`)},
	{"Jenkins", regexp.MustCompile(`(?i)\bjenkins\b`)},
	{"CI/CD", regexp.MustCompile(`(?i)\bci/?cd\b`)},
	{"PostgreSQL", regexp.MustCompile(`(?i)\bpostgresql?\b`)},
	{"MySQL", regexp.MustCompile(`(?i)\bmysql\b`)},
	{"MongoDB", regexp.MustCompile(`(?i)\bmongodb?\b`)},
	{"Redis", regexp.MustCompile(`(?i)\bredis\b`)},
	{"Elasticsearch", regexp.MustCompile(`(?i)\belasticsearch\b`)},
	{"SQL", regexp.MustCompile(`(?i)\bsql\b`)},
	{"GraphQL", regexp.MustCompile(`(?i)\bgraphql\b`)},
	{"REST", regexp.MustCompile(`(?i)\brest\b`)},
	{"API", regexp.MustCompile(`(?i)\bapi\b`)},
	{"Microservices", regexp.MustCompile(`(?i)\bmicroservices?\b`)},
	{"Linux", regexp.MustCompile(`(?i)\blinux\b`)},
	{"Git", regexp.MustCompile(`(?i)\bgit\b`)},
	{"Agile", regexp.MustCompile(`(?i)\bagile\b`)},
	{"Scrum", regexp.MustCompile(`(?i)\bscrum\b`)},
	{"Machine Learning", regexp.MustCompile(`(?i)\bmachine learning\b`)},
	{"ML", regexp.MustCompile(`(?i)\bml\b`)},
	{"AI", regexp.MustCompile(`(?i)\bai\b`)},
	{"Deep Learning", regexp.MustCompile(`(?i)\bdeep learning\b`)},
	{"NLP", regexp.MustCompile(`(?i)\bnlp\b`)},
	{"TensorFlow", regexp.MustCompile(`(?i)\btensorflow\b`)},
	{"PyTorch", regexp.MustCompile(`(?i)\bpytorch\b`)},
	{"Pandas", regexp.MustCompile(`(?i)\bpandas\b`)},
	{"HTML", regexp.MustCompile(`(?i)\bhtml\b`)},
	{"CSS", regexp.MustCompile(`(?i)\bcss\b`)},
	{"Tailwind", regexp.MustCompile(`(?i)\btailwind\b`)},
	{"Bootstrap", regexp.MustCompile(`(?i)\bbootstrap\b`)},
	{"Kafka", regexp.MustCompile(`(?i)\bkafka\b`)},
	{"RabbitMQ", regexp.MustCompile(`(?i)\brabbitmq\b`)},
	{"OAuth", regexp.MustCompile(`(?i)\boauth\b`)},
	{"JWT", regexp.MustCompile(`(?i)\bjwt\b`)},
}

// ExtractSkillsFromText finds technical skills mentioned in text.
func ExtractSkillsFromText(text string) string {
	if text == "" {
		return ""
	}

	found := make(map[string]bool)
	for _, skill := range techSkills {
		if skill.Pattern.MatchString(text) {
			found[skill.Display] = true
		}
	}

	skills := make([]string, 0, len(found))
	for s := range found {
		skills = append(skills, s)
	}
	sort.Strings(skills)

	if len(skills) > 20 {
		skills = skills[:20]
	}
	return strings.Join(skills, ", ")
}

var responsibilitySplitter = regexp.MustCompile(`(?i)(responsibilities|what you'll do|your role|key duties|the role)`)

var actionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:you will|you'll)\s+[^.]+\.`),
	regexp.MustCompile(`(?i)(?:responsible for)\s+[^.]+\.`),
	regexp.MustCompile(`(?i)(?:develop|design|build|create|implement|manage|lead|support)\s+[^.]+\.`),
}

const maxResponsibilityLen = 500

// ExtractResponsibilitiesFromText extracts responsibility sections from job description text.
func ExtractResponsibilitiesFromText(text string) string {
	if text == "" {
		return ""
	}

	sections := responsibilitySplitter.Split(text, -1)
	headers := responsibilitySplitter.FindAllString(text, -1)

	// Find content after a responsibilities header
	for i, header := range headers {
		if responsibilitySplitter.MatchString(header) && i+1 < len(sections) {
			content := strings.TrimSpace(sections[i+1])
			if len(content) > maxResponsibilityLen {
				content = content[:maxResponsibilityLen]
			}
			return content
		}
	}

	// Fallback: extract sentences with action verbs
	var results []string
	for _, pat := range actionPatterns {
		matches := pat.FindAllString(text, 3)
		results = append(results, matches...)
	}

	result := strings.Join(results, " ")
	if len(result) > maxResponsibilityLen {
		result = result[:maxResponsibilityLen]
	}
	return result
}

// DetectWorkType checks page text for remote/hybrid/on-site indicators.
func DetectWorkType(text string) string {
	lower := strings.ToLower(text)
	hasRemote := strings.Contains(lower, "remote")
	hasHybrid := strings.Contains(lower, "hybrid")

	if hasRemote && hasHybrid {
		return "Hybrid"
	}
	if hasRemote {
		return "Remote"
	}
	if hasHybrid {
		return "Hybrid"
	}
	return ""
}

// FindByClassContaining finds elements where the class attribute contains the given substring.
func FindByClassContaining(s *goquery.Selection, tag, substring string) *goquery.Selection {
	lower := strings.ToLower(substring)
	return s.Find(tag).FilterFunction(func(_ int, sel *goquery.Selection) bool {
		class, exists := sel.Attr("class")
		return exists && strings.Contains(strings.ToLower(class), lower)
	})
}
