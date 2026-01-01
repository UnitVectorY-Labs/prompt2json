[![GitHub release](https://img.shields.io/github/release/UnitVectorY-Labs/prompt2json.svg)](https://github.com/UnitVectorY-Labs/prompt2json/releases/latest) [![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://opensource.org/licenses/MIT) [![Active](https://img.shields.io/badge/Status-Active-green)](https://guide.unitvectorylabs.com/bestpractices/status/#active)
 [![Go Report Card](https://goreportcard.com/badge/github.com/UnitVectorY-Labs/prompt2json)](https://goreportcard.com/report/github.com/UnitVectorY-Labs/prompt2json)

# prompt2json

Unix-style CLI that sends a system instruction, required JSON Schema, and text or file inputs to Vertex AI (Gemini models) and returns schema-validated JSON for easy batch processing.

## Overview

`prompt2json` is designed for composable command line workflows:

- Turn free form prompts into machine reliable JSON for automation and batch workflows
- Enforce output shape using JSON Schema rather than post processing heuristics
- Make Gemini usable in shell pipelines, scripts, and data processing jobs
- Enable repeatable, inspectable prompt experiments from the command line
- Treat LLM calls as deterministic interfaces, not interactive sessions

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

The following example is a simple demonstration of how input text can be classified specifying the systems instructions and critically the JSON schema that defines and enforces the expected output structure.

```bash
export GOOGLE_CLOUD_PROJECT=example-project
echo "this is great" | prompt2json \
    --system-instruction "Classify sentiment" \
    --schema '{"type":"object","properties":{"sentiment":{"type":"string","enum":["POSITIVE","NEGATIVE","NEUTRAL"]},"confidence":{"type":"integer","minimum":0,"maximum":100}},"required":["sentiment","confidence"]}' \
    --location us-central1 \
    --model gemini-2.5-flash
```

The output will be minified JSON matching the specified schema:

```json
{"sentiment":"POSITIVE","confidence":95}
```

## Usage

The `prompt2json` application follows Unix-style CLI conventions and can be used in shell pipelines, scripts, and data processing jobs.

```
prompt2json [OPTIONS]
```

### Authentication

`prompt2json` uses Google Application Default Credentials.

Authenticate locally with:

```bash
gcloud auth application-default login
```

Or via service account:

```bash
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/key.json
```

For complete usage documentation including all options, environment variables, and command line conventions, see the [Usage documentation](https://unitvectory-labs.github.io/prompt2json/usage).

## Limitations

- Image attachments are limited to 7 MB each before base64 encoding
- Total request size is limited to roughly 20 MB
- Supported attachment types are PNG, JPEG, WebP, and PDF
- Limitations of Gemini models apply
