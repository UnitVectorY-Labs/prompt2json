package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"golang.org/x/oauth2/google"
)

var Version = "dev" // This will be set by the build systems to the release version

// Schema validation constant
const schemaValidationURL = "schema.json"

// Exit codes
const (
	exitCLIUsageError   = 2
	exitInputError      = 3
	exitValidationError = 4
	exitAPIError        = 5
)

// File size limits
const (
	maxImageSizeBytes = 7 * 1024 * 1024  // 7 MB per image file (before base64 encoding)
	maxTotalSizeBytes = 20 * 1024 * 1024 // ~20 MB total request size limit
)

// CLI flags
var (
	systemInstruction     string
	systemInstructionFile string
	schema                string
	schemaFile            string
	prompt                string
	promptFile            string
	attachments           []string
	outFile               string
	project               string
	location              string
	model                 string
	verbose               bool
	prettyPrint           bool
	showVersion           bool
	showHelp              bool
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(getExitCode(err))
	}
}

func run() error {
	defineFlags()
	flag.Parse()

	if showVersion {
		fmt.Fprintf(os.Stderr, "prompt2json version %s\n", Version)
		return nil
	}

	if showHelp {
		printHelp()
		return nil
	}

	// Validate and load inputs
	config, err := loadConfiguration()
	if err != nil {
		return err
	}

	logVerbose(config, "Configuration loaded successfully")

	// Load attachments
	attachmentParts, err := loadAttachments(config)
	if err != nil {
		return err
	}

	// Build Gemini API request
	requestBody, err := buildGeminiRequest(config, attachmentParts)
	if err != nil {
		return err
	}

	logVerbose(config, "Built Gemini API request")

	// Call Gemini API
	responseJSON, err := callGeminiAPI(config, requestBody)
	if err != nil {
		return err
	}

	logVerbose(config, "Received response from Gemini API")

	// Validate and format the JSON response
	formattedJSON, validationErr := validateAndFormatJSON(config, responseJSON)

	// Write output (always write, even if validation fails)
	if err := writeOutput(config, formattedJSON); err != nil {
		return err
	}

	logVerbose(config, "Output written successfully")

	// Return validation error after writing output (to ensure non-zero exit code)
	if validationErr != nil {
		return validationErr
	}

	return nil
}

func defineFlags() {
	flag.StringVar(&systemInstruction, "system-instruction", "", "System instruction (inline text)")
	flag.StringVar(&systemInstructionFile, "system-instruction-file", "", "System instruction from file")
	flag.StringVar(&schema, "schema", "", "JSON Schema (inline JSON)")
	flag.StringVar(&schemaFile, "schema-file", "", "JSON Schema from file")
	flag.StringVar(&prompt, "prompt", "", "Prompt text (inline)")
	flag.StringVar(&promptFile, "prompt-file", "", "Prompt from file")
	flag.Var((*stringArrayValue)(&attachments), "attach", "Attach file (repeatable)")
	flag.StringVar(&outFile, "out", "", "Output file path (default: STDOUT)")
	flag.StringVar(&project, "project", "", "GCP project ID")
	flag.StringVar(&location, "location", "", "GCP location/region")
	flag.StringVar(&model, "model", "", "Gemini model identifier")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging to STDERR")
	flag.BoolVar(&prettyPrint, "pretty-print", false, "Pretty-print JSON output")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.BoolVar(&showHelp, "help", false, "Show help")
}

type stringArrayValue []string

func (s *stringArrayValue) String() string {
	return strings.Join(*s, ",")
}

func (s *stringArrayValue) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func printHelp() {
	fmt.Fprintf(os.Stderr, `prompt2json - Turn prompts into schema-validated JSON using Vertex AI (Gemini)

Usage:
  prompt2json [OPTIONS]

Required:
  --system-instruction TEXT | --system-instruction-file PATH
  --schema JSON             | --schema-file PATH
  --project ID
  --location REGION
  --model NAME

Input:
  --prompt TEXT              Prompt text (default: read from stdin)
  --prompt-file PATH         Read prompt from file (mutually exclusive with --prompt)
  --attach PATH              Attach file (repeatable): png, jpg/jpeg, webp, pdf

Output:
  --out PATH                 Write JSON to file (default: stdout)
  --pretty-print             Pretty-print JSON output (default: minified)

Misc:
  --verbose                  Log diagnostics to stderr
  --version                  Print version and exit
  --help                     Print help and exit

Environment (used if option not set):
  --project   GOOGLE_CLOUD_PROJECT, CLOUDSDK_CORE_PROJECT
  --location  GOOGLE_CLOUD_LOCATION, GOOGLE_CLOUD_REGION, CLOUDSDK_COMPUTE_REGION

Exit status: 0 success, 2 usage, 3 input, 4 validation/response, 5 API/auth

JSON Processing:
  - LLM responses are validated as parsable JSON
  - Valid JSON is validated against the provided schema
  - JSON is minified by default; use --pretty-print for human-readable output
  - Output is always written regardless of validation results for debugging

Example:
  echo "this is great" | prompt2json \
    --system-instruction "Classify sentiment" \
    --schema '{"type":"object","properties":{"sentiment":{"type":"string","enum":["POSITIVE","NEGATIVE","NEUTRAL"]},"confidence":{"type":"integer","minimum":0,"maximum":100}},"required":["sentiment","confidence"]}' \
    --project example-project \
    --location us-central1 \
    --model gemini-2.5-flash
`)
}

type Config struct {
	SystemInstruction string
	Schema            map[string]interface{}
	SchemaBytes       []byte
	CompiledSchema    *jsonschema.Schema
	Prompt            string
	Project           string
	Location          string
	Model             string
	OutFile           string
	Verbose           bool
	PrettyPrint       bool
}

func loadConfiguration() (*Config, error) {
	config := &Config{
		Verbose:     verbose,
		OutFile:     outFile,
		PrettyPrint: prettyPrint,
	}

	// Load system instruction
	if systemInstruction != "" && systemInstructionFile != "" {
		return nil, &cliError{"cannot specify both --system-instruction and --system-instruction-file"}
	}
	if systemInstruction == "" && systemInstructionFile == "" {
		return nil, &cliError{"must specify either --system-instruction or --system-instruction-file"}
	}

	if systemInstruction != "" {
		config.SystemInstruction = strings.TrimSpace(systemInstruction)
	} else {
		content, err := os.ReadFile(systemInstructionFile)
		if err != nil {
			return nil, &inputError{fmt.Sprintf("failed to read system instruction file: %v", err)}
		}
		config.SystemInstruction = strings.TrimSpace(string(content))
	}

	if config.SystemInstruction == "" {
		return nil, &inputError{"system instruction cannot be empty"}
	}

	// Load schema
	if schema != "" && schemaFile != "" {
		return nil, &cliError{"cannot specify both --schema and --schema-file"}
	}
	if schema == "" && schemaFile == "" {
		return nil, &cliError{"must specify either --schema or --schema-file"}
	}

	var schemaBytes []byte
	if schema != "" {
		schemaBytes = []byte(schema)
	} else {
		content, err := os.ReadFile(schemaFile)
		if err != nil {
			return nil, &inputError{fmt.Sprintf("failed to read schema file: %v", err)}
		}
		schemaBytes = content
	}

	// Parse and validate schema
	if err := json.Unmarshal(schemaBytes, &config.Schema); err != nil {
		return nil, &inputError{fmt.Sprintf("invalid JSON in schema: %v", err)}
	}

	// Store schema bytes for later validation
	config.SchemaBytes = schemaBytes

	// Compile the JSON Schema once for reuse
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020
	if err := compiler.AddResource(schemaValidationURL, bytes.NewReader(schemaBytes)); err != nil {
		return nil, &inputError{fmt.Sprintf("invalid JSON Schema: %v", err)}
	}
	compiledSchema, err := compiler.Compile(schemaValidationURL)
	if err != nil {
		return nil, &inputError{fmt.Sprintf("invalid JSON Schema structure: %v", err)}
	}
	config.CompiledSchema = compiledSchema

	// Load prompt
	if prompt != "" && promptFile != "" {
		return nil, &cliError{"cannot specify both --prompt and --prompt-file"}
	}

	if prompt != "" {
		config.Prompt = strings.TrimSpace(prompt)
	} else if promptFile != "" {
		content, err := os.ReadFile(promptFile)
		if err != nil {
			return nil, &inputError{fmt.Sprintf("failed to read prompt file: %v", err)}
		}
		config.Prompt = strings.TrimSpace(string(content))
	} else {
		// Read from STDIN
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, &inputError{fmt.Sprintf("failed to read from STDIN: %v", err)}
		}
		config.Prompt = strings.TrimSpace(string(content))
	}

	if config.Prompt == "" {
		return nil, &inputError{"prompt cannot be empty"}
	}

	// Load project, location, model with environment fallback
	config.Project = getConfigValue(project, "GOOGLE_CLOUD_PROJECT", "CLOUDSDK_CORE_PROJECT")
	if config.Project == "" {
		return nil, &cliError{"--project is required (or set GOOGLE_CLOUD_PROJECT)"}
	}

	config.Location = getConfigValue(location, "GOOGLE_CLOUD_LOCATION", "GOOGLE_CLOUD_REGION", "CLOUDSDK_COMPUTE_REGION")
	if config.Location == "" {
		return nil, &cliError{"--location is required (or set GOOGLE_CLOUD_LOCATION)"}
	}

	config.Model = model
	if config.Model == "" {
		return nil, &cliError{"--model is required"}
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Configuration:\n")
		fmt.Fprintf(os.Stderr, "  Project: %s\n", config.Project)
		fmt.Fprintf(os.Stderr, "  Location: %s\n", config.Location)
		fmt.Fprintf(os.Stderr, "  Model: %s\n", config.Model)
		fmt.Fprintf(os.Stderr, "  System Instruction: %d chars\n", len(config.SystemInstruction))
		fmt.Fprintf(os.Stderr, "  Prompt: %d chars\n", len(config.Prompt))
		fmt.Fprintf(os.Stderr, "  Attachments: %d files\n", len(attachments))
	}

	return config, nil
}

func getConfigValue(flagValue string, envVars ...string) string {
	if flagValue != "" {
		return flagValue
	}
	for _, envVar := range envVars {
		if val := os.Getenv(envVar); val != "" {
			return val
		}
	}
	return ""
}

type attachmentPart struct {
	InlineData struct {
		MimeType string `json:"mimeType"`
		Data     string `json:"data"`
	} `json:"inlineData"`
}

func loadAttachments(config *Config) ([]interface{}, error) {
	var parts []interface{}
	var totalRawBytes int64
	var totalEncodedBytes int64

	for _, path := range attachments {
		// Determine MIME type from extension
		ext := strings.ToLower(filepath.Ext(path))
		var mimeType string
		var isImage bool
		switch ext {
		case ".png":
			mimeType = "image/png"
			isImage = true
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
			isImage = true
		case ".webp":
			mimeType = "image/webp"
			isImage = true
		case ".pdf":
			mimeType = "application/pdf"
			isImage = false
		default:
			return nil, &inputError{fmt.Sprintf("unsupported attachment type: %s (supported: .png, .jpg, .jpeg, .webp, .pdf)", ext)}
		}

		// Read and encode file
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, &inputError{fmt.Sprintf("failed to read attachment %s: %v", path, err)}
		}

		// Validate image file size (7 MB limit before base64 encoding)
		if isImage && len(content) > maxImageSizeBytes {
			sizeMB := float64(len(content)) / (1024 * 1024)
			return nil, &inputError{fmt.Sprintf("image file %s exceeds 7 MB limit: %.2f MB (Gemini API limits image files to 7 MB before base64 encoding)", path, sizeMB)}
		}

		encodedData := base64.StdEncoding.EncodeToString(content)
		totalRawBytes += int64(len(content))
		totalEncodedBytes += int64(len(encodedData))

		part := map[string]interface{}{
			"inlineData": map[string]interface{}{
				"mimeType": mimeType,
				"data":     encodedData,
			},
		}
		parts = append(parts, part)

		logVerbose(config, fmt.Sprintf("Loaded attachment: %s (%s, %d bytes raw, %d bytes encoded)", path, mimeType, len(content), len(encodedData)))
	}

	// Validate total attachment size doesn't approach the 20 MB request limit
	const maxAttachmentBytes = 20 * 1024 * 1024
	if totalEncodedBytes > maxAttachmentBytes {
		totalMB := float64(totalEncodedBytes) / (1024 * 1024)
		return nil, &inputError{fmt.Sprintf("total attachment size exceeds limit: %.2f MB encoded (limit 20 MB)", totalMB)}
	}

	if len(attachments) > 0 {
		logVerbose(config, fmt.Sprintf("Total attachments: %d files, %.2f MB raw, %.2f MB encoded",
			len(attachments),
			float64(totalRawBytes)/(1024*1024),
			float64(totalEncodedBytes)/(1024*1024)))
	}

	return parts, nil
}

func buildGeminiRequest(config *Config, attachmentParts []interface{}) ([]byte, error) {
	// Build parts array with prompt text and attachments
	contentParts := []interface{}{
		map[string]interface{}{
			"text": config.Prompt,
		},
	}
	contentParts = append(contentParts, attachmentParts...)

	request := map[string]interface{}{
		"systemInstruction": map[string]interface{}{
			"parts": []interface{}{
				map[string]interface{}{
					"text": config.SystemInstruction,
				},
			},
		},
		"contents": []interface{}{
			map[string]interface{}{
				"role":  "user",
				"parts": contentParts,
			},
		},
		"generationConfig": map[string]interface{}{
			"responseMimeType":   "application/json",
			"responseJsonSchema": config.Schema,
		},
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, &inputError{fmt.Sprintf("failed to marshal request: %v", err)}
	}

	return requestBytes, nil
}

func callGeminiAPI(config *Config, requestBody []byte) (string, error) {
	ctx := context.Background()

	// Get credentials and token
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return "", &apiError{fmt.Sprintf("failed to get credentials: %v", err)}
	}

	token, err := creds.TokenSource.Token()
	if err != nil {
		return "", &apiError{fmt.Sprintf("failed to get access token: %v", err)}
	}

	// Build URL
	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		config.Location, config.Project, config.Location, config.Model)

	logVerbose(config, fmt.Sprintf("Calling Gemini API: %s", url))

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBody))
	if err != nil {
		return "", &apiError{fmt.Sprintf("failed to create request: %v", err)}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", &apiError{fmt.Sprintf("failed to call API: %v", err)}
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", &apiError{fmt.Sprintf("failed to read response: %v", err)}
	}

	if resp.StatusCode != http.StatusOK {
		return "", &apiError{fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(respBody))}
	}

	// Parse response
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return "", &validationError{fmt.Sprintf("failed to parse response: %v", err)}
	}

	if len(geminiResp.Candidates) == 0 {
		return "", &validationError{"no candidates in response"}
	}

	candidate := geminiResp.Candidates[0]

	// Check finish reason
	if candidate.FinishReason != "STOP" {
		return "", &validationError{fmt.Sprintf("unexpected finish reason: %s", candidate.FinishReason)}
	}

	if len(candidate.Content.Parts) == 0 {
		return "", &validationError{"no content parts in response"}
	}

	jsonText := candidate.Content.Parts[0].Text
	if jsonText == "" {
		return "", &validationError{"empty response text"}
	}

	return jsonText, nil
}

// formatJSON formats a JSON object as minified or pretty-printed
func formatJSON(jsonObj interface{}, prettyPrint bool) (string, error) {
	var formattedBytes []byte
	var err error

	if prettyPrint {
		formattedBytes, err = json.MarshalIndent(jsonObj, "", "  ")
	} else {
		formattedBytes, err = json.Marshal(jsonObj)
	}

	if err != nil {
		return "", err
	}

	return string(formattedBytes), nil
}

// validateAndFormatJSON parses, validates, and formats JSON from LLM response
func validateAndFormatJSON(config *Config, rawResponse string) (string, error) {
	// Try to parse JSON
	var jsonObj interface{}
	if err := json.Unmarshal([]byte(rawResponse), &jsonObj); err != nil {
		// If parsing fails, return raw text with validation error
		return rawResponse, &validationError{fmt.Sprintf("response is not valid JSON: %v", err)}
	}

	// Validate the JSON against the pre-compiled schema
	if err := config.CompiledSchema.Validate(jsonObj); err != nil {
		// If validation fails, return formatted JSON with validation error
		formattedJSON, formatErr := formatJSON(jsonObj, config.PrettyPrint)
		if formatErr != nil {
			return rawResponse, &validationError{fmt.Sprintf("schema validation failed: %v (and formatting failed: %v)", err, formatErr)}
		}
		return formattedJSON, &validationError{fmt.Sprintf("schema validation failed: %v", err)}
	}

	// If validation succeeds, return formatted JSON with no error
	formattedJSON, err := formatJSON(jsonObj, config.PrettyPrint)
	if err != nil {
		return rawResponse, &validationError{fmt.Sprintf("formatting failed: %v", err)}
	}

	return formattedJSON, nil
}

func writeOutput(config *Config, jsonText string) error {
	if config.OutFile != "" {
		if err := os.WriteFile(config.OutFile, []byte(jsonText), 0644); err != nil {
			return &inputError{fmt.Sprintf("failed to write output file: %v", err)}
		}
	} else {
		fmt.Println(jsonText)
	}
	return nil
}

func logVerbose(config *Config, message string) {
	if config.Verbose {
		fmt.Fprintf(os.Stderr, "[verbose] %s\n", message)
	}
}

// Error types for different exit codes
type cliError struct {
	message string
}

func (e *cliError) Error() string {
	return e.message
}

type inputError struct {
	message string
}

func (e *inputError) Error() string {
	return e.message
}

type validationError struct {
	message string
}

func (e *validationError) Error() string {
	return e.message
}

type apiError struct {
	message string
}

func (e *apiError) Error() string {
	return e.message
}

func getExitCode(err error) int {
	switch err.(type) {
	case *cliError:
		return exitCLIUsageError
	case *inputError:
		return exitInputError
	case *validationError:
		return exitValidationError
	case *apiError:
		return exitAPIError
	default:
		return exitValidationError
	}
}
