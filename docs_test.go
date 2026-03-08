package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type markdownCodeBlock struct {
	Language string
	Label    string
	Content  string
	Heading  string
	Source   string
}

func TestDocumentationExamplesDryRun(t *testing.T) {
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	docPaths := []string{
		filepath.Join(repoRoot, "README.md"),
		filepath.Join(repoRoot, "docs", "EXAMPLES.md"),
	}

	blocks := loadMarkdownCodeBlocks(t, docPaths)
	examples := documentationExamples(blocks)
	if len(examples) == 0 {
		t.Fatal("expected at least one documentation example")
	}

	workDir := t.TempDir()
	writeDocumentationFixtures(t, workDir, blocks)

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "prompt2json")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("failed to build prompt2json for docs validation: %v\n%s", err, output)
	}

	for i, example := range examples {
		t.Run(fmt.Sprintf("%02d_%s", i+1, example.Heading), func(t *testing.T) {
			commandText := prepareDocumentationCommand(example.Content)
			cmd := exec.Command("zsh", "-lc", commandText)
			cmd.Dir = workDir
			cmd.Env = append(os.Environ(),
				"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
				"OPENAI_API_KEY=test-api-key",
				"GOOGLE_CLOUD_PROJECT=example-project",
			)

			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("documentation example failed dry-run\nsource: %s\nheading: %s\ncommand:\n%s\noutput:\n%s\nerror: %v", example.Source, example.Heading, commandText, output, err)
			}

			if strings.Contains(commandText, "--out summary.json") {
				if _, err := os.Stat(filepath.Join(workDir, "summary.json")); err != nil {
					t.Fatalf("expected summary.json to be written by documentation example: %v", err)
				}
			}
		})
	}
}

func loadMarkdownCodeBlocks(t *testing.T, paths []string) []markdownCodeBlock {
	t.Helper()

	var blocks []markdownCodeBlock
	for _, path := range paths {
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read markdown file %s: %v", path, err)
		}
		blocks = append(blocks, parseMarkdownCodeBlocks(string(contentBytes), path)...)
	}
	return blocks
}

func parseMarkdownCodeBlocks(content string, source string) []markdownCodeBlock {
	lines := strings.Split(content, "\n")

	var (
		blocks           []markdownCodeBlock
		headingStack     []string
		inCodeBlock      bool
		codeLanguage     string
		codeLabel        string
		codeHeading      string
		codeLines        []string
		previousNonEmpty string
	)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if inCodeBlock {
			if trimmed == "```" {
				blocks = append(blocks, markdownCodeBlock{
					Language: codeLanguage,
					Label:    codeLabel,
					Content:  strings.Join(codeLines, "\n"),
					Heading:  codeHeading,
					Source:   source,
				})
				inCodeBlock = false
				codeLanguage = ""
				codeLabel = ""
				codeHeading = ""
				codeLines = nil
				continue
			}

			codeLines = append(codeLines, line)
			continue
		}

		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = true
			codeLanguage = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			codeLabel = markdownLabel(previousNonEmpty)
			if codeLabel != "" {
				previousNonEmpty = ""
			}
			codeHeading = strings.Join(headingStack, " / ")
			codeLines = nil
			continue
		}

		if level, heading := markdownHeading(trimmed); level > 0 {
			if len(headingStack) >= level {
				headingStack = headingStack[:level-1]
			}
			headingStack = append(headingStack, heading)
		}

		if trimmed != "" {
			previousNonEmpty = trimmed
		}
	}

	return blocks
}

func markdownHeading(line string) (int, string) {
	if !strings.HasPrefix(line, "#") {
		return 0, ""
	}

	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == len(line) || line[level] != ' ' {
		return 0, ""
	}
	return level, strings.TrimSpace(line[level+1:])
}

func markdownLabel(line string) string {
	if len(line) < 2 || line[0] != '`' || line[len(line)-1] != '`' {
		return ""
	}
	return strings.Trim(line, "`")
}

func documentationExamples(blocks []markdownCodeBlock) []markdownCodeBlock {
	var examples []markdownCodeBlock
	for _, block := range blocks {
		if block.Language != "bash" {
			continue
		}
		if !isPrompt2JSONCommand(block.Content) {
			continue
		}
		examples = append(examples, block)
	}
	return examples
}

func writeDocumentationFixtures(t *testing.T, workDir string, blocks []markdownCodeBlock) {
	t.Helper()

	for _, block := range blocks {
		if block.Label == "" {
			continue
		}
		if !strings.HasSuffix(block.Label, ".json") && !strings.HasSuffix(block.Label, ".txt") {
			continue
		}

		path := filepath.Join(workDir, block.Label)
		if err := os.WriteFile(path, []byte(block.Content), 0600); err != nil {
			t.Fatalf("failed to write fixture %s: %v", path, err)
		}
	}

	extraTextFixtures := map[string]string{
		"ticket.txt": "User cannot access dashboard after login and needs help regaining access.",
	}
	for name, content := range extraTextFixtures {
		path := filepath.Join(workDir, name)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatalf("failed to write fixture %s: %v", path, err)
		}
	}

	extraBinaryFixtures := map[string][]byte{
		"picture.png": []byte("not-a-real-png"),
		"resume.pdf":  []byte("%PDF-1.4\nmock pdf\n"),
	}
	for name, content := range extraBinaryFixtures {
		path := filepath.Join(workDir, name)
		if err := os.WriteFile(path, content, 0600); err != nil {
			t.Fatalf("failed to write fixture %s: %v", path, err)
		}
	}
}

func prepareDocumentationCommand(command string) string {
	command = strings.TrimSpace(command)
	command = strings.ReplaceAll(command, "$(gcloud auth application-default print-access-token)", "test-access-token")

	if !strings.Contains(command, "--show-url") && !strings.Contains(command, "--show-request-body") {
		command += " \\\n    --show-url"
	}

	return command
}

func isPrompt2JSONCommand(command string) bool {
	command = strings.TrimSpace(command)
	return strings.HasPrefix(command, "prompt2json ") ||
		strings.HasPrefix(command, "prompt2json \\") ||
		strings.Contains(command, "| prompt2json ")
}
