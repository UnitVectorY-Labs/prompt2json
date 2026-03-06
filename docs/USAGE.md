---
layout: default
title: Usage
nav_order: 3
permalink: /usage
---

# Usage
{: .no_toc }

## Table of Contents
{: .no_toc .text-delta }

- TOC
{:toc}

The `prompt2json` application follows Unix-style CLI conventions and can be used in shell pipelines, scripts, and data processing jobs.

```
prompt2json [OPTIONS]
```

## Providers

The `--provider` flag is required and determines which API format to use:

| Provider | Description |
|----------|-------------|
| `gemini` | Vertex AI Gemini models |
| `openai` | OpenAI-compatible Chat Completions API |

## Universal Options

These options work with all providers:

| Option                     | Arg   | Required | Notes                                               |
|----------------------------|-------|----------|-----------------------------------------------------|
| `--provider`               | name  | yes      | `gemini` or `openai` (required)                     |
| `--system-instruction`     | text  | yes*     | Exactly one* of this or `--system-instruction-file` |
| `--system-instruction-file`| path  | yes*     | Exactly one* of this or `--system-instruction`      |
| `--schema`                 | json  | yes*     | Exactly one* of this or `--schema-file`             |
| `--schema-file`            | path  | yes*     | Exactly one* of this or `--schema`                  |
| `--schema-profile`         | name  | no       | Override schema profile (see Schema Profile Validation) |
| `--prompt`                 | text  | no       | Mutually exclusive with `--prompt-file`             |
| `--prompt-file`            | path  | no       | Mutually exclusive with `--prompt`                  |
| `--attach`                 | path  | no       | Repeatable. See attachment support by provider      |
| `--model`                  | name  | yes      | Model identifier                                    |
| `--url`                    | url   | no       | Override default API URL                            |
| `--api-key`                | key   | no*      | API key for bearer auth (see authentication)        |
| `--timeout`                | int   | no       | HTTP request timeout in seconds; default is 300 for remote APIs and disabled for localhost URLs |
| `--insecure`               |       | no       | Skip TLS certificate verification; for development or self-signed endpoints only |
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

When `--provider=openai`, using `--project` or `--location` will result in an error.

## OpenAI-only Options

These options only apply when `--provider=openai`:

| Option           | Arg  | Required | Notes                                              |
|------------------|------|----------|----------------------------------------------------|
| `--strict-schema`|      | no       | Enable strict mode for JSON schema validation      |

When `--provider=gemini`, using `--strict-schema` will result in an error.

The `--strict-schema` flag enables OpenAI's [strict mode](https://platform.openai.com/docs/guides/structured-outputs?api-mode=chat) for structured outputs, which will return an error if the JSON schema contains unsupported constructs.

## Schema Profile Validation

Before making any API call, `prompt2json` validates the provided JSON Schema against a provider-specific profile using the [jsonschemaprofiles](https://github.com/UnitVectorY-Labs/jsonschemaprofiles) library. This ensures the schema conforms to the subset of JSON Schema supported by the target provider, catching compatibility issues before a request is sent.

The default profile is automatically selected based on the provider:

| Provider | Default Profile  |
|----------|------------------|
| `openai` | `OPENAI_202602`  |
| `gemini` | `GEMINI_202602`  |

Use the `--schema-profile` flag to override the default:

| Option             | Arg      | Required | Notes                                                     |
|--------------------|----------|----------|-----------------------------------------------------------|
| `--schema-profile` | profile  | no       | Override schema profile for validation                    |

Available profiles: `OPENAI_202602`, `GEMINI_202602`, `GEMINI_202503`, `MINIMAL_202602`

If validation fails, errors are reported to STDERR and the program exits before making any API call.

For detailed documentation on schema limitations for each profile, see:

- [OpenAI schema profile](https://jsonschemaprofiles.unitvectorylabs.com/schemas/openai)
- [Gemini schema profile](https://jsonschemaprofiles.unitvectorylabs.com/schemas/gemini)

## URL Behavior

| Condition | URL Used |
|-----------|----------|
| `--url` provided | Uses the provided URL verbatim |
| `--provider=gemini` (no --url) | Constructed from `--project` and `--location` |
| `--provider=openai` (no --url) | `https://api.openai.com/v1/chat/completions` |

The `--url` flag allows using custom endpoints, including:
- Google Cloud's OpenAI-compatible endpoint
- Ollama local instances
- Any OpenAI-compatible API

When connecting to an HTTPS endpoint with a self-signed or otherwise untrusted certificate, `--insecure` disables TLS certificate verification for the HTTP client. This is intended for local development and test environments and should not be used for production traffic.

## Authentication

| Provider | Default Auth | `--api-key` Behavior |
|----------|--------------|---------------------|
| `gemini` | Google Application Default Credentials (ADC) | Overrides ADC with bearer token |
| `openai` | Required via flag or env (unless --url is used) | Used as bearer token |

**Gemini provider:** Authenticate with:

```bash
gcloud auth application-default login
# or
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/key.json
```

**OpenAI provider:** Provide an API key via:

```bash
--api-key "your-api-key"
# or
export OPENAI_API_KEY="your-api-key"
```

When using `--url` with the openai provider (for local servers like Ollama), the API key is optional.

{: .highlight }
For localhost URLs, the default HTTP timeout is disabled so slower local inference can complete. Set `--timeout` to enforce a deadline, or use `--timeout 0` to disable it explicitly.

## Environment Variables

Options always take precedence over environment variables.

| Option      | Environment Variables                                                     |
|-------------|---------------------------------------------------------------------------|
| `--project` | `GOOGLE_CLOUD_PROJECT`, `CLOUDSDK_CORE_PROJECT`                           |
| `--location`| `GOOGLE_CLOUD_LOCATION`, `GOOGLE_CLOUD_REGION`, `CLOUDSDK_COMPUTE_REGION` |
| `--api-key` | `OPENAI_API_KEY`                                                          |

## Attachment Support

The files types of `png`, `jpg`, `jpeg`, `webp`, and `pdf` are supported as inline attachments for both providers. The files are included as inline base64-encoded data in the request payload. The file extension is used to determine the content type, which is sent in the request metadata. Support for attachments varies based on provider and individual LLM that is being used.

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
- JSON Schema must conform to the provider-specific [schema profile](https://jsonschemaprofiles.unitvectorylabs.com/) (validated before making any API call)
- Attachments must be supported types and within provider-specific limits
- The JSON output will be validated against the provided JSON Schema client side before returning
- Invalid combinations or missing inputs fail before any API call.
