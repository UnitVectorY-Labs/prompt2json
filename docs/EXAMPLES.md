---
layout: default
title: Examples
nav_order: 3
permalink: /examples
---

# Examples

Refer to [installation instructions](./INSTALL.md) to set up `prompt2json` and authenticate with Google Cloud project before running these examples.

## Text Sentiment Analysis

Classify text sentiment from STDIN. The JSON schema enforces structured output with an enum sentiment field and a numeric confidence score.

```bash
echo "this is great" | prompt2json \
    --system-instruction "Classify sentiment as POSITIVE, NEGATIVE, or NEUTRAL" \
    --schema '{"type":"object","properties":{"sentiment":{"type":"string","enum":["POSITIVE","NEGATIVE","NEUTRAL"]},"confidence":{"type":"integer","minimum":0,"maximum":100}},"required":["sentiment","confidence"]}' \
    --location us-central1 \
    --model gemini-2.5-flash
```

Output:

```json
{"sentiment":"POSITIVE","confidence":95}
```

## Image Character Identification

Extract information from an image. This example identifies a character and provides metadata about them.

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

Output:

```json
{
  "name": "Grogu",
  "franchise": "Star Wars",
  "confidence": 99
}
```

## Receipt Data Extraction

Parse a photo of a receipt to extract key transaction details.

```bash
prompt2json \
    --prompt "Extract transaction details from this receipt" \
    --system-instruction "Parse the receipt and extract merchant name, total amount, and transaction date. Use YYYY-MM-DD format for dates." \
    --schema '{"type":"object","properties":{"merchant":{"type":"string"},"total":{"type":"number"},"date":{"type":"string"},"currency":{"type":"string","enum":["USD","EUR","GBP","CAD"]}},"required":["merchant","total","date"]}' \
    --attach receipt.jpg \
    --location us-central1 \
    --model gemini-2.5-flash \
    --pretty-print
```

Output:

```json
{
  "merchant": "Corner Coffee Shop",
  "total": 12.45,
  "date": "2026-01-01",
  "currency": "USD"
}
```

## Resume PDF Parsing

Extract structured data from a resume PDF for candidate screening.

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

Output:

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

## Support Ticket Classification

Automatically categorize incoming support tickets for routing.

```bash
cat ticket.txt | prompt2json \
    --system-instruction "Categorize the support ticket by type and urgency level" \
    --schema '{"type":"object","properties":{"category":{"type":"string","enum":["BILLING","TECHNICAL","ACCOUNT","GENERAL"]},"urgency":{"type":"string","enum":["LOW","MEDIUM","HIGH","CRITICAL"]},"summary":{"type":"string"}},"required":["category","urgency","summary"]}' \
    --location us-central1 \
    --model gemini-2.5-flash
```

Output:

```json
{"category":"TECHNICAL","urgency":"HIGH","summary":"User cannot access dashboard after login"}
```

## PDF Invoice Processing

Extract invoice details from a PDF for automated accounting workflows.

```bash
prompt2json \
    --prompt "Extract invoice information" \
    --system-instruction "Parse the invoice PDF and extract key billing information. Use ISO date format." \
    --schema '{"type":"object","properties":{"invoice_number":{"type":"string"},"vendor":{"type":"string"},"amount":{"type":"number"},"due_date":{"type":"string"},"line_items":{"type":"integer"}},"required":["invoice_number","vendor","amount"]}' \
    --attach invoice.pdf \
    --location us-central1 \
    --model gemini-2.5-flash \
    --pretty-print
```

Output:

```json
{
  "invoice_number": "INV-2026-001",
  "vendor": "Office Supplies Inc",
  "amount": 347.82,
  "due_date": "2026-01-15",
  "line_items": 5
}
```
