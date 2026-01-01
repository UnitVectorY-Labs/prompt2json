---
layout: default
title: prompt2json
nav_order: 1
permalink: /
---

# prompt2json

Unix-style CLI that brings English-as-Code to your command line. Define behavior in natural language, specify output structure with JSON Schema, and get reliable structured data back for automation and batch processing.

## Why prompt2json?

Traditional solutions for handling unstructured content require either building brittle rule-based systems or training specialized machine learning models. Both approaches demand significant upfront engineering effort and struggle with evolving requirements. Large language models unlock powerful capabilities, but their common chat-oriented interfaces make them difficult to use as reliable building blocks in automation. While agentic workflows address some of these limitations, they introduce additional complexity and are not always a natural fit for simple, composable pipelines.

`prompt2json` takes a different approach. It treats LLMs as flexible cognitive components that can be directed with simple English instructions. You describe what you want (in English), define the output (with JSON Schema), and let the LLM handle the hard part.

The key is predictability. By constraining the LLM's output to match a JSON Schema, you get parsable, structured results that integrate cleanly into shell pipelines, scripts, and data processing workflows. No more parsing freeform text or dealing with unpredictable output formats.

## What it does

- Turn free form prompts into machine reliable JSON for automation and batch workflows
- Enforce output shape using JSON Schema rather than post processing heuristics
- Make Gemini usable in shell pipelines, scripts, and data processing jobs
- Enable repeatable, inspectable prompt experiments from the command line
- Treat LLM calls as deterministic interfaces, not interactive sessions
