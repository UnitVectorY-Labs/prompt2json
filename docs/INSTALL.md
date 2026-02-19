---
layout: default
title: Installation
nav_order: 2
permalink: /install
---

# Installation
{: .no_toc }

## Table of Contents
{: .no_toc .text-delta }

- TOC
{:toc}

## Installation Methods

There are several ways to install `prompt2json`:

### Download Binary

Download pre-built binaries from the [GitHub Releases](https://github.com/UnitVectorY-Labs/prompt2json/releases) page for the latest version.

[![GitHub release](https://img.shields.io/github/release/UnitVectorY-Labs/prompt2json.svg)](https://github.com/UnitVectorY-Labs/prompt2json/releases/latest) 

Choose the appropriate binary for your platform and add it to your PATH.

### Install Using Go

Install directly from the Go toolchain:

```bash
go install github.com/UnitVectorY-Labs/prompt2json@latest
```

### Build from Source

Build the application from source code:

```bash
git clone https://github.com/UnitVectorY-Labs/prompt2json.git
cd prompt2json
go build -o prompt2json
```

## Authentication

### Gemini Provider

`prompt2json` requires Google Cloud credentials to access Gemini models when using the Gemini provider (`--provider gemini`).

{: .warning }
You will be charged for usage of Gemini models according to [Google Cloud's pricing](https://cloud.google.com/vertex-ai/pricing#generative_ai_models).

Authenticate locally:

```bash
gcloud auth application-default login
```

Or use a service account:

```bash
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/key.json
```

Set your project (can be specified with `--project` flag as well):

```bash
export GOOGLE_CLOUD_PROJECT=your-project-id
```

### OpenAI Provider

{: .warning }
You will be charged for usage of OpenAI models according to [OpenAI's pricing](https://openai.com/api/pricing/).

When using the OpenAI provider (`--provider openai`), an API key is required when using the default OpenAI URL:

```bash
export OPENAI_API_KEY=your-api-key
```

Or provide it directly via the `--api-key` flag.

When using `--url` to specify a custom endpoint (such as a local Ollama server), the API key is optional.
