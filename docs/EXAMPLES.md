---
layout: default
title: Examples
nav_order: 4
permalink: /examples
---

# Examples
{: .no_toc }

## Table of Contents
{: .no_toc .text-delta }

- TOC
{:toc}

---

Each example demonstrates a different capability. All examples require authentication and project configuration as described in the [installation instructions](./INSTALL.md).

{: .highlight }
The models used in these examples may change over time. Refer to Google's [latest stable models](https://docs.cloud.google.com/vertex-ai/generative-ai/docs/learn/model-versions#latest-stable) for the latest list of available models.

## Text Analysis

Classify text sentiment from STDIN with inline system instruction and JSON schema with output to STDOUT.

{: .note }
System instructions and schema can be provided as inline strings or loaded from external files as shown in later examples.

```bash
echo "this is great" | prompt2json \
    --system-instruction "Classify sentiment as POSITIVE, NEGATIVE, or NEUTRAL" \
    --schema '{"type":"object","properties":{"sentiment":{"type":"string","enum":["POSITIVE","NEGATIVE","NEUTRAL"]},"confidence":{"type":"integer","minimum":0,"maximum":100}},"required":["sentiment","confidence"]}' \
    --location global \
    --model gemini-2.5-flash
```

**Output:**

```json
{"sentiment":"POSITIVE","confidence":95}
```

## Image Processing

Process an image attachment to extract structured information.

{: .note }
Attach a file using the `--attach` flag for the LLM to process directly. Supported formats include `.png`, `.jpg`, `.jpeg`, `.webp`, and `.pdf`.

```bash
prompt2json \
    --prompt "Identify the character in this photo" \
    --system-instruction "Extract the character name, franchise they belong to, and your confidence level" \
    --schema '{"type":"object","properties":{"name":{"type":"string"},"franchise":{"type":"string"},"confidence":{"type":"integer","minimum":0,"maximum":100}},"required":["name","franchise","confidence"]}' \
    --attach character.png \
    --location us-central1 \
    --model gemini-2.5-flash \
    --pretty-print
```

**Output:**

```json
{
  "name": "Grogu",
  "franchise": "Star Wars",
  "confidence": 99
}
```

## PDF Processing

Extract structured data from a PDF document.

```bash
prompt2json \
    --prompt "Resume attached" \
    --system-instruction "Extract basic screening information from the resume. Do not infer missing details." \
    --schema '{"type":"object","properties":{"name":{"type":"string"},"current_role":{"type":"string"},"years_experience":{"type":"integer"},"skills":{"type":"array","items":{"type":"string"}}},"required":["name","current_role","skills"]}' \
    --attach resume.pdf \
    --location us-central1 \
    --model gemini-2.5-flash \
    --pretty-print
```

**Output:**

```json
{
  "name": "Sherlock Holmes",
  "current_role": "Consulting Detective",
  "years_experience": 23,
  "skills": [
    "deductive reasoning",
    "forensic science",
    "observation",
    "chemical analysis"
  ]
}
```

## Using External Files for Instructions and Schema

Load system instructions and JSON schema from files instead of inline strings. This approach is cleaner for complex prompts and reusable schemas.

{: .note }
Instructions file can be stored as a separate text file that is referenced.

`classify_instruction.txt`

```text
Categorize the support ticket by department and priority level.
Use TECHNICAL for infrastructure or software issues.
Use BILLING for payment or invoice questions.
Use ACCOUNT for login or access problems.
Use GENERAL for everything else.
```

{: .note }
Schema file can be stored as a separate JSON file that is referenced.

`classify_schema.json`

```json
{
  "type": "object",
  "properties": {
    "department": {
      "type": "string",
      "enum": ["TECHNICAL", "BILLING", "ACCOUNT", "GENERAL"]
    },
    "priority": {
      "type": "string",
      "enum": ["LOW", "MEDIUM", "HIGH", "CRITICAL"]
    },
    "summary": {
      "type": "string"
    }
  },
  "required": ["department", "priority", "summary"]
}
```

```bash
cat ticket.txt | prompt2json \
    --system-instruction-file classify_instruction.txt \
    --schema-file classify_schema.json \
    --location us-central1 \
    --model gemini-2.5-flash
```

**Output:**

```json
{"department":"TECHNICAL","priority":"HIGH","summary":"User cannot access dashboard after login"}
```

## Files for Input and Output

Process files and save output to a file.


{: .note }
Input file can contain any plain text content to be passed as the prompt.

`notes.txt`

```text
The deployment failed during the final rollout step due to a missing environment variable.
Engineering resolved the issue by updating the configuration and redeploying.
No customer impact was reported, but the release was delayed by two hours.
```

```bash
prompt2json \
  --prompt-file notes.txt \
  --system-instruction "Summarize the incident and extract key facts for reporting. Keep the summary and key facts concise including the important details. Do not invent details." \
  --schema '{"type":"object","properties":{"summary":{"type":"string"},"key_facts":{"type":"array","items":{"type":"string"}}},"required":["summary","key_facts"]}' \
  --location us-central1 \
  --model gemini-2.5-flash \
  --pretty-print \
  --out summary.json
```

**Output:** 

`summary.json`

```json
{
  "key_facts": [
    "Deployment failed during final rollout step.",
    "Cause: Missing environment variable.",
    "Resolution: Engineering updated configuration and redeployed.",
    "Customer Impact: None reported.",
    "Release Delay: Two hours."
  ],
  "summary": "A deployment failed during the final rollout step due to a missing environment variable, causing a two-hour release delay. Engineering resolved the issue with a configuration update and redeployment, and no customer impact was reported."
}
```
