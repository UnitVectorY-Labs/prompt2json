[![GitHub release](https://img.shields.io/github/release/UnitVectorY-Labs/prompt2json.svg)](https://github.com/UnitVectorY-Labs/prompt2json/releases/latest) [![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://opensource.org/licenses/MIT) [![Active](https://img.shields.io/badge/Status-Active-green)](https://guide.unitvectorylabs.com/bestpractices/status/#active)
 [![Go Report Card](https://goreportcard.com/badge/github.com/UnitVectorY-Labs/prompt2json)](https://goreportcard.com/report/github.com/UnitVectorY-Labs/prompt2json)

# prompt2json

Unix-style CLI that sends a system instruction, required JSON Schema, and text inputs to LLM APIs and returns schema-validated JSON for easy batch processing. Supports Vertex AI (Gemini) and OpenAI-compatible endpoints.

## Overview

`prompt2json` is designed for composable command line workflows:

- Turn free form prompts into machine reliable JSON for automation and batch workflows
- Enforce output shape using JSON Schema rather than post processing heuristics
- Make LLMs usable in shell pipelines, scripts, and data processing jobs
- Enable repeatable, inspectable prompt experiments from the command line
- Treat LLM calls as deterministic interfaces, not interactive sessions

## Providers

| Provider | Description | Default URL |
|----------|-------------|-------------|
| `gemini` (default) | Vertex AI Gemini models | Constructed from `--project` and `--location` |
| `openapi` | OpenAI-compatible Chat Completions API | `https://api.openai.com/v1/chat/completions` |

The `openapi` provider works with OpenAI, Google Cloud's OpenAI-compatible endpoint, Ollama, and other compatible services.

## Installation

```bash
go install github.com/UnitVectorY-Labs/prompt2json@latest
```

Build from source:

```bash
git clone https://github.com/UnitVectorY-Labs/prompt2json.git
cd prompt2json
go build -o prompt2json
```

## Examples

### Gemini Provider (default)

```bash
export GOOGLE_CLOUD_PROJECT=example-project
echo "this is great" | prompt2json \
    --system-instruction "Classify sentiment" \
    --schema '{"type":"object","properties":{"sentiment":{"type":"string","enum":["POSITIVE","NEGATIVE","NEUTRAL"]},"confidence":{"type":"integer","minimum":0,"maximum":100}},"required":["sentiment","confidence"]}' \
    --location us-central1 \
    --model gemini-2.5-flash
```

### OpenAPI Provider

```bash
echo "this is great" | prompt2json \
    --provider openapi \
    --system-instruction "Classify sentiment" \
    --schema '{"type":"object","properties":{"sentiment":{"type":"string","enum":["POSITIVE","NEGATIVE","NEUTRAL"]},"confidence":{"type":"integer","minimum":0,"maximum":100}},"required":["sentiment","confidence"]}' \
    --model gpt-4o \
    --api-key "$OPENAI_API_KEY"
```

The output will be minified JSON matching the specified schema:

```json
{"sentiment":"POSITIVE","confidence":95}
```

## Usage

```
prompt2json [OPTIONS]
```

### Authentication

**Gemini provider:** Uses Google Application Default Credentials by default. Authenticate locally with:

```bash
gcloud auth application-default login
```

Or via service account:

```bash
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/key.json
```

**OpenAPI provider:** Requires an API key via `--api-key` flag or `OPENAI_API_KEY` environment variable.

For complete usage documentation including all options, environment variables, and command line conventions, see the [Usage documentation](https://unitvectory-labs.github.io/prompt2json/usage).

## Attachment Support

| Provider | Attachments |
|----------|-------------|
| `gemini` | Supports png, jpg, jpeg, webp, pdf (7 MB per image, 20 MB total) |
| `openapi` | Text prompts only; attachments are not supported |

## Limitations

- Gemini: Image attachments are limited to 7 MB each before base64 encoding
- Gemini: Total request size is limited to roughly 20 MB
- OpenAPI: File attachments are not supported (text prompts only)
- Limitations of the underlying LLM models apply
