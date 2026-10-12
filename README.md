[![GitHub release](https://img.shields.io/github/release/UnitVectorY-Labs/prompt2json.svg)](https://github.com/UnitVectorY-Labs/prompt2json/releases/latest) [![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://opensource.org/licenses/MIT) [![Active](https://img.shields.io/badge/Status-Active-green)](https://guide.unitvectorylabs.com/bestpractices/status/#active) 

# prompt2json

Unix-style CLI that sends a system instruction, required JSON Schema, and text or inline file inputs to LLM APIs and returns schema-validated JSON for easy batch processing. Supports Vertex AI (Gemini) and OpenAI-compatible Chat Completions endpoints.

An optional `--mode decisions` accepts Jev-style questions instead of a schema and system instruction. It returns Jev-compatible answers using a native System One API or structured-output simulation through either existing provider. Existing invocations keep their original behavior.

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
| `systemone` | Native Jev-compatible decisions (`--mode decisions` required) | `https://api.typesafe.ai/v1/systemone` |

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
    --schema '{"type":"object","properties":{"sentiment":{"type":"string","enum":["POSITIVE","NEGATIVE","NEUTRAL"]},"confidence":{"type":"integer","minimum":0,"maximum":100}},"required":["sentiment","confidence"],"additionalProperties":false}' \
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
    --schema '{"type":"object","properties":{"sentiment":{"type":"string","enum":["POSITIVE","NEGATIVE","NEUTRAL"]},"confidence":{"type":"integer","minimum":0,"maximum":100}},"required":["sentiment","confidence"],"additionalProperties":false}' \
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
    --schema '{"type":"object","properties":{"sentiment":{"type":"string"}},"required":["sentiment"],"additionalProperties":false}' \
    --model llama3.2
```

The output will be minified JSON matching the specified schema:

```json
{"sentiment":"POSITIVE","confidence":95}
```

### Decisions mode

Questions use the Jev `questions` object, supplied inline or from a file. The existing prompt options and stdin supply the shared state. No output schema is needed. Set `SYSTEMONE_API_KEY` to your TypeSafe API key before calling the official Jev endpoint.

```bash
printf '%s\n' 'The service is reachable. The disk usage is at 97 percent and growing.' |
  prompt2json --mode decisions \
    --provider systemone \
    --url https://api.typesafe.ai/v1/systemone \
    --api-key "$SYSTEMONE_API_KEY" \
    --model jev-latest \
    --questions-file examples/decisions/questions.json \
    --pretty-print
```

To simulate the same questions through an ordinary LLM, change the transport and model. Set `OPENAI_API_KEY` to your OpenAI API key and `OPENAI_MODEL` to a model supporting strict structured outputs:

```bash
printf '%s\n' 'The service is reachable. The disk usage is at 97 percent and growing.' |
  prompt2json --mode decisions \
    --provider openai \
    --url https://api.openai.com/v1/chat/completions \
    --api-key "$OPENAI_API_KEY" \
    --model "$OPENAI_MODEL" \
    --questions-file examples/decisions/questions.json \
    --pretty-print
```

Both produce `{"model": "...", "answers": {...}, "usage": {...}}`. The application derives choices, scores, confidence, and score legends from validated probabilities. LLM simulation makes one call per question by default; add `--batch-questions` for one combined call. Native System One requests always include all questions. Simulation probabilities are generated estimates; matching the format does not establish calibration or native inference performance.

See [Decisions mode](docs/DECISIONS.md) for question formats, CLI design, JSON state input, normalization, and limitations. `--provider openai` uses Chat Completions; a native OpenAI Decisions API adapter is not included.

## Usage

```
prompt2json [OPTIONS]
```

Missing required options include a `prompt2json --help` hint. Help uses color on terminals; set `NO_COLOR` or redirect stderr for plain text. Color is limited to help. Decisions question files and inline questions are validated against the embedded [questions JSON Schema](docs/questions.schema.json).

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

**System One provider:** Uses `--api-key` or `SYSTEMONE_API_KEY`. A key is required for the default endpoint and optional with `--url`.

For complete usage documentation including all options, environment variables, and command line conventions, see the [Usage documentation](https://unitvectory-labs.github.io/prompt2json/usage).

## Attachment Support

The file types of `png`, `jpg`, `jpeg`, `webp`, and `pdf` are supported as inline attachments for the Gemini and OpenAI providers, in either mode. The files are included as inline base64-encoded data in the request payload. The file extension is used to determine the content type, which is sent in the request metadata. Support for attachments varies based on provider and individual LLM that is being used. The native System One provider does not support attachments.

## Limitations

- Gemini: Image attachments are limited to 7 MB each before base64 encoding
- Gemini: Total request size is limited to roughly 20 MB
- OpenAI: Attachments are sent inline (`image_url` / `file_data`) and require a model and endpoint that support multimodal Chat Completions
- OpenAI-compatible endpoints (for example, some Ollama setups) may reject multimodal attachment payloads even though text-only requests work
- Limitations of the underlying LLM models apply
