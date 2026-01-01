---
layout: default
title: Installation
nav_order: 2
permalink: /install
---

# Installation

## Using Go

Install directly from the Go toolchain:

```bash
go install github.com/UnitVectorY-Labs/prompt2json@latest
```

## Download Binary

Download pre-built binaries from the [GitHub Releases](https://github.com/UnitVectorY-Labs/prompt2json/releases) page.

Choose the appropriate binary for your platform and add it to your PATH.

## Authentication

`prompt2json` requires Google Cloud credentials to access Gemini models.

Authenticate locally:

```bash
gcloud auth application-default login
```

Or use a service account:

```bash
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/key.json
```

Set your project:

```bash
export GOOGLE_CLOUD_PROJECT=your-project-id
```
