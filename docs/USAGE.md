---
layout: default
title: Usage
nav_order: 3
permalink: /usage
---

# Usage

The `prompt2json` application follows Unix-style CLI conventions and can be used in shell pipelines, scripts, and data processing jobs.

```
prompt2json [OPTIONS]
```

## Providers

| Provider | Description |
|----------|-------------|
| `gemini` (default) | Vertex AI Gemini models |
| `openapi` | OpenAI-compatible Chat Completions API |

## Universal Options

These options work with all providers:

| Option                     | Arg   | Required | Notes                                               |
|----------------------------|-------|----------|-----------------------------------------------------|
| `--provider`               | name  | no       | `gemini` (default) or `openapi`                     |
| `--system-instruction`     | text  | yes*     | Exactly one* of this or `--system-instruction-file` |
| `--system-instruction-file`| path  | yes*     | Exactly one* of this or `--system-instruction`      |
| `--schema`                 | json  | yes*     | Exactly one* of this or `--schema-file`             |
| `--schema-file`            | path  | yes*     | Exactly one* of this or `--schema`                  |
| `--prompt`                 | text  | no       | Mutually exclusive with `--prompt-file`             |
| `--prompt-file`            | path  | no       | Mutually exclusive with `--prompt`                  |
| `--attach`                 | path  | no       | Repeatable. See attachment support by provider      |
| `--model`                  | name  | yes      | Model identifier                                    |
| `--url`                    | url   | no       | Override default API URL                            |
| `--api-key`                | key   | no*      | API key for bearer auth (required for openapi)      |
| `--timeout`                | int   | no       | HTTP request timeout in seconds; default is 60      |
| `--out`                    | path  | no       | Output file path; defaults to STDOUT if not set     |
| `--pretty-print`           |       | no       | Pretty-print JSON output; default is minified       |
| `--show-url`               |       | no       | Output the API URL without making the request       |
| `--show-request-body`      |       | no       | Output the JSON request body without making request |
| `--verbose`                |       | no       | Logs additional information to STDERR               |
| `--version`                |       | no       | Print version and exit                              |
| `--help`                   |       | no       | Print help and exit                                 |

## Gemini-only Options

These options only apply when `--provider=gemini`:

| Option      | Arg    | Required | Notes                                    |
|-------------|--------|----------|------------------------------------------|
| `--project` | id     | yes*     | GCP project ID (unless --url is provided)|
| `--location`| region | yes*     | GCP region (unless --url is provided)    |

When `--provider=openapi`, using `--project` or `--location` will result in an error.

## URL Behavior

| Condition | URL Used |
|-----------|----------|
| `--url` provided | Uses the provided URL verbatim |
| `--provider=gemini` (no --url) | Constructed from `--project` and `--location` |
| `--provider=openapi` (no --url) | `https://api.openai.com/v1/chat/completions` |

The `--url` flag allows using custom endpoints, including:
- Google Cloud's OpenAI-compatible endpoint
- Ollama local instances
- Any OpenAI-compatible API

## Authentication

| Provider | Default Auth | `--api-key` Behavior |
|----------|--------------|---------------------|
| `gemini` | Google Application Default Credentials (ADC) | Overrides ADC with bearer token |
| `openapi` | None (required via flag or env) | Used as bearer token |

**Gemini provider:** Authenticate with:

```bash
gcloud auth application-default login
# or
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/key.json
```

**OpenAPI provider:** Provide an API key via:

```bash
--api-key "your-api-key"
# or
export OPENAI_API_KEY="your-api-key"
```

## Environment Variables

Options always take precedence over environment variables.

| Option      | Environment Variables                                                     |
|-------------|---------------------------------------------------------------------------|
| `--project` | `GOOGLE_CLOUD_PROJECT`, `CLOUDSDK_CORE_PROJECT`                           |
| `--location`| `GOOGLE_CLOUD_LOCATION`, `GOOGLE_CLOUD_REGION`, `CLOUDSDK_COMPUTE_REGION` |
| `--api-key` | `OPENAI_API_KEY`                                                          |

## Attachment Support

| Provider | Supported Types | Limits |
|----------|-----------------|--------|
| `gemini` | `.png`, `.jpg`, `.jpeg`, `.webp`, `.pdf` | 7 MB per image, 20 MB total |
| `openapi` | Not supported | Text prompts only |

Using `--attach` with `--provider=openapi` will result in an error.

## Command Line

The `prompt2json` CLI follows standard UNIX conventions for input and output to facilitate easy integration with other command-line tools enabling chaining and composition of commands.

- STDIN is used as the prompt when neither `--prompt` nor `--prompt-file` is provided
- STDOUT emits the final JSON result when `--out` is not specified
- STDERR is reserved for logs, errors, and verbose output

The output will always be re-encoded as minified JSON by default unless `--pretty-print` is specified.

Exit status: 0 success, 2 usage, 3 input, 4 validation/response, 5 API/auth

## Dry-run Modes

The dry-run options allow you to inspect the API request that would be made without actually sending it. These are useful for debugging, testing, and understanding the exact request structure.

- `--show-url` outputs the complete URL endpoint that would be called
- `--show-request-body` outputs the JSON payload that would be sent in the request body

When using either dry-run option:
- The API request is not performed
- No authentication is required
- Output goes to STDOUT or the file specified by `--out`
- The `--pretty-print` flag can be used with `--show-request-body` to format the JSON

Both dry-run modes work with all providers.

## Validation rules

- Exactly one system instruction source is required
- Exactly one schema source is required
- Prompt is read from a flag or STDIN and must be non empty
- JSON Schema must be valid and compilable
- Attachments must be supported types and within size limits (gemini only)
- The JSON output will be validated against the provided JSON Schema client side before returning
- Invalid combinations or missing inputs fail before any API call.
