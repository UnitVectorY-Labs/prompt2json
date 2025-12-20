[![License](https://img.shields.io/badge/license-MIT-blue.svg)](https://opensource.org/licenses/MIT) [![Work In Progress](https://img.shields.io/badge/Status-Work%20In%20Progress-yellow)](https://guide.unitvectorylabs.com/bestpractices/status/#work-in-progress)
 [![Go Report Card](https://goreportcard.com/badge/github.com/UnitVectorY-Labs/prompt2json)](https://goreportcard.com/report/github.com/UnitVectorY-Labs/prompt2json)

# prompt2json

Unix-style CLI that sends a system instruction, required JSON Schema, and text or file inputs to Vertex AI (Gemini models) and returns schema-validated JSON for easy batch processing.

## Overview

`prompt2json` is designed for composable command line workflows:

- Turn free form prompts into machine reliable JSON for automation and batch workflows
- Enforce output shape using a JSON Schema rather than post processing heuristics
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
echo "this is great" | ./prompt2json \
    --system-instruction "Classify sentiment" \
    --schema '{"type":"object","properties":{"sentiment":{"type":"string","enum":["POSITIVE","NEGATIVE","NEUTRAL"]},"confidence":{"type":"integer","minimum":0,"maximum":100}},"required":["sentiment","confidence"]}' \
    --location us-central1 \
    --model gemini-2.5-flash
```

The output will be a JSON object returned to the standard output:

```json
{"sentiment": "POSITIVE", "confidence": 95}
```

## Usage

The `prompt2json` application follows unix-style CLI conventions and can be used in shell pipelines, scripts, and data processing jobs.

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

### Options

| Options                    | Arg   | Required | Notes                                               |
|----------------------------|-------|----------|-----------------------------------------------------|
| `--system-instruction`     | text  | yes*     | Exactly one* of this or `--system-instruction-file` |
| `--system-instruction-file`| path  | yes*     | Exactly one* of this or `--system-instruction`      |
| `--schema`                 | json  | yes*     | Exactly one* of this or `--schema-file`             |
| `--schema-file`            | path  | yes*     | Exactly one* of this or `--schema`                  |
| `--prompt`                 | text  | no       | Mutually exclusive with `--prompt-file`             |
| `--prompt-file`            | path  | no       | Mutually exclusive with `--prompt`                  |
| `--attach`                 | path  | no       | Repeatable. `.png .jpg .jpeg .webp .pdf`            |
| `--project`                | id    | yes      | Environment variable fallback supported             |
| `--location`               | region| yes      | Environment variable fallback supported             |
| `--model`                  | name  | yes      | Gemini model id                                     |
| `--out`                    | path  | no       | Output file path; defaults to STDOUT if not set.    |
| `--verbose`                |       | no       | Logs additional information to STDERR               |
| `--version`                |       | no       | Print version and exit                              |
| `--help`                   |       | no       | Print help and exit                                 |

### Environment Variables

Options always take precedence over environment variables.

| Option      | Environment Variables                                                     |
|-------------|---------------------------------------------------------------------------|
| `--project` | `GOOGLE_CLOUD_PROJECT`, `CLOUDSDK_CORE_PROJECT`                           |
| `--location`| `GOOGLE_CLOUD_LOCATION`, `GOOGLE_CLOUD_REGION`, `CLOUDSDK_COMPUTE_REGION` |

### Command Line

The `prompt2json` CLI follows standard UNIX conventions for input and output to facilitate easy integration with other command-line tools enabling chaining and composition of commands.

- STDIN is used as the prompt when neither `--prompt` nor `--prompt-file` is provided
- STDOUT emits the final JSON result when `--out` is not specified
- STDERR is reserved for logs, errors, and verbose output

Exit status: 0 success, 2 usage, 3 input, 4 validation/response, 5 API/auth

## Validation rules

- Exactly one system instruction source is required
- Exactly one schema source is required
- Prompt is read from a flag or STDIN and must be non empty
- JSON Schema must be valid and compilable
- Attachments must be supported types and within size limits
- Invalid combinations or missing inputs fail before any API call.

## Limitations

- Image attachments are limited to 7 MB each before base64 encoding
- Total request size is limited to roughly 20 MB
- Supported attachment types are PNG, JPEG, WebP, and PDF
- Limitations of Gemini models apply
