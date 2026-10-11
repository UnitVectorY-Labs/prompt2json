package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testQuestions = `{
  "flag": {"type":"noul","instructions":"Is the service reachable?","criteria":{"true":"Reachable","false":"Not reachable"}},
  "route": {"type":"choice","instructions":{"task":"Which team?"},"criteria":{"a":null,"b":"Billing","c":{"description":"Technical"}}},
  "severity": {"type":"score","instructions":"How severe?","criteria":["Cosmetic","Workaround","Blocked"]}
}`

func mustQuestions(t *testing.T, text string) map[string]DecisionQuestion {
	t.Helper()
	questions, err := parseDecisionQuestions([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return questions
}

func TestParseDecisionQuestions(t *testing.T) {
	questions := mustQuestions(t, testQuestions)
	if len(questions) != 3 || questions["route"].Criteria.(map[string]any)["a"] != nil {
		t.Fatalf("structured questions changed: %#v", questions)
	}
	// The published API allows absent instructions and nullable descriptions.
	mustQuestions(t, `{"flag":{"type":"noul","criteria":{"true":["Evidence",{"example":true}]}},"route":{"type":"choice","criteria":{"a":null,"b":null}}}`)
	bad := []string{
		`{}`, `[]`, `null`, `{"questions":{}}`, `{"q":{}}`,
		`{"q":{"type":"boolean"}}`, `{"q":{"type":"noul","instruction":"typo"}}`,
		`{"q":{"type":"noul","instructions":true}}`,
		`{"q":{"type":"noul","criteria":{"yes":"Wrong key"}}}`,
		`{"q":{"type":"choice","criteria":["a","b"]}}`,
		`{"q":{"type":"choice","criteria":{"a":null}}}`,
		`{"q":{"type":"choice","criteria":{"a":3,"b":null}}}`,
		`{"q":{"type":"score","criteria":["one"]}}`,
		`{"q":{"type":"score","criteria":["one",null]}}`,
		`{"q":{"type":"noul"},"q":{"type":"noul"}}`,
		`{"q":{"type":"choice","criteria":{"a":null,"a":null,"b":null}}}`,
		`{"q":{"type":"noul"}} {}`, `{" ":{"type":"noul"}}`,
	}
	for _, text := range bad {
		t.Run(text, func(t *testing.T) {
			if _, err := parseDecisionQuestions([]byte(text)); err == nil {
				t.Fatalf("accepted malformed questions: %s", text)
			}
		})
	}
	for _, limit := range []struct {
		kind  string
		count int
	}{{"choice", 256}, {"score", 11}} {
		var criteria any
		if limit.kind == "choice" {
			options := map[string]any{}
			for i := range limit.count {
				options[fmt.Sprint(i)] = nil
			}
			criteria = options
		} else {
			levels := make([]string, limit.count)
			criteria = levels
		}
		data, _ := json.Marshal(map[string]any{"q": map[string]any{"type": limit.kind, "criteria": criteria}})
		if _, err := parseDecisionQuestions(data); err == nil {
			t.Fatalf("accepted too many %s criteria", limit.kind)
		}
	}
}

func TestNormalizeDecisionAnswer(t *testing.T) {
	questions := mustQuestions(t, testQuestions)
	choice, err := normalizeDecisionAnswer(questions["route"], map[string]any{"probabilities": map[string]any{"a": 0.1, "b": 0.3, "c": 0.6}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if choice["choice"] != "c" || math.Abs(choice["confidence"].(float64)-0.4) > 1e-12 {
		t.Fatalf("wrong derived choice: %#v", choice)
	}
	score, err := normalizeDecisionAnswer(questions["severity"], map[string]any{"probabilities": map[string]any{"0": 0.1, "1": 0.7, "2": 0.2}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(score["score"].(float64)-1.1) > 1e-12 || math.Abs(score["confidence"].(float64)-0.55) > 1e-12 {
		t.Fatalf("wrong derived score: %#v", score)
	}
	// Adjacent and distant ambiguity have different score confidence.
	for _, test := range []struct {
		ps         []float64
		confidence float64
	}{{[]float64{0, 0.5, 0.5}, 0.25}, {[]float64{0.5, 0, 0.5}, 0}} {
		answer, err := normalizeDecisionAnswer(questions["severity"], map[string]any{"probabilities": map[string]any{"0": test.ps[0], "1": test.ps[1], "2": test.ps[2]}}, false)
		if err != nil || math.Abs(answer["confidence"].(float64)-test.confidence) > 1e-12 {
			t.Fatalf("score confidence: %#v, %v", answer, err)
		}
	}
	// Rounding drift is normalized; ties resolve in sorted label order.
	tie := mustQuestions(t, `{"q":{"type":"choice","criteria":{"z":null,"a":null}}}`)["q"]
	answer, err := normalizeDecisionAnswer(tie, map[string]any{"probabilities": map[string]any{"a": 0.50001, "z": 0.50001}}, false)
	if err != nil || answer["choice"] != "a" || answer["confidence"] != float64(0) {
		t.Fatalf("tie: %#v, %v", answer, err)
	}
	levels := make([]any, 10)
	ps := map[string]any{}
	for i := range levels {
		levels[i] = fmt.Sprint(i)
		ps[fmt.Sprint(i)] = float64(0)
	}
	ps["9"] = float64(1)
	answer, err = normalizeDecisionAnswer(DecisionQuestion{Type: "score", Criteria: levels}, map[string]any{"probabilities": ps}, false)
	if err != nil || answer["score"] != float64(9) || answer["confidence"] != float64(1) {
		t.Fatalf("10-level score: %#v, %v", answer, err)
	}
}

func TestDecisionAnswerRejectsInvalidData(t *testing.T) {
	q := mustQuestions(t, testQuestions)["route"]
	for _, ps := range []map[string]any{
		{"a": 0.1, "b": 0.1, "c": 0.1}, {"a": 0.0, "b": 0.0, "c": 0.0},
		{"a": -0.1, "b": 0.5, "c": 0.6}, {"a": 1.1, "b": 0.0, "c": 0.0},
		{"a": 0.1, "b": 0.3, "d": 0.6}, {"a": 0.1, "b": 0.9},
		{"a": "0.1", "b": 0.3, "c": 0.6}, {"a": math.NaN(), "b": 0.3, "c": 0.6},
	} {
		if _, err := normalizeDecisionAnswer(q, map[string]any{"probabilities": ps}, false); err == nil {
			t.Fatalf("accepted distribution: %#v", ps)
		}
	}
	native := map[string]any{"type": "choice", "choice": "a", "confidence": 0.9, "probabilities": map[string]any{"a": 0.1, "b": 0.3, "c": 0.6}}
	if _, err := normalizeDecisionAnswer(q, native, true); err == nil {
		t.Fatal("accepted inconsistent native choice")
	}
	for _, p := range []any{nil, "0.5", -0.1, 1.1, math.Inf(1)} {
		if _, err := normalizeDecisionAnswer(DecisionQuestion{Type: "noul"}, map[string]any{"noul": p}, false); err == nil {
			t.Fatalf("accepted noul %v", p)
		}
	}
	score := mustQuestions(t, testQuestions)["severity"]
	native = map[string]any{"type": "score", "score": 2.0, "confidence": 0.9, "legend": map[string]any{"0": "Cosmetic", "1": "Workaround", "2": "Blocked"}, "probabilities": map[string]any{"0": 0.1, "1": 0.7, "2": 0.2}}
	if _, err := normalizeDecisionAnswer(score, native, true); err == nil {
		t.Fatal("accepted inconsistent native score")
	}
	native["score"] = 1.1
	native["legend"] = map[string]any{}
	if _, err := normalizeDecisionAnswer(score, native, true); err == nil {
		t.Fatal("accepted incorrect native legend")
	}
}

// Run the real CLI in a fresh process to exercise flag parsing, stdin, error
// exit codes, request construction, normalization, and output atomicity together.
func TestDecisionCLIProcess(t *testing.T) {
	if os.Getenv("PROMPT2JSON_TEST_CLI") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{os.Args[0]}, os.Args[i+1:]...)
			main()
			os.Exit(0)
		}
	}
	os.Exit(99)
}

func runDecisionCLI(t *testing.T, stdin string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"-test.run=^TestDecisionCLIProcess$", "--"}, args...)...)
	for _, variable := range os.Environ() {
		if !strings.HasPrefix(variable, "OPENAI_API_KEY=") && !strings.HasPrefix(variable, "SYSTEMONE_API_KEY=") && !strings.HasPrefix(variable, "PROMPT2JSON_TEST_CLI=") {
			cmd.Env = append(cmd.Env, variable)
		}
	}
	cmd.Env = append(cmd.Env, "PROMPT2JSON_TEST_CLI=1")
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	return stdout.String(), stderr.String(), code
}

func TestDecisionCLINative(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request["model"] != "decision-model" || request["state"].(map[string]any)["message"] != "Reachable" {
			t.Errorf("unexpected request: %#v", request)
		}
		questions := request["questions"].(map[string]any)
		if len(questions) != 3 || questions["flag"].(map[string]any)["type"] != "noul" || request["messages"] != nil {
			t.Errorf("unexpected native format: %#v", request)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing bearer auth")
		}
		fmt.Fprint(w, `{"model":"resolved-decision-model","answers":{"flag":{"type":"noul","noul":0.99},"route":{"type":"choice","choice":"c","probabilities":{"a":0.1,"b":0.3,"c":0.6},"confidence":0.4},"severity":{"type":"score","score":1.1,"confidence":0.55,"probabilities":{"0":0.1,"1":0.7,"2":0.2},"legend":{"0":"Cosmetic","1":"Workaround","2":"Blocked"}}},"usage":{"input_tokens":150,"output_tokens":0,"extra":"preserved"}}`)
	}))
	defer server.Close()
	file := filepath.Join(t.TempDir(), "questions.json")
	if err := os.WriteFile(file, []byte(testQuestions), 0600); err != nil {
		t.Fatal(err)
	}
	out, errText, code := runDecisionCLI(t, `{"message":"Reachable"}`, "--mode", "decisions", "--provider", "systemone", "--model", "decision-model", "--url", server.URL, "--questions-file", file, "--state-format", "json", "--api-key", "test-key")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errText)
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(out), &response); err != nil {
		t.Fatal(err)
	}
	if response["model"] != "resolved-decision-model" || response["usage"].(map[string]any)["extra"] != "preserved" {
		t.Fatalf("metadata lost: %s", out)
	}
	answers := response["answers"].(map[string]any)
	if len(answers) != 3 || answers["route"].(map[string]any)["choice"] != "c" || math.Abs(answers["severity"].(map[string]any)["score"].(float64)-1.1) > 1e-12 {
		t.Fatalf("native decisions changed: %s", out)
	}
}

func TestDecisionCLISimulation(t *testing.T) {
	for _, provider := range []string{"openai", "gemini"} {
		t.Run(provider, func(t *testing.T) {
			content := `{"answers":{"q0":{"noul":0.9},"q1":{"probabilities":{"a":0.1,"b":0.3,"c":0.6}},"q2":{"probabilities":{"0":0.1,"1":0.7,"2":0.2}}}}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request map[string]any
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				var instruction, evidence string
				if provider == "openai" {
					messages := request["messages"].([]any)
					instruction = messages[0].(map[string]any)["content"].(string)
					evidence = messages[1].(map[string]any)["content"].(string)
					schema := request["response_format"].(map[string]any)["json_schema"].(map[string]any)
					if schema["strict"] != true {
						t.Error("simulation must enable strict schema")
					}
					json.NewEncoder(w).Encode(map[string]any{"model": "resolved-generative-model", "choices": []any{map[string]any{"message": map[string]any{"content": content}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": 55, "completion_tokens": 117, "total_tokens": 172}})
				} else {
					instruction = request["systemInstruction"].(map[string]any)["parts"].([]any)[0].(map[string]any)["text"].(string)
					evidence = request["contents"].([]any)[0].(map[string]any)["parts"].([]any)[0].(map[string]any)["text"].(string)
					if request["generationConfig"].(map[string]any)["responseMimeType"] != "application/json" {
						t.Error("missing constrained output")
					}
					json.NewEncoder(w).Encode(map[string]any{"modelVersion": "resolved-gemini", "candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": content}}}, "finishReason": "STOP"}}, "usageMetadata": map[string]any{"promptTokenCount": 55, "candidatesTokenCount": 14, "thoughtsTokenCount": 3, "totalTokenCount": 72}})
				}
				if !strings.Contains(instruction, `"q0"`) || strings.Contains(instruction, `"severity"`) || !strings.Contains(evidence, `"state":"The service is reachable."`) {
					t.Errorf("questions and evidence mixed: %s / %s", instruction, evidence)
				}
			}))
			defer server.Close()
			out, errText, code := runDecisionCLI(t, "The service is reachable.", "--mode", "decisions", "--provider", provider, "--model", "requested-alias", "--url", server.URL, "--questions", testQuestions, "--batch-questions", "--api-key", "test-key")
			if code != 0 {
				t.Fatalf("exit %d: %s", code, errText)
			}
			var response map[string]any
			if err := json.Unmarshal([]byte(out), &response); err != nil {
				t.Fatal(err)
			}
			answers := response["answers"].(map[string]any)
			if len(answers) != 3 || answers["flag"].(map[string]any)["noul"] != 0.9 || answers["route"].(map[string]any)["choice"] != "c" || math.Abs(answers["route"].(map[string]any)["confidence"].(float64)-0.4) > 1e-12 {
				t.Fatalf("wrong normalized answers: %s", out)
			}
			usage := response["usage"].(map[string]any)
			expectedOutput := float64(117)
			expectedModel := "resolved-generative-model"
			if provider == "gemini" {
				expectedOutput = 17
				expectedModel = "resolved-gemini"
			}
			if response["model"] != expectedModel || usage["input_tokens"] != float64(55) || usage["output_tokens"] != expectedOutput {
				t.Fatalf("fabricated metadata: %s", out)
			}
		})
	}
}

func TestDecisionCLIFlagErrors(t *testing.T) {
	base := []string{"--mode", "decisions", "--provider", "systemone", "--model", "decision-model", "--questions", `{"flag":{"type":"noul"}}`, "--prompt", "state"}
	for _, test := range []struct {
		args     []string
		code     int
		contains string
	}{
		{[]string{"--schema", `{}`}, 2, "instead of --schema"},
		{[]string{"--system-instruction", "override"}, 2, "instead of --schema"},
		{[]string{"--questions-file", "file.json"}, 2, "exactly one"},
		{[]string{"--strict-schema"}, 2, "not supported"},
		{[]string{"--batch-questions"}, 2, "only supported"},
		{[]string{"--schema-profile", "MINIMAL_202602"}, 2, "not supported"},
		{[]string{"--attach", "missing.pdf"}, 2, "not supported"},
		{[]string{"--project", "project"}, 2, "not valid"},
		{[]string{"--state-format", "yaml"}, 2, "text' or 'json"},
		{[]string{"--state-format", "json"}, 3, "invalid JSON state"},
		{[]string{"--questions", `{}`}, 3, "nonempty object"},
		{[]string{"--mode", "json"}, 2, "require --mode decisions"},
		{[]string{"--mode", "other"}, 2, "--mode must"},
		{[]string{"unexpected"}, 2, "positional arguments"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			out, errText, code := runDecisionCLI(t, "", append(append([]string{}, base...), test.args...)...)
			if code != test.code || out != "" || !strings.Contains(errText, test.contains) {
				t.Fatalf("exit %d stdout=%q stderr=%q", code, out, errText)
			}
		})
	}
	for _, state := range []string{"null", "true", "7", `{"x":1,"x":2}`} {
		out, errText, code := runDecisionCLI(t, "", append(append([]string{}, base...), "--state-format", "json", "--prompt", state)...)
		if code != 3 || out != "" {
			t.Fatalf("accepted state %s: %d %q %s", state, code, out, errText)
		}
	}
	for _, args := range [][]string{
		{"--provider", "systemone", "--model", "decision-model"},
		{"--mode", "decisions", "--provider", "openai", "--model", "generative-model", "--prompt", "state"},
		{"--mode", "decisions", "--provider", "openai", "--model", "generative-model", "--questions-file", "missing.json", "--prompt", "state"},
	} {
		out, _, code := runDecisionCLI(t, "", args...)
		if code == 0 || out != "" {
			t.Fatalf("accepted invalid flags: %v", args)
		}
	}
}

func TestDecisionCLIIndependentQuestions(t *testing.T) {
	for _, provider := range []string{"openai", "gemini"} {
		t.Run(provider, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var request map[string]any
				json.NewDecoder(r.Body).Decode(&request)
				var schema map[string]any
				var instruction, evidence string
				if provider == "openai" {
					schema = request["response_format"].(map[string]any)["json_schema"].(map[string]any)["schema"].(map[string]any)
					messages := request["messages"].([]any)
					instruction = messages[0].(map[string]any)["content"].(string)
					evidence = messages[1].(map[string]any)["content"].(string)
				} else {
					schema = request["generationConfig"].(map[string]any)["responseJsonSchema"].(map[string]any)
					instruction = request["systemInstruction"].(map[string]any)["parts"].([]any)[0].(map[string]any)["text"].(string)
					evidence = request["contents"].([]any)[0].(map[string]any)["parts"].([]any)[0].(map[string]any)["text"].(string)
				}
				answerSchemas := schema["properties"].(map[string]any)["answers"].(map[string]any)["properties"].(map[string]any)
				if len(answerSchemas) != 1 || answerSchemas["q0"] == nil || strings.Contains(instruction, `"q1"`) || evidence != `{"state":{"message":"Reachable"}}` {
					t.Errorf("request must contain only its question and shared state: %s", instruction)
				}
				var content string
				switch calls {
				case 1:
					content = `{"answers":{"q0":{"noul":1}}}`
				case 2:
					content = `{"answers":{"q0":{"probabilities":{"a":0,"b":0,"c":1}}}}`
				case 3:
					content = `{"answers":{"q0":{"probabilities":{"0":0,"1":0,"2":1}}}}`
				default:
					t.Error("unexpected extra call")
				}
				if provider == "openai" {
					json.NewEncoder(w).Encode(map[string]any{"model": "resolved-model", "choices": []any{map[string]any{"message": map[string]any{"content": content}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5}})
				} else {
					json.NewEncoder(w).Encode(map[string]any{"modelVersion": "resolved-model", "candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": content}}}, "finishReason": "STOP"}}, "usageMetadata": map[string]any{"promptTokenCount": 10, "candidatesTokenCount": 4, "thoughtsTokenCount": 1}})
				}
			}))
			defer server.Close()
			out, errText, code := runDecisionCLI(t, `{"message":"Reachable"}`, "--mode", "decisions", "--provider", provider, "--model", "requested-model", "--url", server.URL, "--questions", testQuestions, "--state-format", "json")
			if code != 0 || calls != 3 {
				t.Fatalf("calls=%d exit=%d stderr=%s", calls, code, errText)
			}
			var response map[string]any
			json.Unmarshal([]byte(out), &response)
			answers := response["answers"].(map[string]any)
			usage := response["usage"].(map[string]any)
			if len(answers) != 3 || answers["route"].(map[string]any)["choice"] != "c" || answers["severity"].(map[string]any)["score"] != float64(2) || usage["input_tokens"] != float64(30) || usage["output_tokens"] != float64(15) {
				t.Fatalf("wrong merged response: %s", out)
			}
			for _, want := range []string{`"noul":1.0`, `"score":2.0`, `"confidence":1.0`, `"a":0.0`, `"c":1.0`, `"input_tokens":30`} {
				if !strings.Contains(out, want) {
					t.Errorf("missing %s in %s", want, out)
				}
			}
		})
	}
}

func TestDecisionCLIIndependentDryRun(t *testing.T) {
	out, errText, code := runDecisionCLI(t, "evidence", "--mode", "decisions", "--provider", "openai", "--model", "example-model", "--questions", testQuestions, "--show-request-body")
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errText)
	}
	var requests []struct {
		QuestionID string         `json:"question_id"`
		Request    map[string]any `json:"request"`
	}
	if err := json.Unmarshal([]byte(out), &requests); err != nil || len(requests) != 3 {
		t.Fatalf("wrong dry-run requests: %s (%v)", out, err)
	}
	for i, id := range []string{"flag", "route", "severity"} {
		if requests[i].QuestionID != id || requests[i].Request["messages"] == nil {
			t.Fatalf("missing request for %s", id)
		}
	}
}

func TestDecisionCLIIndependentFailureIsAtomic(t *testing.T) {
	for _, failure := range []string{"validation", "api", "model"} {
		t.Run(failure, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls == 2 && failure == "api" {
					http.Error(w, "unavailable", 503)
					return
				}
				content := `{"answers":{"q0":{"noul":1}}}`
				if calls == 2 {
					content = `{"answers":{"q0":{"probabilities":{"a":0,"b":0,"c":0}}}}`
				}
				model := "resolved-model"
				if calls == 2 && failure == "model" {
					model = "different-model"
					content = `{"answers":{"q0":{"probabilities":{"a":0,"b":0,"c":1}}}}`
				}
				json.NewEncoder(w).Encode(map[string]any{"model": model, "choices": []any{map[string]any{"message": map[string]any{"content": content}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5}})
			}))
			defer server.Close()
			file := filepath.Join(t.TempDir(), "output.json")
			os.WriteFile(file, []byte("existing"), 0600)
			out, errText, code := runDecisionCLI(t, "evidence", "--mode", "decisions", "--provider", "openai", "--model", "example-model", "--url", server.URL, "--questions", testQuestions, "--out", file)
			wantCode := 4
			if failure == "api" {
				wantCode = 5
			}
			data, _ := os.ReadFile(file)
			if code != wantCode || out != "" || calls != 2 || string(data) != "existing" {
				t.Fatalf("exit=%d calls=%d stdout=%q file=%q stderr=%s", code, calls, out, data, errText)
			}
		})
	}
}

func TestDecisionCLIDryRun(t *testing.T) {
	questions := `{"flag":{"type":"noul","instructions":"Reachable?"}}`
	base := []string{"--mode", "decisions", "--provider", "systemone", "--model", "decision-model", "--questions", questions, "--prompt", "state"}
	out, errText, code := runDecisionCLI(t, "", append(append([]string{}, base...), "--show-url")...)
	if code != 0 || strings.TrimSpace(out) != defaultSystemOneURL {
		t.Fatalf("dry run: %d %s %s", code, out, errText)
	}
	out, errText, code = runDecisionCLI(t, "", append(append([]string{}, base...), "--show-request-body", "--pretty-print")...)
	if code != 0 || !strings.Contains(out, `"state": "state"`) || strings.Contains(out, "response_format") {
		t.Fatalf("dry run body: %d %s %s", code, out, errText)
	}
	// Legacy invocations still require and send the caller's schema/instruction.
	out, errText, code = runDecisionCLI(t, "", "--provider", "openai", "--model", "legacy", "--schema", `{"type":"object","properties":{"label":{"type":"string"}},"required":["label"],"additionalProperties":false}`, "--system-instruction", "legacy instruction", "--prompt", "legacy prompt", "--show-request-body")
	if code != 0 || !strings.Contains(out, "legacy instruction") || strings.Contains(out, "Questions:") {
		t.Fatalf("legacy behavior changed: %d %s %s", code, out, errText)
	}
}

func TestDecisionCLIFailureDoesNotWriteOutput(t *testing.T) {
	for _, test := range []struct {
		provider, body string
		status, code   int
	}{
		{"openai", `{"model":"generative-model","choices":[{"message":{"content":"{\"answers\":{\"q0\":{\"probabilities\":{\"a\":0.2,\"b\":0.2}}}}"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":10}}`, 200, 4},
		{"openai", `{"model":"generative-model","choices":[{"message":{"content":"{}"},"finish_reason":"length"}],"usage":{"prompt_tokens":10,"completion_tokens":10}}`, 200, 4},
		{"openai", `{"model":"generative-model","choices":[{"message":{"content":"{}"},"finish_reason":"stop"}]}`, 200, 4},
		{"systemone", `{"model":"decision-model","answers":{},"usage":{"input_tokens":1,"output_tokens":0}}`, 200, 4},
		{"systemone", `{"model":"decision-model","answers":{"q":{"type":"refusal"}},"usage":{"input_tokens":1,"output_tokens":0}}`, 200, 4},
		{"systemone", `{"error":"bad request"}`, 422, 5},
	} {
		t.Run(fmt.Sprintf("%s-%s", test.provider, test.body), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(test.status); fmt.Fprint(w, test.body) }))
			defer server.Close()
			file := filepath.Join(t.TempDir(), "existing.json")
			if err := os.WriteFile(file, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			out, errText, code := runDecisionCLI(t, "state", "--mode", "decisions", "--provider", test.provider, "--model", "model", "--url", server.URL, "--questions", `{"q":{"type":"choice","criteria":{"a":null,"b":null}}}`, "--out", file)
			if code != test.code || out != "" {
				t.Fatalf("exit %d stdout=%q stderr=%s", code, out, errText)
			}
			data, err := os.ReadFile(file)
			if err != nil || string(data) != "original" {
				t.Fatalf("failed response overwrote output: %s %v", data, err)
			}
		})
	}
}
