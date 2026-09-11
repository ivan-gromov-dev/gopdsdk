package analyzer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestLSPFramingAndInitializeSession(t *testing.T) {
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	rootURI := pathFileURI(root)
	input := bytes.Join([][]byte{
		frameLSP(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"rootUri": rootURI, "initializationOptions": map[string]any{"target": "device"}}}),
		frameLSP(t, map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}}),
		frameLSP(t, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "shutdown", "params": nil}),
		frameLSP(t, map[string]any{"jsonrpc": "2.0", "method": "exit", "params": nil}),
	}, nil)
	var output, logs bytes.Buffer
	if err := RunLSP(context.Background(), []string{"lsp"}, bytes.NewReader(input), &output, &logs, options); err != nil {
		t.Fatal(err)
	}
	messages := decodeLSPFrames(t, output.Bytes())
	if len(messages) != 2 {
		t.Fatalf("responses = %d, want 2: %s", len(messages), output.String())
	}
	capabilities := messages[0]["result"].(map[string]any)["capabilities"].(map[string]any)
	if capabilities["diagnosticProvider"] == nil || capabilities["codeActionProvider"] == nil || capabilities["completionProvider"] != nil {
		t.Fatalf("unexpected capabilities: %#v", capabilities)
	}
	if strings.Contains(logs.String(), root) || !strings.Contains(logs.String(), "event=initialized") {
		t.Fatalf("logs leak a path or miss event: %q", logs.String())
	}
}

func TestLSPPublishPullClearingAndCodeActions(t *testing.T) {
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	uri := pathFileURI(filepath.Join(root, "game.go"))
	rootURI := pathFileURI(root)
	var output bytes.Buffer
	server := &lspServer{ctx: context.Background(), options: options, engine: NewIncrementalEngine(options.Catalog, options.Registry, IncrementalOptions{}), out: &output, logs: io.Discard, docs: map[string]lspDocument{}, roots: map[string]*lspWorkspace{}, cancel: map[string]context.CancelFunc{}}
	server.roots[rootURI] = &lspWorkspace{URI: rootURI, Root: root, Version: 1, Target: "device", Diagnostics: map[string][]map[string]any{}, Report: map[string][]Diagnostic{}}
	rule, ok := options.Catalog.Rule("device-goroutine")
	if !ok {
		t.Fatal("missing fixture rule")
	}
	diagnostic := Diagnostic{Rule: rule.ID, Category: rule.Family, Severity: rule.Default, Confidence: rule.Confidence, Target: TargetDevice, Message: "avoid goroutine", Primary: SourceRange{Path: "game.go", Start: Point{Line: 2, Column: 2}, End: Point{Line: 2, Column: 4}}, Documentation: documentationFor(rule, ""), Related: []Related{}, Edits: []EditGroup{{Message: "replace safely", Edits: []Edit{{Range: SourceRange{Path: "game.go", Start: Point{Line: 2, Column: 2}, End: Point{Line: 2, Column: 4}}, NewText: "ok"}}}}}
	server.publish(rootURI, 1, []Diagnostic{diagnostic}, map[string][]byte{filepath.Join(root, "game.go"): []byte("package game\n\tgo f()\n")})
	server.pullDiagnostics(json.RawMessage(`7`), mustJSON(t, map[string]any{"textDocument": map[string]any{"uri": uri}}))
	server.codeActions(json.RawMessage(`8`), mustJSON(t, map[string]any{"textDocument": map[string]any{"uri": uri}}))
	server.mu.Lock()
	server.roots[rootURI].Version = 2
	server.mu.Unlock()
	server.publish(rootURI, 2, nil, nil)
	messages := decodeLSPFrames(t, output.Bytes())
	var sawPull, sawAction, sawClear bool
	for _, message := range messages {
		if message["id"] == float64(7) {
			items := message["result"].(map[string]any)["items"].([]any)
			sawPull = len(items) == 1
		}
		if message["id"] == float64(8) {
			actions := message["result"].([]any)
			if len(actions) == 1 {
				diagnostics := actions[0].(map[string]any)["diagnostics"].([]any)
				item := diagnostics[0].(map[string]any)
				sawAction = item["range"] != nil && item["message"] == "avoid goroutine" && item["source"] == lspSource
			}
		}
		if message["method"] == "textDocument/publishDiagnostics" {
			params := message["params"].(map[string]any)
			if params["uri"] == uri && len(params["diagnostics"].([]any)) == 0 {
				sawClear = true
			}
		}
	}
	if !sawPull || !sawAction || !sawClear {
		t.Fatalf("pull=%v action=%v clear=%v messages=%#v", sawPull, sawAction, sawClear, messages)
	}
}

func TestLSPMultiRootSelectionAndMalformedInput(t *testing.T) {
	outer := t.TempDir()
	inner := filepath.Join(outer, "nested")
	server := &lspServer{roots: map[string]*lspWorkspace{}}
	outerURI, innerURI := pathFileURI(outer), pathFileURI(inner)
	server.roots[outerURI] = &lspWorkspace{URI: outerURI}
	server.roots[innerURI] = &lspWorkspace{URI: innerURI}
	if got := server.workspaceForURI(pathFileURI(filepath.Join(inner, "game.go"))); got == nil || got.URI != innerURI {
		t.Fatalf("selected %#v, want inner root", got)
	}
	_, err := readLSPMessage(bufio.NewReader(strings.NewReader("Content-Length: nope\r\n\r\n")))
	if err == nil {
		t.Fatal("malformed length accepted")
	}
	_, err = readLSPMessage(bufio.NewReader(strings.NewReader("X-Test: 1\r\n\r\n{}")))
	if err == nil {
		t.Fatal("missing length accepted")
	}
	point := lspPoint(Point{Line: 1, Column: 6}, []byte("a😀b"))
	if point["character"] != 3 {
		t.Fatalf("UTF-16 character = %d, want 3", point["character"])
	}
	documentURI := pathFileURI(filepath.Join(outer, "stale.go"))
	server.docs = map[string]lspDocument{}
	server.roots = map[string]*lspWorkspace{}
	server.changeDocument("textDocument/didOpen", mustJSON(t, map[string]any{"textDocument": map[string]any{"uri": documentURI, "version": 4, "text": "new"}}))
	server.changeDocument("textDocument/didChange", mustJSON(t, map[string]any{"textDocument": map[string]any{"uri": documentURI, "version": 3}, "contentChanges": []map[string]any{{"text": "stale"}}}))
	if got := string(server.docs[documentURI].Text); got != "new" {
		t.Fatalf("stale edit replaced document with %q", got)
	}
}

func TestLSPTargetsAlwaysIncludeSharedAnalysis(t *testing.T) {
	tests := []struct {
		name string
		want []Target
	}{
		{name: "simulator", want: []Target{TargetShared, TargetSimulator}},
		{name: "device", want: []Target{TargetShared, TargetDevice}},
		{name: "both", want: []Target{TargetShared, TargetSimulator, TargetDevice}},
	}
	for _, test := range tests {
		got, err := lspTargets(test.name)
		if err != nil || !slices.Equal(got, test.want) {
			t.Errorf("lspTargets(%q) = %v, %v; want %v, nil", test.name, got, err, test.want)
		}
	}
	if _, err := lspTargets("shared"); err == nil {
		t.Fatal("shared accepted as a configurable LSP target")
	}
}

func frameLSP(t *testing.T, value any) []byte {
	t.Helper()
	data := mustJSON(t, value)
	return []byte("Content-Length: " + strconv.Itoa(len(data)) + "\r\n\r\n" + string(data))
}
func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func decodeLSPFrames(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	reader := bufio.NewReader(bytes.NewReader(data))
	var result []map[string]any
	for {
		message, err := readRawLSP(reader)
		if err == io.EOF {
			return result
		}
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if json.Unmarshal(message, &value) != nil {
			t.Fatal("invalid output JSON")
		}
		result = append(result, value)
	}
}
func readRawLSP(reader *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if name, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(name, "Content-Length") {
			length, _ = strconv.Atoi(strings.TrimSpace(value))
		}
	}
	if length < 0 {
		return nil, io.ErrUnexpectedEOF
	}
	data := make([]byte, length)
	_, err := io.ReadFull(reader, data)
	return data, err
}
