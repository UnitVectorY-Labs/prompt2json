[![GitHub release](https://img.shields.io/github/release/UnitVectorY-Labs/prompt2json.svg)](https://github.com/UnitVectorY-Labs/prompt2json/releases/latest) [![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://opensource.org/licenses/MIT) [![Active](https://img.shields.io/badge/Status-Active-green)](https://guide.unitvectorylabs.com/bestpractices/status/#active)
 [![Go Report Card](https://goreportcard.com/badge/github.com/UnitVectorY-Labs/prompt2json)](https://goreportcard.com/report/github.com/UnitVectorY-Labs/prompt2json)

# prompt2json

Unix-style CLI that sends a system instruction, required JSON Schema, and text or inline file inputs to LLM APIs and returns schema-validated JSON for easy batch processing. Supports Vertex AI (Gemini) and OpenAI-compatible Chat Completions endpoints.

## Overview

`prompt2json` is designed for composable command line workflows:

- Turn free form prompts into machine reliable JSON for automation in bash workflows
- Enforce output shape using JSON Schema rather than post processing heuristics
- Make LLMs usable in shell pipelines, scripts, and data processing jobs
- Enable repeatable, inspectable prompt experiments from the command line
- Treat LLM calls as deterministic interfaces, not interactive sessions

![prompt2json diagram](docs/diagram.svg)

## Providers

The `--provider` flag is required and determines which API format to use:

| Provider | Description | Default URL |
|----------|-------------|-------------|
| `gemini` | Vertex AI Gemini models | Constructed from `--project` and `--location` |
| `openai` | OpenAI-compatible Chat Completions API | `https://api.openai.com/v1/chat/completions` |

The `openai` provider works with OpenAI, Google Cloud's OpenAI-compatible endpoint, Ollama, and other compatible services.

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

### Gemini Provider

```bash
prompt2json \
    --provider gemini \
    --prompt "this is great" \
    --system-instruction "Classify sentiment" \
    --schema '{"type":"object","properties":{"sentiment":{"type":"string","enum":["POSITIVE","NEGATIVE","NEUTRAL"]},"confidence":{"type":"integer","minimum":0,"maximum":100}},"required":["sentiment","confidence"]}' \
    --location us-central1 \
    --project example-project \
    --model gemini-2.5-flash
```

### OpenAI Provider

```bash
prompt2json \
    --provider openai \
    --prompt "this is great" \
    --system-instruction "Classify sentiment" \
    --schema '{"type":"object","properties":{"sentiment":{"type":"string","enum":["POSITIVE","NEGATIVE","NEUTRAL"]},"confidence":{"type":"integer","minimum":0,"maximum":100}},"required":["sentiment","confidence"]}' \
    --model gpt-5-nano \
    --api-key "$OPENAI_API_KEY"
```

### OpenAI Provider with Ollama (local)

```bash
prompt2json \
    --provider openai \
    --url "http://localhost:11434/v1/chat/completions" \
    --prompt "this is great" \
    --system-instruction "Classify sentiment" \
    --schema '{"type":"object","properties":{"sentiment":{"type":"string"}},"required":["sentiment"]}' \
    --model llama3.2
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

**OpenAI provider:** Requires an API key via `--api-key` flag or `OPENAI_API_KEY` environment variable when using the default OpenAI URL. When `--url` is provided (for local servers like Ollama), the API key is optional.

For complete usage documentation including all options, environment variables, and command line conventions, see the [Usage documentation](https://unitvectory-labs.github.io/prompt2json/usage).

## Attachment Support

The files types of `png`, `jpg`, `jpeg`, `webp`, and `pdf` are supported as inline attachments for both providers. The files are included as inline base64-encoded data in the request payload. The file extension is used to determine the content type, which is sent in the request metadata. Support for attachments varies based on provider and individual LLM that is being used.

## Limitations

- Gemini: Image attachments are limited to 7 MB each before base64 encoding
- Gemini: Total request size is limited to roughly 20 MB
- OpenAI: Attachments are sent inline (`image_url` / `file_data`) and require a model and endpoint that support multimodal Chat Completions
- OpenAI-compatible endpoints (for example, some Ollama setups) may reject multimodal attachment payloads even though text-only requests work
- Limitations of the underlying LLM models apply
