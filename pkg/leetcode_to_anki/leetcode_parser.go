package leetcode_to_anki

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"encoding/base64"

	"github.com/PuerkitoBio/goquery"
)

type CodeLine struct {
	Text     string
	TopValue int
}

// Image represents an image in the problem description
type Image struct {
	Filename string
	Data     []byte
}

// Problem represents a LeetCode problem with its description and code
type Problem struct {
	Title       string
	Description string
	Code        string
	Images      []*Image
}

// FormatDescription formats the problem description for Anki cards
func FormatDescription(description string, images []*Image) string {
	// Keep the original problem HTML intact. LeetCode uses inline <code> in
	// examples and <pre><code> for longer samples; flattening to text loses both.
	_ = images // image src values are already rewritten by extractImages.
	return `<style>
.leetcode-description { text-align: left; line-height: 1.55; }
.leetcode-description p { margin: 10px 0; }
.leetcode-description code {
    font-family: 'Courier New', monospace !important;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    background: #353535 !important;
    color: #eff1f6 !important;
    border: 0 !important;
    border-radius: 4px;
    padding: 2px 5px !important;
}
.leetcode-description pre {
    display: block;
    max-width: 100%;
    overflow-x: auto;
    white-space: pre !important;
    tab-size: 4;
    background: #282828 !important;
    color: #eff1f6 !important;
    border: 1px solid #3e3e3e !important;
    border-radius: 8px;
    padding: 14px !important;
    font: 14px/1.5 'Courier New', monospace !important;
}
.leetcode-description pre code {
    display: block;
    background: transparent !important;
    padding: 0 !important;
    border: 0 !important;
    white-space: inherit;
}
.night_mode .leetcode-description code { background: #353535 !important; color: #eff1f6 !important; }
.night_mode .leetcode-description pre { background: #282828 !important; color: #eff1f6 !important; }
</style>
<div class="description leetcode-description">` + description + `</div>`
}

func ExtractCodeFromHTML(htmlContent string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("error parsing HTML: %v", err)
	}

	var lines []CodeLine

	// Try different selectors for code containers
	codeContainers := []string{
		"div.view-lines.monaco-mouse-cursor-text",
		"div.view-lines",
		"div[role='presentation'] div.view-line",
		"div.monaco-scrollable-element div.view-line",
	}

	var viewLinesDiv *goquery.Selection
	for _, selector := range codeContainers {
		viewLinesDiv = doc.Find(selector)
		if viewLinesDiv.Length() > 0 {
			break
		}
	}

	if viewLinesDiv.Length() > 0 {
		viewLinesDiv.Find("div.view-line").Each(func(i int, lineDiv *goquery.Selection) {
			// Get top value
			style, exists := lineDiv.Attr("style")
			topValue := 0
			if exists {
				re := regexp.MustCompile(`top:(\d+)px`)
				if matches := re.FindStringSubmatch(style); len(matches) > 1 {
					if val, err := strconv.Atoi(matches[1]); err == nil {
						topValue = val
					}
				}
			}

			// Get the raw HTML content of the line
			lineHTML, _ := lineDiv.Html()

			// Remove span tags and their attributes first
			lineHTML = regexp.MustCompile(`<span[^>]*>`).ReplaceAllString(lineHTML, "")
			lineHTML = strings.ReplaceAll(lineHTML, "</span>", "")

			// Get the text content and clean it
			text := strings.TrimRight(lineHTML, " \t\r\n")
			text = strings.ReplaceAll(text, "\u00a0", " ") // Replace non-breaking spaces
			text = html.UnescapeString(text)               // Unescape any HTML entities in the code
			trimmed := strings.TrimSpace(text)

			// Skip empty lines and lines starting with #
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				// Preserve indentation by counting leading spaces
				leadingSpaces := 0
				for _, r := range text {
					if r == ' ' {
						leadingSpaces++
					} else {
						break
					}
				}

				// Reconstruct the line with proper indentation
				indentedText := strings.Repeat(" ", leadingSpaces) + strings.TrimSpace(text)
				lines = append(lines, CodeLine{
					Text:     indentedText,
					TopValue: topValue,
				})
			}
		})
	}

	// If no lines were found, try alternative extraction method
	if len(lines) == 0 {
		doc.Find("div[role='presentation']").Each(func(i int, codeDiv *goquery.Selection) {
			text := codeDiv.Text()
			text = html.UnescapeString(text) // Unescape any HTML entities in the code
			if strings.TrimSpace(text) != "" {
				lines = append(lines, CodeLine{
					Text:     text,
					TopValue: i * 20, // Approximate line spacing
				})
			}
		})
	}

	// Sort lines by top value
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].TopValue < lines[j].TopValue
	})

	// Build final code with proper spacing
	var codeBuilder strings.Builder
	lastTop := -1

	for _, line := range lines {
		// Add blank line if there's a significant gap
		if lastTop != -1 && line.TopValue-lastTop > 20 {
			codeBuilder.WriteString("\n")
		}

		codeBuilder.WriteString(line.Text)
		codeBuilder.WriteString("\n")
		lastTop = line.TopValue
	}

	code := strings.TrimSpace(codeBuilder.String()) + "\n"

	// Final cleanup of any remaining HTML entities
	code = html.UnescapeString(code)

	// Ensure proper line endings and spacing
	codeLines := strings.Split(code, "\n")
	var cleanedLines []string
	for _, line := range codeLines {
		// Preserve indentation by counting leading spaces
		leadingSpaces := 0
		for _, r := range line {
			if r == ' ' {
				leadingSpaces++
			} else {
				break
			}
		}
		// Reconstruct the line with proper indentation
		cleanedLine := strings.Repeat(" ", leadingSpaces) + strings.TrimSpace(line)
		cleanedLines = append(cleanedLines, cleanedLine)
	}
	code = strings.Join(cleanedLines, "\n") + "\n"

	return code, nil
}

func cleanCode(code string) string {
	// Fix common replacements
	code = strings.ReplaceAll(code, "→", "->")
	code = strings.ReplaceAll(code, `"`, `"`)
	code = strings.ReplaceAll(code, `"`, `"`)
	code = strings.ReplaceAll(code, "\u00a0", " ")

	// Split into lines for final cleaning
	lines := strings.Split(code, "\n")
	var cleanedLines []string
	var lastLineWasEmpty bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if !lastLineWasEmpty {
				cleanedLines = append(cleanedLines, "")
				lastLineWasEmpty = true
			}
		} else if !strings.HasPrefix(trimmed, "#") {
			cleanedLines = append(cleanedLines, line)
			lastLineWasEmpty = false
		}
	}

	// Remove trailing empty lines
	for len(cleanedLines) > 0 && cleanedLines[len(cleanedLines)-1] == "" {
		cleanedLines = cleanedLines[:len(cleanedLines)-1]
	}

	// Join lines and ensure trailing newline
	code = strings.Join(cleanedLines, "\n")
	if !strings.HasSuffix(code, "\n") {
		code += "\n"
	}

	return code
}

// extractImages extracts images from the problem description HTML
func extractImages(doc *goquery.Document) ([]*Image, error) {
	var images []*Image
	var err error

	// Find all img tags in the description content
	doc.Find("div[data-track-load='description_content'] img").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists {
			return
		}

		var imageData []byte
		var filename string

		// Handle base64 encoded images
		if strings.HasPrefix(src, "data:image/") {
			// Extract base64 data
			parts := strings.Split(src, ",")
			if len(parts) != 2 {
				return
			}

			// Determine image type from the data URI
			mimeType := strings.Split(strings.Split(parts[0], ";")[0], ":")[1]
			ext := strings.Split(mimeType, "/")[1]

			// Decode base64 data
			imageData, err = base64.StdEncoding.DecodeString(parts[1])
			if err != nil {
				return
			}

			// Generate a unique filename
			filename = fmt.Sprintf("image_%d.%s", i, ext)

			// Replace the base64 data with just the filename
			s.SetAttr("src", filename)
		} else if strings.HasPrefix(src, "http") {
			// Handle remote images
			resp, err := http.Get(src)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			imageData, err = io.ReadAll(resp.Body)
			if err != nil {
				return
			}

			// Extract filename from URL or generate one
			filename = filepath.Base(src)
			if filename == "" || filename == "." {
				ext := ".png" // Default extension
				contentType := resp.Header.Get("Content-Type")
				if strings.Contains(contentType, "jpeg") {
					ext = ".jpg"
				} else if strings.Contains(contentType, "gif") {
					ext = ".gif"
				}
				filename = fmt.Sprintf("image_%d%s", i, ext)
			}

			// Update the src attribute to use the filename
			s.SetAttr("src", filename)
		}

		if imageData != nil && filename != "" {
			images = append(images, &Image{
				Filename: filename,
				Data:     imageData,
			})
		}
	})

	return images, nil
}

// ParseDirectory parses all HTML files in the given directory and returns a slice of problems
func ParseDirectory(inputDir string, debug bool) ([]*Problem, error) {
	var problems []*Problem

	files, err := os.ReadDir(inputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %v", err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(strings.ToLower(file.Name()), ".html") {
			filePath := filepath.Join(inputDir, file.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %v", file.Name(), err)
			}

			doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(content)))
			if err != nil {
				return nil, fmt.Errorf("failed to parse HTML file %s: %v", file.Name(), err)
			}

			// Extract title
			title := doc.Find("title").Text()
			title = strings.TrimSuffix(title, " - LeetCode")

			// Extract description HTML
			descriptionDiv := doc.Find("div[data-track-load='description_content']")
			if descriptionDiv.Length() == 0 {
				return nil, fmt.Errorf("description content not found in %s", file.Name())
			}

			// Extract images before modifying the HTML
			images, err := extractImages(doc)
			if err != nil {
				return nil, fmt.Errorf("failed to extract images from %s: %v", file.Name(), err)
			}

			// Keep the original markup so inline and block code remain actual code
			// elements in Anki. Remove only page controls and embedded styles/scripts.
			descriptionDiv.Find("[data-example-lang-controls], script, style").Remove()
			description, err := descriptionDiv.Html()
			if err != nil {
				return nil, fmt.Errorf("failed to get description HTML from %s: %v", file.Name(), err)
			}


			// Extract code
			codeHTML, err := doc.Find("div.view-lines").Parent().Html()
			if err != nil {
				return nil, fmt.Errorf("failed to get code HTML from %s: %v", file.Name(), err)
			}

			code, err := ExtractCodeFromHTML(codeHTML)
			if err != nil {
				return nil, fmt.Errorf("failed to extract code from %s: %v", file.Name(), err)
			}

			// Format description with images
			formattedDescription := FormatDescription(description, images)

			problem := &Problem{
				Title:       title,
				Description: formattedDescription,
				Code:        code,
				Images:      images,
			}

			problems = append(problems, problem)

			if debug {
				fmt.Printf("Processed %s:\nTitle: %s\nDescription length: %d\nCode length: %d\nImages: %d\n\n",
					file.Name(), problem.Title, len(problem.Description), len(problem.Code), len(problem.Images))
			}
		}
	}

	return problems, nil
}
