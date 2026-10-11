---
layout: default
title: Decisions
nav_order: 6
permalink: /decisions
---

# Decisions mode

Decisions mode evaluates Jev-style questions against shared evidence and returns a System One response containing `model`, `answers`, and `usage`. Use native Jev-compatible APIs or simulate the same contract with a traditional LLM. You supply questions instead of an output JSON Schema.

## CLI design

Existing commands continue to use JSON mode. `--mode json` is the default and can be omitted. Opt in to decisions with `--mode decisions`; the program never infers the mode from your model, URL, or input.

| Concern | JSON mode | Decisions mode |
|---------|-----------|----------------|
| Operation | Default, or `--mode json` | `--mode decisions` |
| Task definition | `--system-instruction` / `--system-instruction-file` | Instructions inside each question |
| Output definition | `--schema` / `--schema-file` | `--questions` / `--questions-file` |
| Question execution | Always one native request | One LLM call per question by default; `--batch-questions` combines them |
| Evidence | `--prompt`, `--prompt-file`, or stdin | The same options supply the shared state |
| Input interpretation | Prompt text | `--state-format text` (default), or `json` |
| Transport | `--provider openai` or `gemini` | `openai`, `gemini`, or `systemone` |
| Output | Validated caller-defined JSON | Jev-compatible response envelope |

An explicit mode adds a second operation without changing the existing flag parser or requiring a subcommand migration. Question flags follow the existing inline/file convention. There is no extra backend flag: the provider already selects the wire protocol. `--provider systemone` names the protocol rather than a model or hosting service.

Exactly one of `--questions` and `--questions-file` is required in decisions mode. `--schema`, `--schema-file`, `--system-instruction`, and `--system-instruction-file` are rejected in this mode. The generated schema and instructions belong to the implementation; inspect them with `--show-request-body`.

## Question files

A question file contains **only the object normally placed in the Jev request's `questions` field**. It is a request fragment, not a partial JSON Schema, and does not include `model`, `state`, or an outer `questions` wrapper.

Save this as `questions.json`:

```json
{
  "reachable": {
    "type": "noul",
    "instructions": "Is the service reachable?"
  },
  "severity": {
    "type": "choice",
    "instructions": "How severe is this situation?",
    "criteria": {
      "low": "Healthy; no action needed.",
      "medium": "Degraded; address soon.",
      "high": "Production unavailable, or disk usage at least 95 percent and growing; act now.",
      "unknown": "Insufficient evidence to determine severity."
    }
  },
  "urgency": {
    "type": "score",
    "instructions": "How urgently does this need action?",
    "criteria": [
      "Healthy; no action needed.",
      "Degraded; can wait until the next working day.",
      "Production unavailable, or disk usage at least 95 percent and growing; immediate action needed."
    ]
  }
}
```

The repository includes a reusable [three-question example](https://github.com/UnitVectorY-Labs/prompt2json/blob/main/examples/decisions/questions.json).

Supported question forms follow the [TypeSafe question documentation](https://docs.typesafe.ai/primitives/overview) and [published API schema](https://api.typesafe.ai/openapi.json):

- `noul`: a proposition, returning a probability of true. Optional `criteria` is an object with `true` and/or `false` descriptions.
- `choice`: `criteria` is an object containing 2–255 named options. Descriptions can be strings, objects, arrays, or null.
- `score`: `criteria` is an array containing 2–10 ordered descriptions. Their indices are the levels, starting at zero. Descriptions can be strings, objects, or arrays.

`instructions` may be a string, object, array, null, or omitted, as in the published API schema. Write a clear instruction for each question; application IDs do not explain the question to the model. Question IDs and choice labels must be nonempty. Unknown question fields, invalid types, duplicate JSON keys, and trailing JSON documents are rejected before a request is made. The 2–10 score range follows the documented primitive; this CLI does not support the single-level score allowed by the current OpenAPI schema.

Descriptions can contain structured guidance without flattening it:

```json
{
  "route": {
    "type": "choice",
    "instructions": {"task": "Select the team handling the main issue"},
    "criteria": {
      "billing": {"covers": ["Invoices", "Refunds"], "excludes": "Delivery tracking"},
      "shipping": "Delivery and tracking",
      "other": null
    }
  }
}
```

## Native System One

Set `SYSTEMONE_API_KEY` to your TypeSafe API key. The example uses the [official Jev API](https://api.typesafe.ai/openapi.json) endpoint and model alias:

```bash
printf '%s\n' 'The service is reachable. The disk usage is at 97 percent and growing.' |
  prompt2json --mode decisions \
    --provider systemone \
    --url https://api.typesafe.ai/v1/systemone \
    --api-key "$SYSTEMONE_API_KEY" \
    --model jev-latest \
    --questions-file questions.json \
    --pretty-print
```

Without `--url`, the endpoint is `https://api.typesafe.ai/v1/systemone`. Set `--api-key` or `SYSTEMONE_API_KEY` for bearer authentication. A key is optional with a custom URL. This provider does not read `OPENAI_API_KEY`.

The request contains `model`, `state`, and `questions`. Native mode has no generated schema or system instruction. `--attach`, `--strict-schema`, `--schema-profile`, `--project`, and `--location` are rejected for this provider. Native multimodal extensions are outside this implementation.

## Traditional LLM simulation

Use the same questions with the official OpenAI Chat Completions endpoint. Set `OPENAI_API_KEY` to your OpenAI API key and `OPENAI_MODEL` to a model supporting strict structured outputs:

```bash
printf '%s\n' 'The service is reachable. The disk usage is at 97 percent and growing.' |
  prompt2json --mode decisions \
    --provider openai \
    --url https://api.openai.com/v1/chat/completions \
    --api-key "$OPENAI_API_KEY" \
    --model "$OPENAI_MODEL" \
    --questions-file questions.json \
    --pretty-print
```

By default, simulation sends each question in a separate, sequential API call with the same state and attachments. Each call receives only that question's instructions and a generated schema for its answer. The application validates every answer, restores question IDs, derives the remaining fields, and assembles one final response. Input and output token counts are summed across calls, including repeated state and instructions. All calls must report the same resolved model. A failure stops execution without emitting partial answers or replacing an existing output file.

Add `--batch-questions` to send all questions in one LLM call. This reduces repeated input and request overhead, but all questions share a model context. The flag applies only to decisions simulation with `openai` or `gemini`; `systemone` always sends the complete questions object in one native request and rejects this flag.

For Vertex AI, select `--provider gemini` and supply the usual `--project`, `--location`, and `--model` options. Authentication and endpoint overrides behave as in JSON mode. Attachments are supported through the existing generative-provider formats, subject to the selected model's capabilities.

The compiler builds a closed JSON Schema from your questions and validates it against the provider's schema profile. OpenAI simulation always sends `strict: true`; the existing `--strict-schema` flag remains optional in JSON mode. Gemini uses its JSON response MIME type and schema. `--schema-profile` can override profile validation for either simulation provider.

The question definitions are placed in the system instruction. State is encoded separately as a JSON object in the user message. Original question IDs are replaced with opaque `q0`, `q1`, etc. aliases, then restored in the final response. The model outputs only a `noul` probability or a distribution for each question. It does not generate metadata or derived fields.

This is a translation into text generation, not native decision inference. Probabilities are generated estimates, and a valid schema does not establish calibration or correctness. With `--batch-questions`, questions share one generated response, so independent evaluation is requested in the prompt but is not guaranteed. Separate calls isolate question contexts but do not guarantee statistical independence or calibrated probabilities. Instructions embedded in state are treated as evidence by the generated system instruction; schema constraints alone cannot establish resistance to prompt injection. Define missing-evidence behavior in your questions and include an `unknown` or `other` choice when appropriate. Evaluate your own labeled examples before choosing thresholds.

`--provider openai` continues to mean Chat Completions, including in decisions mode. It does **not** call OpenAI's separate `/v1/decisions` endpoint; a native adapter for that protocol is outside this change.

## State input

The default is text, including text that happens to look like JSON. Use `--state-format json` to parse the input as a JSON string, object, or array:

```bash
printf '%s\n' '{"service":"unreachable","disk_usage_percent":97,"trend":"growing"}' |
  prompt2json --mode decisions --state-format json \
    --provider systemone --model jev-latest \
    --url https://api.typesafe.ai/v1/systemone \
    --api-key "$SYSTEMONE_API_KEY" \
    --questions-file questions.json
```

The same interpretation applies to `--prompt` and `--prompt-file`. JSON numbers, booleans, null, duplicate keys, and multiple documents are rejected as state. State input is required and must be nonempty. All flags must precede any positional argument; this CLI accepts no positional input.

## Output and validation

Both execution paths emit the same envelope. Probabilities, `noul`, `score`, and `confidence` use decimal notation in the final JSON, including values such as `0.0`, `1.0`, and `2.0`. Integer-valued JSON numbers from a model are accepted as numbers; the application computes derived fields and formats them deterministically. JSON itself has a single number type, so changing `1` to `1.0` does not change its value. Token counts remain integers and configured legends are preserved. This example illustrates the format rather than a guaranteed model result:

```json
{
  "model": "resolved-model-name",
  "answers": {
    "reachable": {"type": "noul", "noul": 0.99},
    "severity": {
      "type": "choice",
      "choice": "high",
      "probabilities": {"high": 0.97, "low": 0.01, "medium": 0.01, "unknown": 0.01},
      "confidence": 0.96
    },
    "urgency": {
      "type": "score",
      "score": 1.9,
      "legend": {"0": "Healthy; no action needed.", "1": "Degraded; can wait until the next working day.", "2": "Production unavailable, or disk usage at least 95 percent and growing; immediate action needed."},
      "probabilities": {"0": 0.0, "1": 0.1, "2": 0.9},
      "confidence": 0.85
    }
  },
  "usage": {"input_tokens": 200, "output_tokens": 100}
}
```

Validation requires exactly one answer per question, matching answer types, finite probabilities in `[0,1]`, and exactly the configured options or levels. Distributions must sum to 1 within an absolute tolerance of `0.0001`. Drift within that tolerance is normalized; larger errors fail rather than being silently repaired.

The normalizer derives fields for **both** native and simulated answers:

- `choice` is the highest-probability label. Exact ties use lexicographic label order.
- `score` is the weighted mean of zero-based level indices.
- `legend` is copied from the configured score criteria.
- Choice and score confidence use the [documented TypeSafe formulas](https://docs.typesafe.ai/confidence). Score confidence accounts for distance between levels. A tied score distribution uses the lowest most-likely level when computing confidence.

Native responses must include valid confidence fields; native choices must identify a highest-probability option, scores must match their distributions within rounding tolerance, and legends must match the requested criteria. Confidence is recomputed from the normalized distribution rather than trusting a backend-specific value. Additional answer fields are not forwarded. Refusals and incompatible answer types fail validation; they are not converted into fabricated probabilities.

`model` uses the resolved model supplied by the API. If Vertex omits `modelVersion`, it uses the requested model identifier. `usage` uses real reported token counts: Chat Completions prompt/completion counts are mapped to input/output counts; Gemini output counts include reported candidate and thought tokens. Simulation usage contains these two counts. Native usage is preserved, including additional fields. Nothing assumes output tokens are zero.

The existing Unix output conventions apply: minified JSON on stdout, `--pretty-print` for formatting, `--out` for a file, and diagnostics on stderr. Validation failures produce no stdout and leave an existing output file untouched. Exit codes remain 0 success, 2 CLI usage, 3 input, 4 response/validation, and 5 API/authentication.

For pipelines, select an answer explicitly:

```bash
printf '%s\n' 'The service is reachable.' |
  prompt2json --mode decisions --provider systemone --model jev-latest \
    --url https://api.typesafe.ai/v1/systemone \
    --api-key "$SYSTEMONE_API_KEY" \
    --questions '{"reachable":{"type":"noul","instructions":"Is the service reachable?"}}' |
  jq '.answers.reachable.noul'
```

Inspect native or simulated requests without authentication or network calls:

```bash
prompt2json --mode decisions --provider openai --model example-model \
  --questions-file questions.json --prompt 'Example evidence' \
  --show-request-body --pretty-print
```

In the default separate-call mode with multiple questions, `--show-request-body` returns a JSON array of `{ "question_id": "...", "request": { ... } }` entries, one per planned call. With `--batch-questions` or a single question, it returns the single API request body. No dry-run request is sent.
