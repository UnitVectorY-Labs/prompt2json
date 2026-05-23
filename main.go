package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"runtime"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/UnitVectorY-Labs/gcpvalidate/location"
	"github.com/UnitVectorY-Labs/gcpvalidate/project"
	"github.com/UnitVectorY-Labs/gcpvalidate/vertexai"
	jsp "github.com/UnitVectorY-Labs/jsonschemaprofiles"
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

// File size limits (Gemini-specific)
const (
	maxImageSizeBytes = 7 * 1024 * 1024  // 7 MB per image file (before base64 encoding)
	maxTotalSizeBytes = 20 * 1024 * 1024 // ~20 MB total request size limit
)

// Default OpenAI-compatible API URL
const defaultOpenAIURL = "https://api.openai.com/v1/chat/completions"

const (
	defaultRemoteTimeoutSeconds = 300
	autoTimeoutSeconds          = -1
)

// CLI flags
var (
	providerFlag          string
	systemInstruction     string
	systemInstructionFile string
	schema                string
	schemaFile            string
	prompt                string
	promptFile            string
	attachments           []string
	outFile               string
	projectFlag           string
	locationFlag          string
	modelFlag             string
	urlFlag               string
	apiKeyFlag            string
	strictSchemaFlag      bool
	schemaProfileFlag     string
	timeout               int
	verbose               bool
	prettyPrint           bool
	insecureFlag          bool
	showVersion           bool
	showHelp              bool
	showURL               bool
	showRequestBody       bool
)

func main() {
	// Set the build version from the build info if not set by the build system
	if Version == "dev" || Version == "" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
				Version = bi.Main.Version
			}
		}
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(getExitCode(err))
	}
}

func run() error {
	defineFlags()
	flag.Parse()

	if showVersion {
		fmt.Fprintf(os.Stderr, "%s\n", buildVersionString())
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

	// Load attachments (provider-aware)
	attachmentData, err := loadAttachments(config)
	if err != nil {
		return err
	}

	// Build API request (provider-specific)
	var requestBody []byte
	if config.Provider == "openai" {
		requestBody, err = buildOpenAIRequest(config, attachmentData)
	} else {
		requestBody, err = buildGeminiRequest(config, attachmentData)
	}
	if err != nil {
		return err
	}

	// Handle dry-run modes
	if showURL {
		url := buildAPIURL(config)
		if err := writeOutput(config, url); err != nil {
			return err
		}
		return nil
	}

	if showRequestBody {
		var formattedRequest string
		if prettyPrint {
			// Pretty-print the request body using json.Indent
			var prettyBuf bytes.Buffer
			if err := json.Indent(&prettyBuf, requestBody, "", "  "); err != nil {
				return &inputError{fmt.Sprintf("failed to format request body: %v", err)}
			}
			formattedRequest = prettyBuf.String()
		} else {
			formattedRequest = string(requestBody)
		}

		if err := writeOutput(config, formattedRequest); err != nil {
			return err
		}
		return nil
	}

	// Call API (provider-specific)
	var responseJSON string
	if config.Provider == "openai" {
		responseJSON, err = callOpenAIAPI(config, requestBody)
	} else {
		responseJSON, err = callGeminiAPI(config, requestBody)
	}
	if err != nil {
		return err
	}

	// Validate and format the JSON response
	formattedJSON, validationErr := validateAndFormatJSON(config, responseJSON)

	// If validation failed, don't write to STDOUT
	if validationErr != nil {
		return validationErr
	}

	if config.Verbose {
		if config.OutFile != "" {
			fmt.Fprintf(os.Stderr, "Output to: %s\n", config.OutFile)
		} else {
			fmt.Fprintf(os.Stderr, "Output to: stdout\n")
		}
	}

	// Write output only when validation succeeds
	if err := writeOutput(config, formattedJSON); err != nil {
		return err
	}

	return nil
}

func defineFlags() {
	flag.StringVar(&providerFlag, "provider", "", "API provider: gemini or openai (required)")
	flag.StringVar(&systemInstruction, "system-instruction", "", "System instruction (inline text)")
	flag.StringVar(&systemInstructionFile, "system-instruction-file", "", "System instruction from file")
	flag.StringVar(&schema, "schema", "", "JSON Schema (inline JSON)")
	flag.StringVar(&schemaFile, "schema-file", "", "JSON Schema from file")
	flag.StringVar(&prompt, "prompt", "", "Prompt text (inline)")
	flag.StringVar(&promptFile, "prompt-file", "", "Prompt from file")
	flag.Var((*stringArrayValue)(&attachments), "attach", "Attach file (repeatable)")
	flag.StringVar(&outFile, "out", "", "Output file path (default: STDOUT)")
	flag.StringVar(&projectFlag, "project", "", "GCP project ID (gemini only)")
	flag.StringVar(&locationFlag, "location", "", "GCP location/region (gemini only)")
	flag.StringVar(&modelFlag, "model", "", "Model identifier")
	flag.StringVar(&urlFlag, "url", "", "Override API URL (universal)")
	flag.StringVar(&apiKeyFlag, "api-key", "", "API key for bearer auth (universal)")
	flag.BoolVar(&strictSchemaFlag, "strict-schema", false, "Enable strict mode for JSON schema validation (openai only)")
	flag.StringVar(&schemaProfileFlag, "schema-profile", "", "Override schema profile for validation (e.g., OPENAI_202602, GEMINI_202602, MINIMAL_202602)")
	flag.IntVar(&timeout, "timeout", autoTimeoutSeconds, "HTTP request timeout in seconds (default: auto)")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging to STDERR")
	flag.BoolVar(&prettyPrint, "pretty-print", false, "Pretty-print JSON output")
	flag.BoolVar(&insecureFlag, "insecure", false, "Skip TLS certificate verification (use with caution)")
	flag.BoolVar(&showVersion, "version", false, "Show version")
	flag.BoolVar(&showHelp, "help", false, "Show help")
	flag.BoolVar(&showURL, "show-url", false, "Show the API URL that would be called (dry-run mode)")
	flag.BoolVar(&showRequestBody, "show-request-body", false, "Show the JSON request body that would be sent (dry-run mode)")
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
	fmt.Fprintf(os.Stderr, `prompt2json - Turn prompts into schema-validated JSON

Usage:
  prompt2json [OPTIONS]

Provider (required):
  --provider NAME            API provider: gemini or openai (required)

Required (all providers):
  --system-instruction TEXT | --system-instruction-file PATH
  --schema JSON             | --schema-file PATH
  --model NAME

Gemini-only (required unless --url is provided):
  --project ID              GCP project ID (env: GOOGLE_CLOUD_PROJECT)
  --location REGION         GCP location (env: GOOGLE_CLOUD_LOCATION)

OpenAI-only:
  --strict-schema            Enable strict mode for JSON schema validation

Schema profile:
  --schema-profile PROFILE   Override schema profile for validation
                             Default: OPENAI_202602 (openai), GEMINI_202602 (gemini)
                             Available: OPENAI_202602, GEMINI_202602, GEMINI_202503, MINIMAL_202602
                             See: https://jsonschemaprofiles.unitvectorylabs.com/

Universal overrides:
  --url URL                  Override provider's default API URL
  --api-key KEY              API key for bearer auth (env: OPENAI_API_KEY)

Input:
  --prompt TEXT              Prompt text (default: read from stdin)
  --prompt-file PATH         Read prompt from file (mutually exclusive with --prompt)
  --attach PATH              Attach file (repeatable): png, jpg/jpeg, webp, pdf

Output:
  --out PATH                 Write JSON to file (default: stdout)
  --pretty-print             Pretty-print JSON output (default: minified)

Dry-run (debug):
  --show-url                 Output the API URL without making the request
  --show-request-body        Output the JSON request body without making the request

Misc:
  --timeout SECONDS          HTTP request timeout in seconds
                             default: 300 for remote APIs, disabled for localhost
  --insecure                 Skip TLS certificate verification (like curl --insecure)
                             Use only for development servers with self-signed or
                             mismatched certificates. Not recommended for production.
  --verbose                  Log diagnostics to stderr
  --version                  Print version and exit
  --help                     Print help and exit

Environment (used if option not set):
  --project   GOOGLE_CLOUD_PROJECT, CLOUDSDK_CORE_PROJECT
  --location  GOOGLE_CLOUD_LOCATION, GOOGLE_CLOUD_REGION, CLOUDSDK_COMPUTE_REGION
  --api-key   OPENAI_API_KEY

Providers:
  gemini   Uses Vertex AI Gemini models. Default URL is constructed from
           project/location. Authentication uses ADC unless --api-key is set.
  openai   Uses OpenAI-compatible Chat Completions API. Default URL is
           https://api.openai.com/v1/chat/completions. Requires --api-key or
           OPENAI_API_KEY unless --url is provided (for local servers like Ollama).
           Compatible with OpenAI, Google Cloud's OpenAI endpoint, Ollama, and
           other compatible services.

Attachment support:
  gemini   Supports png, jpg, jpeg, webp, pdf (7 MB per image, 20 MB total)
  openai   Supports png, jpg, jpeg, webp, pdf as inline base64 content
           (some OpenAI-compatible endpoints may reject multimodal payloads)

Exit status: 0 success, 2 usage, 3 input, 4 validation/response, 5 API/auth

Example (gemini):
  echo "this is great" | prompt2json \
    --provider gemini \
    --system-instruction "Classify sentiment" \
    --schema '{"type":"object","properties":{"sentiment":{"type":"string"}},"required":["sentiment"],"additionalProperties":false}' \
    --project example-project \
    --location us-central1 \
    --model gemini-2.5-flash

Example (openai):
  echo "this is great" | prompt2json \
    --provider openai \
    --system-instruction "Classify sentiment" \
    --schema '{"type":"object","properties":{"sentiment":{"type":"string"}},"required":["sentiment"],"additionalProperties":false}' \
    --model gpt-4o \
    --api-key "$OPENAI_API_KEY"

Example (openai with Ollama):
  echo "this is great" | prompt2json \
    --provider openai \
    --url "http://localhost:11434/v1/chat/completions" \
    --system-instruction "Classify sentiment" \
    --schema '{"type":"object","properties":{"sentiment":{"type":"string"}},"required":["sentiment"],"additionalProperties":false}' \
    --model llama3
`)
}

var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+`)

func buildVersionString() string {
	version := Version
	if semverRe.MatchString(version) && !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return fmt.Sprintf("prompt2json version %s (%s, %s/%s)", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

type Config struct {
	Provider             string
	SystemInstruction    string
	SystemInstructionSrc string // Source: "flag" or file path
	Schema               map[string]any
	SchemaSrc            string // Source: "flag" or file path
	CompiledSchema       *jsonschema.Schema
	Prompt               string
	PromptSrc            string // Source: "stdin", "flag", or file path
	Project              string // Gemini only
	Location             string // Gemini only
	Model                string
	URL                  string // Override URL
	APIKey               string // Bearer token
	StrictSchema         bool   // OpenAI only: enable strict mode for JSON schema
	SchemaProfile        string // Override schema profile for validation
	Timeout              int
	OutFile              string
	Verbose              bool
	PrettyPrint          bool
	Insecure             bool
}

type Attachment struct {
	Path        string
	Filename    string
	MIMEType    string
	EncodedData string
	IsImage     bool
}

func loadConfiguration() (*Config, error) {
	config := &Config{
		Verbose:     verbose,
		OutFile:     outFile,
		PrettyPrint: prettyPrint,
	}

	// Validate and set provider (now required)
	provider := strings.ToLower(providerFlag)
	if provider == "" {
		return nil, &cliError{"--provider is required (gemini or openai)"}
	}
	if provider != "gemini" && provider != "openai" {
		return nil, &cliError{"--provider must be 'gemini' or 'openai'"}
	}
	config.Provider = provider

	// Load system instruction
	if systemInstruction != "" && systemInstructionFile != "" {
		return nil, &cliError{"cannot specify both --system-instruction and --system-instruction-file"}
	}
	if systemInstruction == "" && systemInstructionFile == "" {
		return nil, &cliError{"must specify either --system-instruction or --system-instruction-file"}
	}

	if systemInstruction != "" {
		config.SystemInstruction = strings.TrimSpace(systemInstruction)
		config.SystemInstructionSrc = "flag"
	} else {
		content, err := os.ReadFile(systemInstructionFile)
		if err != nil {
			return nil, &inputError{fmt.Sprintf("failed to read system instruction file: %v", err)}
		}
		config.SystemInstruction = strings.TrimSpace(string(content))
		config.SystemInstructionSrc = systemInstructionFile
	}

	if config.SystemInstruction == "" {
		return nil, &inputError{"system instruction cannot be empty"}
	}

	if verbose {
		if config.SystemInstructionSrc == "flag" {
			fmt.Fprintf(os.Stderr, "System instruction: %d bytes (from flag)\n", len(config.SystemInstruction))
		} else {
			fmt.Fprintf(os.Stderr, "System instruction: %d bytes (from %s)\n", len(config.SystemInstruction), config.SystemInstructionSrc)
		}
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
		config.SchemaSrc = "flag"
	} else {
		content, err := os.ReadFile(schemaFile)
		if err != nil {
			return nil, &inputError{fmt.Sprintf("failed to read schema file: %v", err)}
		}
		schemaBytes = content
		config.SchemaSrc = schemaFile
	}

	// Parse and validate schema
	if err := json.Unmarshal(schemaBytes, &config.Schema); err != nil {
		return nil, &inputError{fmt.Sprintf("invalid JSON in schema: %v", err)}
	}

	if verbose {
		if config.SchemaSrc == "flag" {
			fmt.Fprintf(os.Stderr, "Schema: %d bytes (from flag) - valid JSON\n", len(schemaBytes))
		} else {
			fmt.Fprintf(os.Stderr, "Schema: %d bytes (from %s) - valid JSON\n", len(schemaBytes), config.SchemaSrc)
		}
	}

	// Validate schema against provider-specific profile using jsonschemaprofiles
	profileID := resolveSchemaProfile(provider, schemaProfileFlag)
	config.SchemaProfile = string(profileID)

	report, err := jsp.ValidateSchema(profileID, schemaBytes, nil)
	if err != nil {
		return nil, &inputError{fmt.Sprintf("schema profile validation failed: %v", err)}
	}

	if !report.Valid {
		fmt.Fprintf(os.Stderr, "Schema profile validation (%s):\n%s\n", profileID, report.Text())
		return nil, &inputError{fmt.Sprintf("schema does not conform to %s profile; see https://jsonschemaprofiles.unitvectorylabs.com/ for details on schema limitations", profileID)}
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Schema profile validation (%s): PASSED\n", profileID)
	}

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

	if verbose {
		fmt.Fprintf(os.Stderr, "Schema validation: compiled successfully\n")
	}

	// Load prompt
	if prompt != "" && promptFile != "" {
		return nil, &cliError{"cannot specify both --prompt and --prompt-file"}
	}

	if prompt != "" {
		config.Prompt = strings.TrimSpace(prompt)
		config.PromptSrc = "flag"
	} else if promptFile != "" {
		content, err := os.ReadFile(promptFile)
		if err != nil {
			return nil, &inputError{fmt.Sprintf("failed to read prompt file: %v", err)}
		}
		config.Prompt = strings.TrimSpace(string(content))
		config.PromptSrc = promptFile
	} else {
		// Read from STDIN
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, &inputError{fmt.Sprintf("failed to read from STDIN: %v", err)}
		}
		config.Prompt = strings.TrimSpace(string(content))
		config.PromptSrc = "stdin"
	}

	if config.Prompt == "" {
		return nil, &inputError{"prompt cannot be empty"}
	}

	if verbose {
		switch config.PromptSrc {
		case "stdin":
			fmt.Fprintf(os.Stderr, "Prompt: %d bytes (from stdin)\n", len(config.Prompt))
		case "flag":
			fmt.Fprintf(os.Stderr, "Prompt: %d bytes (from flag)\n", len(config.Prompt))
		default:
			fmt.Fprintf(os.Stderr, "Prompt: %d bytes (from %s)\n", len(config.Prompt), config.PromptSrc)
		}
	}

	// Universal flags
	config.URL = urlFlag
	config.APIKey = getConfigValue(apiKeyFlag, "OPENAI_API_KEY")

	// Provider-specific configuration
	if config.Provider == "gemini" {
		// Gemini: require project/location unless --url is provided
		if config.URL == "" {
			config.Project = getConfigValue(projectFlag, "GOOGLE_CLOUD_PROJECT", "CLOUDSDK_CORE_PROJECT")
			if config.Project == "" {
				return nil, &cliError{"--project is required for gemini provider (or set GOOGLE_CLOUD_PROJECT, or use --url)"}
			}

			// Validate project ID
			if !project.IsValidProjectID(config.Project) {
				return nil, &inputError{fmt.Sprintf("invalid GCP project ID: %s", config.Project)}
			}

			config.Location = getConfigValue(locationFlag, "GOOGLE_CLOUD_LOCATION", "GOOGLE_CLOUD_REGION", "CLOUDSDK_COMPUTE_REGION")
			if config.Location == "" {
				return nil, &cliError{"--location is required for gemini provider (or set GOOGLE_CLOUD_LOCATION, or use --url)"}
			}

			// Validate region (allow "global" for Vertex AI models that are only available globally)
			if config.Location != "global" && !location.IsValidRegion(config.Location) {
				return nil, &inputError{fmt.Sprintf("invalid GCP region: %s", config.Location)}
			}
		} else {
			// When --url is provided, project/location are optional
			config.Project = projectFlag
			config.Location = locationFlag
		}

		// Reject --strict-schema for gemini provider
		if strictSchemaFlag {
			return nil, &cliError{"--strict-schema is only valid for openai provider"}
		}
	} else {
		// OpenAI provider: reject --project and --location with hard error
		if projectFlag != "" {
			return nil, &cliError{"--project is not valid for openai provider"}
		}
		if locationFlag != "" {
			return nil, &cliError{"--location is not valid for openai provider"}
		}

		// Set strict schema flag for OpenAI
		config.StrictSchema = strictSchemaFlag
	}

	// Model is required for all providers
	config.Model = getConfigValue(modelFlag)
	if config.Model == "" {
		return nil, &cliError{"--model is required"}
	}

	// Validate model name only for gemini provider
	if config.Provider == "gemini" && config.URL == "" {
		if !vertexai.IsValidVertexModelName(config.Model) {
			return nil, &inputError{fmt.Sprintf("invalid Vertex AI model name: %s", config.Model)}
		}
	}

	// Validate timeout
	if timeout < autoTimeoutSeconds {
		return nil, &cliError{"--timeout must be 0 or greater (-1 is reserved for automatic defaults)"}
	}
	config.Timeout = timeout
	config.Insecure = insecureFlag

	if verbose {
		if config.Provider == "gemini" {
			fmt.Fprintf(os.Stderr, "Provider: gemini\n")
			if config.URL != "" {
				fmt.Fprintf(os.Stderr, "API configuration: url=%s model=%s\n", config.URL, config.Model)
			} else {
				fmt.Fprintf(os.Stderr, "API configuration: project=%s location=%s model=%s\n", config.Project, config.Location, config.Model)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Provider: openai\n")
			if config.URL != "" {
				fmt.Fprintf(os.Stderr, "API configuration: url=%s model=%s\n", config.URL, config.Model)
			} else {
				fmt.Fprintf(os.Stderr, "API configuration: url=%s model=%s\n", defaultOpenAIURL, config.Model)
			}
		}
		if config.Insecure {
			fmt.Fprintf(os.Stderr, "TLS: certificate verification disabled (--insecure)\n")
		}
	}

	return config, nil
}

// resolveSchemaProfile determines which schema profile to use based on provider and optional override.
func resolveSchemaProfile(provider string, override string) jsp.ProfileID {
	if override != "" {
		return jsp.ProfileID(override)
	}
	if provider == "openai" {
		return jsp.OPENAI_202602
	}
	return jsp.GEMINI_202602
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

func loadAttachments(config *Config) ([]Attachment, error) {
	var parsedAttachments []Attachment
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

		// Validate image file size (7 MB limit before base64 encoding) - Gemini specific
		if config.Provider == "gemini" && isImage && len(content) > maxImageSizeBytes {
			sizeMB := float64(len(content)) / (1024 * 1024)
			return nil, &inputError{fmt.Sprintf("image file %s exceeds 7 MB limit: %.2f MB (Gemini API limits image files to 7 MB before base64 encoding)", path, sizeMB)}
		}

		encodedData := base64.StdEncoding.EncodeToString(content)
		totalEncodedBytes += int64(len(encodedData))

		parsedAttachments = append(parsedAttachments, Attachment{
			Path:        path,
			Filename:    filepath.Base(path),
			MIMEType:    mimeType,
			EncodedData: encodedData,
			IsImage:     isImage,
		})

		if config.Verbose {
			if isImage {
				sizeMB := float64(len(content)) / (1024 * 1024)
				fmt.Fprintf(os.Stderr, "Attachment: %s (%s, %.2f MB) - within size limits\n", path, mimeType, sizeMB)
			} else {
				fmt.Fprintf(os.Stderr, "Attachment: %s (%s, %d bytes)\n", path, mimeType, len(content))
			}
		}
	}

	// Validate total attachment size doesn't approach the 20 MB request limit (Gemini specific)
	if config.Provider == "gemini" && totalEncodedBytes > maxTotalSizeBytes {
		totalMB := float64(totalEncodedBytes) / (1024 * 1024)
		return nil, &inputError{fmt.Sprintf("total attachment size exceeds limit: %.2f MB encoded (Gemini limit is 20 MB)", totalMB)}
	}

	if len(attachments) > 0 && config.Verbose {
		totalMB := float64(totalEncodedBytes) / (1024 * 1024)
		if config.Provider == "gemini" {
			fmt.Fprintf(os.Stderr, "Total attachments: %d files, %.2f MB (encoded) - within Gemini limits\n", len(attachments), totalMB)
		} else {
			fmt.Fprintf(os.Stderr, "Total attachments: %d files, %.2f MB (encoded)\n", len(attachments), totalMB)
		}
	}

	return parsedAttachments, nil
}

func buildGeminiRequest(config *Config, attachmentData []Attachment) ([]byte, error) {
	// Build parts array with prompt text and attachments
	contentParts := []any{
		map[string]any{
			"text": config.Prompt,
		},
	}
	for _, attachment := range attachmentData {
		contentParts = append(contentParts, map[string]any{
			"inlineData": map[string]any{
				"mimeType": attachment.MIMEType,
				"data":     attachment.EncodedData,
			},
		})
	}

	request := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []any{
				map[string]any{
					"text": config.SystemInstruction,
				},
			},
		},
		"contents": []any{
			map[string]any{
				"role":  "user",
				"parts": contentParts,
			},
		},
		"generationConfig": map[string]any{
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

func buildGeminiURL(config *Config) string {
	// For global region, use aiplatform.googleapis.com (no region prefix)
	// For regional endpoints, use {region}-aiplatform.googleapis.com
	var url string
	if config.Location == "global" {
		url = fmt.Sprintf("https://aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
			config.Project, config.Location, config.Model)
	} else {
		url = fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
			config.Location, config.Project, config.Location, config.Model)
	}
	return url
}

// buildAPIURL returns the API URL for the configured provider
func buildAPIURL(config *Config) string {
	// If --url is provided, use it verbatim
	if config.URL != "" {
		return config.URL
	}

	// Provider-specific URL construction
	if config.Provider == "openai" {
		return defaultOpenAIURL
	}

	// Default: Gemini
	return buildGeminiURL(config)
}

func resolveHTTPTimeout(config *Config) time.Duration {
	if config.Timeout >= 0 {
		return time.Duration(config.Timeout) * time.Second
	}

	if isLoopbackURL(buildAPIURL(config)) {
		return 0
	}

	return defaultRemoteTimeoutSeconds * time.Second
}

func buildHTTPClient(config *Config) *http.Client {
	client := &http.Client{
		Timeout: resolveHTTPTimeout(config),
	}
	if config.Insecure {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
	}
	return client
}

func isLoopbackURL(rawURL string) bool {
	parsedURL, err := neturl.Parse(rawURL)
	if err != nil {
		return false
	}

	host := parsedURL.Hostname()
	if host == "" {
		return false
	}

	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func callGeminiAPI(config *Config, requestBody []byte) (string, error) {
	ctx := context.Background()

	// Build URL
	url := buildAPIURL(config)

	// Get authorization token
	var authToken string
	if config.APIKey != "" {
		// Use provided API key
		authToken = config.APIKey
	} else {
		// Use ADC for Gemini
		creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
		if err != nil {
			return "", &apiError{fmt.Sprintf("failed to get credentials: %v", err)}
		}

		token, err := creds.TokenSource.Token()
		if err != nil {
			return "", &apiError{fmt.Sprintf("failed to get access token: %v", err)}
		}
		authToken = token.AccessToken
	}

	if config.Verbose {
		fmt.Fprintf(os.Stderr, "Request: POST %s\n", url)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBody))
	if err != nil {
		return "", &apiError{fmt.Sprintf("failed to create request: %v", err)}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))

	// Send request
	client := buildHTTPClient(config)
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
			FinishReason  string `json:"finishReason"`
			FinishMessage string `json:"finishMessage"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
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
		// Include finishMessage in error for better diagnostics
		errorMsg := fmt.Sprintf("unexpected finish reason: %s", candidate.FinishReason)
		if candidate.FinishMessage != "" {
			errorMsg = fmt.Sprintf("%s (finishMessage: %s)", errorMsg, candidate.FinishMessage)
			// Log finishMessage to STDERR even when not in verbose mode
			fmt.Fprintf(os.Stderr, "Generation stopped: finishReason=%s, finishMessage=%s\n", candidate.FinishReason, candidate.FinishMessage)
		} else {
			fmt.Fprintf(os.Stderr, "Generation stopped: finishReason=%s\n", candidate.FinishReason)
		}
		return "", &validationError{errorMsg}
	}

	if len(candidate.Content.Parts) == 0 {
		return "", &validationError{"no content parts in response"}
	}

	// Concatenate all parts[].text in order
	var jsonTextBuilder strings.Builder
	for _, part := range candidate.Content.Parts {
		jsonTextBuilder.WriteString(part.Text)
	}
	jsonText := jsonTextBuilder.String()

	if jsonText == "" {
		return "", &validationError{"empty response text"}
	}

	// Log token usage if verbose
	if config.Verbose {
		fmt.Fprintf(os.Stderr, "API response: finish_reason=%s\n", candidate.FinishReason)
		if geminiResp.UsageMetadata.TotalTokenCount > 0 {
			fmt.Fprintf(os.Stderr, "Token usage:\n")
			fmt.Fprintf(os.Stderr, "  promptTokenCount:     %d\n", geminiResp.UsageMetadata.PromptTokenCount)
			fmt.Fprintf(os.Stderr, "  candidatesTokenCount: %d\n", geminiResp.UsageMetadata.CandidatesTokenCount)
			fmt.Fprintf(os.Stderr, "  totalTokenCount:      %d\n", geminiResp.UsageMetadata.TotalTokenCount)
		}
	}

	return jsonText, nil
}

// buildOpenAIRequest creates a Chat Completions API request body with structured outputs
func buildOpenAIRequest(config *Config, attachmentData []Attachment) ([]byte, error) {
	// Build the request using OpenAI Chat Completions format with structured outputs
	// The response_format with json_schema enforces structured output
	jsonSchemaConfig := map[string]any{
		"name":   "response",
		"schema": config.Schema,
	}

	// Add strict mode only if enabled
	if config.StrictSchema {
		jsonSchemaConfig["strict"] = true
	}

	userContent := any(config.Prompt)
	if len(attachmentData) > 0 {
		// Chat Completions multimodal content blocks for inline files:
		// - Images use `type: image_url` with a base64 data URL.
		// - PDFs use `type: file` with `file_data` and `filename`.
		contentParts := []any{
			map[string]any{
				"type": "text",
				"text": config.Prompt,
			},
		}

		for _, attachment := range attachmentData {
			if attachment.IsImage {
				contentParts = append(contentParts, map[string]any{
					"type": "image_url",
					"image_url": map[string]any{
						"url": fmt.Sprintf("data:%s;base64,%s", attachment.MIMEType, attachment.EncodedData),
					},
				})
			} else {
				contentParts = append(contentParts, map[string]any{
					"type": "file",
					"file": map[string]any{
						"filename":  attachment.Filename,
						"file_data": fmt.Sprintf("data:%s;base64,%s", attachment.MIMEType, attachment.EncodedData),
					},
				})
			}
		}

		userContent = contentParts
	}

	request := map[string]any{
		"model": config.Model,
		"messages": []map[string]any{
			{
				"role":    "system",
				"content": config.SystemInstruction,
			},
			{
				"role":    "user",
				"content": userContent,
			},
		},
		"response_format": map[string]any{
			"type":        "json_schema",
			"json_schema": jsonSchemaConfig,
		},
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, &inputError{fmt.Sprintf("failed to marshal request: %v", err)}
	}

	return requestBytes, nil
}

// callOpenAIAPI calls an OpenAI-compatible Chat Completions API
func callOpenAIAPI(config *Config, requestBody []byte) (string, error) {
	ctx := context.Background()

	// Build URL
	url := buildAPIURL(config)

	// API key is optional when --url is provided (for local servers like Ollama)
	// but required when using the default OpenAI URL
	if config.APIKey == "" && config.URL == "" {
		return "", &apiError{"--api-key is required for openai provider (or set OPENAI_API_KEY, or use --url for local servers)"}
	}

	if config.Verbose {
		fmt.Fprintf(os.Stderr, "Request: POST %s\n", url)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBody))
	if err != nil {
		return "", &apiError{fmt.Sprintf("failed to create request: %v", err)}
	}

	req.Header.Set("Content-Type", "application/json")
	// Only set Authorization header if API key is provided
	if config.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", config.APIKey))
	}

	// Send request
	client := buildHTTPClient(config)
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

	// Parse response (OpenAI Chat Completions format)
	var openaiResp struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return "", &validationError{fmt.Sprintf("failed to parse response: %v", err)}
	}

	if len(openaiResp.Choices) == 0 {
		return "", &validationError{"no choices in response"}
	}

	choice := openaiResp.Choices[0]

	// Check finish reason
	if choice.FinishReason != "stop" {
		errorMsg := fmt.Sprintf("unexpected finish reason: %s", choice.FinishReason)
		fmt.Fprintf(os.Stderr, "Generation stopped: finish_reason=%s\n", choice.FinishReason)
		return "", &validationError{errorMsg}
	}

	jsonText := choice.Message.Content

	if jsonText == "" {
		return "", &validationError{"empty response content"}
	}

	// Log token usage if verbose
	if config.Verbose {
		fmt.Fprintf(os.Stderr, "API response: finish_reason=%s\n", choice.FinishReason)
		if openaiResp.Usage.TotalTokens > 0 {
			fmt.Fprintf(os.Stderr, "Token usage:\n")
			fmt.Fprintf(os.Stderr, "  prompt_tokens:     %d\n", openaiResp.Usage.PromptTokens)
			fmt.Fprintf(os.Stderr, "  completion_tokens: %d\n", openaiResp.Usage.CompletionTokens)
			fmt.Fprintf(os.Stderr, "  total_tokens:      %d\n", openaiResp.Usage.TotalTokens)
		}
	}

	return jsonText, nil
}
func formatJSON(jsonObj any, prettyPrint bool) (string, error) {
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
	var jsonObj any
	if err := json.Unmarshal([]byte(rawResponse), &jsonObj); err != nil {
		// If parsing fails, return raw text with validation error
		if config.Verbose {
			fmt.Fprintf(os.Stderr, "Validation: response is not valid JSON - FAILED\n")
		}
		return rawResponse, &validationError{fmt.Sprintf("response is not valid JSON: %v", err)}
	}

	if config.Verbose {
		fmt.Fprintf(os.Stderr, "Validation: response is valid JSON - PASSED\n")
	}

	// Defensive check for nil compiled schema (should not happen in normal flow)
	if config.CompiledSchema == nil {
		return rawResponse, &validationError{"schema not compiled"}
	}

	// Validate the JSON against the pre-compiled schema
	if err := config.CompiledSchema.Validate(jsonObj); err != nil {
		// If validation fails, return formatted JSON with validation error
		if config.Verbose {
			fmt.Fprintf(os.Stderr, "Validation: schema validation - FAILED\n")
		}
		formattedJSON, formatErr := formatJSON(jsonObj, config.PrettyPrint)
		if formatErr != nil {
			return rawResponse, &validationError{fmt.Sprintf("schema validation failed: %v (and formatting failed: %v)", err, formatErr)}
		}
		return formattedJSON, &validationError{fmt.Sprintf("schema validation failed: %v", err)}
	}

	if config.Verbose {
		fmt.Fprintf(os.Stderr, "Validation: schema validation - PASSED\n")
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
