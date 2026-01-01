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

## Options

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
| `--timeout`                | int   | no       | HTTP request timeout in seconds; default is 60      |
| `--out`                    | path  | no       | Output file path; defaults to STDOUT if not set.    |
| `--pretty-print`           |       | no       | Pretty-print JSON output; default is minified       |
| `--verbose`                |       | no       | Logs additional information to STDERR               |
| `--version`                |       | no       | Print version and exit                              |
| `--help`                   |       | no       | Print help and exit                                 |

## Environment Variables

Options always take precedence over environment variables.

| Option      | Environment Variables                                                     |
|-------------|---------------------------------------------------------------------------|
| `--project` | `GOOGLE_CLOUD_PROJECT`, `CLOUDSDK_CORE_PROJECT`                           |
| `--location`| `GOOGLE_CLOUD_LOCATION`, `GOOGLE_CLOUD_REGION`, `CLOUDSDK_COMPUTE_REGION` |

## Command Line

The `prompt2json` CLI follows standard UNIX conventions for input and output to facilitate easy integration with other command-line tools enabling chaining and composition of commands.

- STDIN is used as the prompt when neither `--prompt` nor `--prompt-file` is provided
- STDOUT emits the final JSON result when `--out` is not specified
- STDERR is reserved for logs, errors, and verbose output

The output will always be re-encoded as minified JSON by default unless `--pretty-print` is specified.

Exit status: 0 success, 2 usage, 3 input, 4 validation/response, 5 API/auth

## Validation rules

- Exactly one system instruction source is required
- Exactly one schema source is required
- Prompt is read from a flag or STDIN and must be non empty
- JSON Schema must be valid and compilable
- Attachments must be supported types and within size limits
- The JSON output will be validated against the provided JSON Schema client side before returning
- Invalid combinations or missing inputs fail before any API call.
