package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	jsp "github.com/UnitVectorY-Labs/jsonschemaprofiles"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

func TestBuildOpenAIRequestTextOnlyUserContentString(t *testing.T) {
	config := &Config{
		Model:             "gpt-4o",
		SystemInstruction: "Classify sentiment",
		Prompt:            "this is great",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"sentiment": map[string]any{"type": "string"},
			},
			"required": []any{"sentiment"},
		},
	}

	requestBytes, err := buildOpenAIRequest(config, nil)
	if err != nil {
		t.Fatalf("buildOpenAIRequest returned error: %v", err)
	}

	var request map[string]any
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	messages, ok := request["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %#v", request["messages"])
	}

	userMessage, ok := messages[1].(map[string]any)
	if !ok {
		t.Fatalf("expected user message object, got %#v", messages[1])
	}

	content, ok := userMessage["content"].(string)
	if !ok {
		t.Fatalf("expected user content string, got %#v", userMessage["content"])
	}

	if content != "this is great" {
		t.Fatalf("unexpected user content: %q", content)
	}
}

func TestBuildOpenAIRequestWithInlineImageAndPDF(t *testing.T) {
	config := &Config{
		Model:             "gpt-4o",
		SystemInstruction: "Extract information",
		Prompt:            "Process attachments",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []any{"name"},
		},
	}

	attachmentData := []Attachment{
		{
			Filename:    "picture.png",
			MIMEType:    "image/png",
			EncodedData: "aW1hZ2UtYnl0ZXM=",
			IsImage:     true,
		},
		{
			Filename:    "resume.pdf",
			MIMEType:    "application/pdf",
			EncodedData: "cGRmLWJ5dGVz",
			IsImage:     false,
		},
	}

	requestBytes, err := buildOpenAIRequest(config, attachmentData)
	if err != nil {
		t.Fatalf("buildOpenAIRequest returned error: %v", err)
	}

	var request map[string]any
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	messages, ok := request["messages"].([]any)
	if !ok || len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %#v", request["messages"])
	}

	userMessage, ok := messages[1].(map[string]any)
	if !ok {
		t.Fatalf("expected user message object, got %#v", messages[1])
	}

	contentParts, ok := userMessage["content"].([]any)
	if !ok {
		t.Fatalf("expected multimodal content array, got %#v", userMessage["content"])
	}

	if len(contentParts) != 3 {
		t.Fatalf("expected 3 content parts (text, image, file), got %d", len(contentParts))
	}

	textPart := contentParts[0].(map[string]any)
	if textPart["type"] != "text" || textPart["text"] != "Process attachments" {
		t.Fatalf("unexpected text part: %#v", textPart)
	}

	imagePart := contentParts[1].(map[string]any)
	if imagePart["type"] != "image_url" {
		t.Fatalf("unexpected image part type: %#v", imagePart)
	}
	imageURL := imagePart["image_url"].(map[string]any)["url"]
	if imageURL != "data:image/png;base64,aW1hZ2UtYnl0ZXM=" {
		t.Fatalf("unexpected image url: %#v", imageURL)
	}

	filePart := contentParts[2].(map[string]any)
	if filePart["type"] != "file" {
		t.Fatalf("unexpected file part type: %#v", filePart)
	}
	fileContent := filePart["file"].(map[string]any)
	if fileContent["filename"] != "resume.pdf" {
		t.Fatalf("unexpected filename: %#v", fileContent["filename"])
	}
	if fileContent["file_data"] != "data:application/pdf;base64,cGRmLWJ5dGVz" {
		t.Fatalf("unexpected file_data: %#v", fileContent["file_data"])
	}
}

func TestBuildOpenAIRequestStrictSchemaToggle(t *testing.T) {
	baseConfig := &Config{
		Model:             "gpt-4o",
		SystemInstruction: "Extract information",
		Prompt:            "Process this",
		Schema: map[string]any{
			"type": "object",
		},
	}

	t.Run("strict enabled", func(t *testing.T) {
		config := *baseConfig
		config.StrictSchema = true

		requestBytes, err := buildOpenAIRequest(&config, nil)
		if err != nil {
			t.Fatalf("buildOpenAIRequest returned error: %v", err)
		}

		var request map[string]any
		if err := json.Unmarshal(requestBytes, &request); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}

		responseFormat := request["response_format"].(map[string]any)
		jsonSchemaConfig := responseFormat["json_schema"].(map[string]any)

		if jsonSchemaConfig["strict"] != true {
			t.Fatalf("expected strict=true, got %#v", jsonSchemaConfig["strict"])
		}
	})

	t.Run("strict disabled", func(t *testing.T) {
		config := *baseConfig
		config.StrictSchema = false

		requestBytes, err := buildOpenAIRequest(&config, nil)
		if err != nil {
			t.Fatalf("buildOpenAIRequest returned error: %v", err)
		}

		var request map[string]any
		if err := json.Unmarshal(requestBytes, &request); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}

		responseFormat := request["response_format"].(map[string]any)
		jsonSchemaConfig := responseFormat["json_schema"].(map[string]any)

		if _, ok := jsonSchemaConfig["strict"]; ok {
			t.Fatalf("expected strict to be omitted when disabled, got %#v", jsonSchemaConfig["strict"])
		}
	})
}

func TestBuildGeminiRequestIncludesPromptAndAttachment(t *testing.T) {
	config := &Config{
		SystemInstruction: "Extract fields",
		Prompt:            "Parse this attachment",
		Schema: map[string]any{
			"type": "object",
		},
	}
	attachmentData := []Attachment{
		{
			MIMEType:    "application/pdf",
			EncodedData: "cGRmLWJ5dGVz",
		},
	}

	requestBytes, err := buildGeminiRequest(config, attachmentData)
	if err != nil {
		t.Fatalf("buildGeminiRequest returned error: %v", err)
	}

	var request map[string]any
	if err := json.Unmarshal(requestBytes, &request); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	contents := request["contents"].([]any)
	parts := contents[0].(map[string]any)["parts"].([]any)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts (prompt + attachment), got %d", len(parts))
	}

	textPart := parts[0].(map[string]any)
	if textPart["text"] != "Parse this attachment" {
		t.Fatalf("unexpected text part: %#v", textPart)
	}

	inline := parts[1].(map[string]any)["inlineData"].(map[string]any)
	if inline["mimeType"] != "application/pdf" {
		t.Fatalf("unexpected mimeType: %#v", inline["mimeType"])
	}
	if inline["data"] != "cGRmLWJ5dGVz" {
		t.Fatalf("unexpected attachment data: %#v", inline["data"])
	}
}

func TestBuildAPIURL(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{
			name: "custom URL override",
			config: Config{
				Provider: "openai",
				URL:      "http://localhost:11434/v1/chat/completions",
			},
			want: "http://localhost:11434/v1/chat/completions",
		},
		{
			name: "openai default URL",
			config: Config{
				Provider: "openai",
			},
			want: defaultOpenAIURL,
		},
		{
			name: "gemini regional URL",
			config: Config{
				Provider: "gemini",
				Project:  "example-project",
				Location: "us-central1",
				Model:    "gemini-2.5-flash",
			},
			want: "https://us-central1-aiplatform.googleapis.com/v1/projects/example-project/locations/us-central1/publishers/google/models/gemini-2.5-flash:generateContent",
		},
		{
			name: "gemini global URL",
			config: Config{
				Provider: "gemini",
				Project:  "example-project",
				Location: "global",
				Model:    "gemini-2.5-flash",
			},
			want: "https://aiplatform.googleapis.com/v1/projects/example-project/locations/global/publishers/google/models/gemini-2.5-flash:generateContent",
		},
	}

	for _, tc := range tests {
		got := buildAPIURL(&tc.config)
		if got != tc.want {
			t.Fatalf("%s: expected %q, got %q", tc.name, tc.want, got)
		}
	}
}

func TestResolveHTTPTimeout(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string
	}{
		{
			name: "explicit timeout is respected",
			config: Config{
				Provider: "openai",
				Timeout:  15,
			},
			want: "15s",
		},
		{
			name: "remote openai uses default timeout",
			config: Config{
				Provider: "openai",
				Timeout:  autoTimeoutSeconds,
			},
			want: "5m0s",
		},
		{
			name: "localhost openai disables timeout",
			config: Config{
				Provider: "openai",
				URL:      "http://localhost:11434/v1/chat/completions",
				Timeout:  autoTimeoutSeconds,
			},
			want: "0s",
		},
		{
			name: "loopback ip disables timeout",
			config: Config{
				Provider: "openai",
				URL:      "http://127.0.0.1:11434/v1/chat/completions",
				Timeout:  autoTimeoutSeconds,
			},
			want: "0s",
		},
	}

	for _, tc := range tests {
		got := resolveHTTPTimeout(&tc.config).String()
		if got != tc.want {
			t.Fatalf("%s: expected %q, got %q", tc.name, tc.want, got)
		}
	}
}

func TestGetConfigValuePrecedence(t *testing.T) {
	t.Setenv("PROMPT2JSON_ENV_ONE", "from-env-one")
	t.Setenv("PROMPT2JSON_ENV_TWO", "from-env-two")

	if got := getConfigValue("from-flag", "PROMPT2JSON_ENV_ONE", "PROMPT2JSON_ENV_TWO"); got != "from-flag" {
		t.Fatalf("expected flag value to win, got %q", got)
	}

	if got := getConfigValue("", "PROMPT2JSON_ENV_ONE", "PROMPT2JSON_ENV_TWO"); got != "from-env-one" {
		t.Fatalf("expected first non-empty env value, got %q", got)
	}

	t.Setenv("PROMPT2JSON_ENV_ONE", "")
	if got := getConfigValue("", "PROMPT2JSON_ENV_ONE", "PROMPT2JSON_ENV_TWO"); got != "from-env-two" {
		t.Fatalf("expected fallback env value, got %q", got)
	}
}

func TestValidateAndFormatJSONSuccess(t *testing.T) {
	schema := mustCompileSchema(t, `{
		"type":"object",
		"required":["name"],
		"properties":{"name":{"type":"string"}}
	}`)

	config := &Config{
		CompiledSchema: schema,
		PrettyPrint:    false,
	}
	got, err := validateAndFormatJSON(config, `{"name":"jared"}`)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got != `{"name":"jared"}` {
		t.Fatalf("unexpected formatted JSON: %q", got)
	}
}

func TestValidateAndFormatJSONSchemaFailure(t *testing.T) {
	schema := mustCompileSchema(t, `{
		"type":"object",
		"required":["age"],
		"properties":{"age":{"type":"integer"}}
	}`)

	config := &Config{
		CompiledSchema: schema,
		PrettyPrint:    false,
	}
	got, err := validateAndFormatJSON(config, `{"age":"not-an-int"}`)
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "schema validation failed") {
		t.Fatalf("expected schema validation error, got %v", err)
	}
	if got != `{"age":"not-an-int"}` {
		t.Fatalf("expected formatted JSON on schema failure, got %q", got)
	}
}

func TestValidateAndFormatJSONInvalidJSON(t *testing.T) {
	config := &Config{}
	raw := `{"name":`
	got, err := validateAndFormatJSON(config, raw)
	if err == nil {
		t.Fatalf("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "response is not valid JSON") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}
	if got != raw {
		t.Fatalf("expected original raw response, got %q", got)
	}
}

func TestValidateAndFormatJSONRequiresCompiledSchema(t *testing.T) {
	config := &Config{}
	got, err := validateAndFormatJSON(config, `{"ok":true}`)
	if err == nil {
		t.Fatalf("expected schema-not-compiled error, got nil")
	}
	if err.Error() != "schema not compiled" {
		t.Fatalf("expected schema not compiled error, got %v", err)
	}
	if got != `{"ok":true}` {
		t.Fatalf("expected raw response when schema missing, got %q", got)
	}
}

func TestLoadAttachmentsParsesAndEncodesFiles(t *testing.T) {
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "picture.PNG")
	pdfPath := filepath.Join(tmpDir, "resume.pdf")

	imageContent := []byte("image-bytes")
	pdfContent := []byte("pdf-bytes")
	if err := os.WriteFile(imagePath, imageContent, 0600); err != nil {
		t.Fatalf("failed to write image test file: %v", err)
	}
	if err := os.WriteFile(pdfPath, pdfContent, 0600); err != nil {
		t.Fatalf("failed to write pdf test file: %v", err)
	}

	restoreAttachments := setTestAttachments([]string{imagePath, pdfPath})
	defer restoreAttachments()

	got, err := loadAttachments(&Config{Provider: "openai"})
	if err != nil {
		t.Fatalf("loadAttachments returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(got))
	}

	if got[0].MIMEType != "image/png" || !got[0].IsImage {
		t.Fatalf("unexpected first attachment metadata: %#v", got[0])
	}
	if got[0].Filename != "picture.PNG" {
		t.Fatalf("unexpected first attachment filename: %q", got[0].Filename)
	}
	if got[0].EncodedData != base64.StdEncoding.EncodeToString(imageContent) {
		t.Fatalf("unexpected first attachment encoded data: %q", got[0].EncodedData)
	}

	if got[1].MIMEType != "application/pdf" || got[1].IsImage {
		t.Fatalf("unexpected second attachment metadata: %#v", got[1])
	}
	if got[1].Filename != "resume.pdf" {
		t.Fatalf("unexpected second attachment filename: %q", got[1].Filename)
	}
	if got[1].EncodedData != base64.StdEncoding.EncodeToString(pdfContent) {
		t.Fatalf("unexpected second attachment encoded data: %q", got[1].EncodedData)
	}
}

func TestLoadAttachmentsRejectsUnsupportedType(t *testing.T) {
	tmpDir := t.TempDir()
	txtPath := filepath.Join(tmpDir, "notes.txt")
	if err := os.WriteFile(txtPath, []byte("not supported"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	restoreAttachments := setTestAttachments([]string{txtPath})
	defer restoreAttachments()

	_, err := loadAttachments(&Config{Provider: "openai"})
	if err == nil {
		t.Fatalf("expected unsupported type error, got nil")
	}

	var inErr *inputError
	if !errors.As(err, &inErr) {
		t.Fatalf("expected inputError, got %T (%v)", err, err)
	}
	if !strings.Contains(err.Error(), "unsupported attachment type") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLoadAttachmentsGeminiImageSizeLimit(t *testing.T) {
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "too-large.png")
	tooLargeImage := bytes.Repeat([]byte{0x01}, maxImageSizeBytes+1)
	if err := os.WriteFile(imagePath, tooLargeImage, 0600); err != nil {
		t.Fatalf("failed to write large image test file: %v", err)
	}

	restoreAttachments := setTestAttachments([]string{imagePath})
	defer restoreAttachments()

	_, err := loadAttachments(&Config{Provider: "gemini"})
	if err == nil {
		t.Fatalf("expected gemini image size limit error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds 7 MB limit") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestGetExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "cli", err: &cliError{"x"}, want: exitCLIUsageError},
		{name: "input", err: &inputError{"x"}, want: exitInputError},
		{name: "validation", err: &validationError{"x"}, want: exitValidationError},
		{name: "api", err: &apiError{"x"}, want: exitAPIError},
		{name: "fallback", err: errors.New("x"), want: exitValidationError},
	}

	for _, tc := range tests {
		if got := getExitCode(tc.err); got != tc.want {
			t.Fatalf("%s: expected exit code %d, got %d", tc.name, tc.want, got)
		}
	}
}

func setTestAttachments(paths []string) func() {
	original := attachments
	attachments = paths
	return func() {
		attachments = original
	}
}

func mustCompileSchema(t *testing.T, schemaJSON string) *jsonschema.Schema {
	t.Helper()

	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020
	if err := compiler.AddResource(schemaValidationURL, strings.NewReader(schemaJSON)); err != nil {
		t.Fatalf("failed to add schema resource: %v", err)
	}

	compiled, err := compiler.Compile(schemaValidationURL)
	if err != nil {
		t.Fatalf("failed to compile schema: %v", err)
	}
	return compiled
}

func TestResolveSchemaProfile(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		override string
		want     jsp.ProfileID
	}{
		{
			name:     "openai default",
			provider: "openai",
			override: "",
			want:     jsp.OPENAI_202602,
		},
		{
			name:     "gemini default",
			provider: "gemini",
			override: "",
			want:     jsp.GEMINI_202602,
		},
		{
			name:     "override with MINIMAL_202602",
			provider: "openai",
			override: "MINIMAL_202602",
			want:     jsp.MINIMAL_202602,
		},
		{
			name:     "override with GEMINI_202602 on openai provider",
			provider: "openai",
			override: "GEMINI_202602",
			want:     jsp.GEMINI_202602,
		},
		{
			name:     "override with OPENAI_202602 on gemini provider",
			provider: "gemini",
			override: "OPENAI_202602",
			want:     jsp.OPENAI_202602,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveSchemaProfile(tc.provider, tc.override)
			if got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}

func TestSchemaProfileValidationValidSchema(t *testing.T) {
	schemaBytes := []byte(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"],"additionalProperties":false}`)

	report, err := jsp.ValidateSchema(jsp.OPENAI_202602, schemaBytes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.Valid {
		t.Fatalf("expected schema to be valid, got invalid: %s", report.Text())
	}
}

func TestSchemaProfileValidationInvalidSchema(t *testing.T) {
	// Schema with patternProperties, which is not allowed in OpenAI profile
	schemaBytes := []byte(`{"type":"object","patternProperties":{"^S_":{"type":"string"}}}`)

	report, err := jsp.ValidateSchema(jsp.OPENAI_202602, schemaBytes, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Valid {
		t.Fatalf("expected schema to be invalid for OpenAI profile")
	}
}
