package analyzer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf16"
)

const lspSource = "gopdsdk"

type lspMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *lspError       `json:"error,omitempty"`
}

type lspError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type lspDocument struct {
	URI     string
	Path    string
	Text    []byte
	Version int
}

type lspWorkspace struct {
	URI         string
	Root        string
	Version     uint64
	Target      string
	Rules       []RuleID
	Categories  []RuleFamily
	Excluded    []RuleID
	Deep        bool
	Diagnostics map[string][]map[string]any
	Report      map[string][]Diagnostic
	Cancel      context.CancelFunc
}

type lspServer struct {
	ctx     context.Context
	options CheckOptions
	engine  *IncrementalEngine
	out     io.Writer
	logs    io.Writer

	writeMu sync.Mutex
	mu      sync.Mutex
	docs    map[string]lspDocument
	roots   map[string]*lspWorkspace
	cancel  map[string]context.CancelFunc
	closed  bool
}

// RunLSP serves the analyzer's editor API over standard LSP JSON-RPC framing.
// It deliberately advertises no general Go language features so it can run
// beside gopls without competing for completion, navigation, or formatting.
func RunLSP(ctx context.Context, args []string, in io.Reader, out, logs io.Writer, options CheckOptions) error {
	if len(args) != 1 || args[0] != "lsp" {
		return commandError(ExitConfiguration, errors.New("usage: gopdsdk lsp"))
	}
	if options.AnalyzerVersion == "" || options.SDKVersion == "" {
		return commandError(ExitInternal, errors.New("language server analyzer composition has no version"))
	}
	server := &lspServer{ctx: ctx, options: options, engine: NewIncrementalEngine(options.Catalog, options.Registry, IncrementalOptions{}), out: out, logs: logs, docs: make(map[string]lspDocument), roots: make(map[string]*lspWorkspace), cancel: make(map[string]context.CancelFunc)}
	reader := bufio.NewReader(in)
	var workers sync.WaitGroup
	for {
		message, err := readLSPMessage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(ctx.Err(), context.Canceled) {
				workers.Wait()
				return nil
			}
			return commandError(ExitConfiguration, fmt.Errorf("read LSP message: %w", err))
		}
		if message.Method == "$/cancelRequest" {
			server.cancelRequest(message.Params)
			continue
		}
		if len(message.ID) == 0 {
			if message.Method == "exit" {
				workers.Wait()
				return nil
			}
			server.notify(message)
			continue
		}
		if message.Method == "initialize" || message.Method == "shutdown" {
			server.request(message)
			continue
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			server.request(message)
		}()
	}
}

func readLSPMessage(reader *bufio.Reader) (lspMessage, error) {
	length := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return lspMessage{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return lspMessage{}, fmt.Errorf("malformed header")
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil || length < 0 || length > 16<<20 {
				return lspMessage{}, fmt.Errorf("invalid Content-Length")
			}
		}
	}
	if length < 0 {
		return lspMessage{}, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(reader, body); err != nil {
		return lspMessage{}, err
	}
	var message lspMessage
	if err := json.Unmarshal(body, &message); err != nil {
		return lspMessage{}, fmt.Errorf("invalid JSON: %w", err)
	}
	if message.JSONRPC != "2.0" || message.Method == "" {
		return lspMessage{}, fmt.Errorf("invalid JSON-RPC message")
	}
	return message, nil
}

func (server *lspServer) send(message lspMessage) error {
	message.JSONRPC = "2.0"
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	server.writeMu.Lock()
	defer server.writeMu.Unlock()
	_, err = fmt.Fprintf(server.out, "Content-Length: %d\r\n\r\n%s", len(data), data)
	return err
}

func (server *lspServer) respond(id json.RawMessage, result any, err *lspError) {
	if writeErr := server.send(lspMessage{ID: id, Result: result, Error: err}); writeErr != nil {
		server.log("response-write", writeErr)
	}
}

func (server *lspServer) log(event string, err error) {
	// Logs intentionally contain only event names and error classes: source,
	// paths, request payloads, and environment values never cross this boundary.
	if err == nil {
		fmt.Fprintf(server.logs, "gopdsdk-lsp event=%s\n", event)
	} else {
		fmt.Fprintf(server.logs, "gopdsdk-lsp event=%s error=%T\n", event, err)
	}
}

func (server *lspServer) request(message lspMessage) {
	key := string(message.ID)
	ctx, cancel := context.WithCancel(server.ctx)
	server.mu.Lock()
	server.cancel[key] = cancel
	server.mu.Unlock()
	defer func() {
		cancel()
		server.mu.Lock()
		delete(server.cancel, key)
		server.mu.Unlock()
	}()
	switch message.Method {
	case "initialize":
		server.initialize(message.ID, message.Params)
	case "shutdown":
		server.mu.Lock()
		server.closed = true
		for _, workspace := range server.roots {
			if workspace.Cancel != nil {
				workspace.Cancel()
			}
		}
		server.mu.Unlock()
		server.respond(message.ID, nil, nil)
	case "textDocument/diagnostic":
		server.pullDiagnostics(message.ID, message.Params)
	case "textDocument/codeAction":
		server.codeActions(message.ID, message.Params)
	case "gopdsdk/ruleHelp":
		server.ruleHelp(message.ID, message.Params)
	default:
		server.respond(message.ID, nil, &lspError{Code: -32601, Message: "method not supported"})
	}
	_ = ctx
}

func (server *lspServer) cancelRequest(params json.RawMessage) {
	var value struct {
		ID json.RawMessage `json:"id"`
	}
	if json.Unmarshal(params, &value) != nil {
		return
	}
	server.mu.Lock()
	cancel := server.cancel[string(value.ID)]
	server.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (server *lspServer) initialize(id json.RawMessage, params json.RawMessage) {
	var value struct {
		RootURI          string `json:"rootUri"`
		WorkspaceFolders []struct {
			URI string `json:"uri"`
		} `json:"workspaceFolders"`
		InitializationOptions json.RawMessage `json:"initializationOptions"`
	}
	if err := json.Unmarshal(params, &value); err != nil {
		server.respond(id, nil, &lspError{Code: -32602, Message: "invalid initialize parameters"})
		return
	}
	if len(value.WorkspaceFolders) == 0 && value.RootURI != "" {
		value.WorkspaceFolders = append(value.WorkspaceFolders, struct {
			URI string `json:"uri"`
		}{value.RootURI})
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	for _, folder := range value.WorkspaceFolders {
		root, err := fileURIPath(folder.URI)
		if err != nil {
			continue
		}
		workspace := &lspWorkspace{URI: folder.URI, Root: root, Target: "both", Diagnostics: make(map[string][]map[string]any), Report: make(map[string][]Diagnostic)}
		applyLSPSettings(workspace, value.InitializationOptions)
		server.roots[folder.URI] = workspace
	}
	server.respond(id, map[string]any{"capabilities": map[string]any{
		"textDocumentSync":   map[string]any{"openClose": true, "change": 1, "save": map[string]any{"includeText": true}},
		"diagnosticProvider": map[string]any{"identifier": lspSource, "interFileDependencies": true, "workspaceDiagnostics": false},
		"codeActionProvider": map[string]any{"codeActionKinds": []string{"quickfix"}},
	}, "serverInfo": map[string]any{"name": "gopdsdk analyzer", "version": server.options.AnalyzerVersion}}, nil)
}

func (server *lspServer) notify(message lspMessage) {
	switch message.Method {
	case "initialized":
		server.log("initialized", nil)
	case "workspace/didChangeConfiguration":
		var value struct {
			Settings json.RawMessage `json:"settings"`
		}
		if json.Unmarshal(message.Params, &value) != nil {
			return
		}
		server.mu.Lock()
		for _, workspace := range server.roots {
			applyLSPSettings(workspace, value.Settings)
		}
		server.mu.Unlock()
		server.refresh()
	case "workspace/didChangeWorkspaceFolders":
		server.changeRoots(message.Params)
	case "textDocument/didOpen", "textDocument/didChange", "textDocument/didSave", "textDocument/didClose":
		server.changeDocument(message.Method, message.Params)
	}
}

func applyLSPSettings(workspace *lspWorkspace, raw json.RawMessage) {
	var settings struct {
		Target       string       `json:"target"`
		Rules        []RuleID     `json:"rules"`
		Categories   []RuleFamily `json:"categories"`
		ExcludeRules []RuleID     `json:"excludeRules"`
		Deep         bool         `json:"deep"`
	}
	if json.Unmarshal(raw, &settings) != nil {
		return
	}
	if settings.Target == "shared" || settings.Target == "simulator" || settings.Target == "device" || settings.Target == "both" {
		workspace.Target = settings.Target
	}
	workspace.Rules, workspace.Categories, workspace.Excluded, workspace.Deep = settings.Rules, settings.Categories, settings.ExcludeRules, settings.Deep
}

func (server *lspServer) changeRoots(params json.RawMessage) {
	var value struct {
		Event struct {
			Added []struct {
				URI string `json:"uri"`
			} `json:"added"`
			Removed []struct {
				URI string `json:"uri"`
			} `json:"removed"`
		} `json:"event"`
	}
	if json.Unmarshal(params, &value) != nil {
		return
	}
	server.mu.Lock()
	defer server.mu.Unlock()
	for _, item := range value.Event.Removed {
		delete(server.roots, item.URI)
	}
	for _, item := range value.Event.Added {
		if root, err := fileURIPath(item.URI); err == nil {
			server.roots[item.URI] = &lspWorkspace{URI: item.URI, Root: root, Target: "both", Diagnostics: make(map[string][]map[string]any), Report: make(map[string][]Diagnostic)}
		}
	}
}

func (server *lspServer) changeDocument(method string, params json.RawMessage) {
	var value struct {
		TextDocument struct {
			URI     string `json:"uri"`
			Version int    `json:"version"`
			Text    string `json:"text"`
		} `json:"textDocument"`
		ContentChanges []struct {
			Text string `json:"text"`
		} `json:"contentChanges"`
		Text string `json:"text"`
	}
	if json.Unmarshal(params, &value) != nil || value.TextDocument.URI == "" {
		return
	}
	server.mu.Lock()
	if method == "textDocument/didClose" {
		delete(server.docs, value.TextDocument.URI)
	} else {
		prior, exists := server.docs[value.TextDocument.URI]
		if exists && method == "textDocument/didChange" && value.TextDocument.Version > 0 && value.TextDocument.Version <= prior.Version {
			server.mu.Unlock()
			return
		}
		text := value.TextDocument.Text
		if len(value.ContentChanges) != 0 {
			text = value.ContentChanges[len(value.ContentChanges)-1].Text
		}
		if method == "textDocument/didSave" && value.Text != "" {
			text = value.Text
		}
		if method == "textDocument/didSave" && text == "" {
			text = string(prior.Text)
		}
		path, err := fileURIPath(value.TextDocument.URI)
		if err == nil {
			server.docs[value.TextDocument.URI] = lspDocument{URI: value.TextDocument.URI, Path: path, Text: []byte(text), Version: value.TextDocument.Version}
		}
	}
	workspace := server.workspaceForURI(value.TextDocument.URI)
	server.mu.Unlock()
	if workspace != nil {
		go server.analyze(workspace.URI, method == "textDocument/didSave")
	}
}

func (server *lspServer) analyze(workspaceURI string, save bool) {
	server.mu.Lock()
	workspace := server.roots[workspaceURI]
	if workspace == nil {
		server.mu.Unlock()
		return
	}
	workspace.Version++
	version := workspace.Version
	if workspace.Cancel != nil {
		workspace.Cancel()
	}
	analysisContext, cancel := context.WithCancel(server.ctx)
	workspace.Cancel = cancel
	overlay := make(map[string][]byte)
	for _, document := range server.docs {
		if pathWithinRoot(workspace.Root, document.Path) {
			overlay[document.Path] = append([]byte(nil), document.Text...)
		}
	}
	selection := RuleSelection{IDs: append([]RuleID(nil), workspace.Rules...), Families: append([]RuleFamily(nil), workspace.Categories...), ExcludedIDs: append([]RuleID(nil), workspace.Excluded...), IncludeExperimental: workspace.Deep}
	targetName, deep := workspace.Target, workspace.Deep
	server.mu.Unlock()
	defer cancel()
	targets, err := checkTargets(targetName)
	if err != nil {
		server.log("configuration", err)
		return
	}
	all := make([]Finding, 0)
	for _, target := range targets {
		schedule := ScheduleFastEdit
		if save {
			schedule = ScheduleSave
		}
		if deep {
			schedule = ScheduleDeep
		}
		result, analyzeErr := server.engine.Analyze(analysisContext, IncrementalRequest{LoadConfig: LoadConfig{ModuleRoot: workspace.Root, Patterns: []string{"./..."}, Target: target, Overlay: overlay}, Version: version, Schedule: schedule, Selection: selection, Generated: GeneratedExclude, Progress: func(progress Progress) { server.progress(workspaceURI, version, progress) }})
		if analyzeErr != nil {
			if !errors.Is(analyzeErr, ErrStaleAnalysis) && !errors.Is(analyzeErr, context.Canceled) {
				server.log("analysis", analyzeErr)
			}
			return
		}
		all = append(all, result.Findings...)
	}
	sortFindings(all)
	report, err := NewReport(server.options.Catalog, server.options.AnalyzerVersion, server.options.SDKVersion, all)
	if err != nil {
		server.log("report", err)
		return
	}
	server.publish(workspaceURI, version, report.Diagnostics, overlay)
}

func (server *lspServer) publish(workspaceURI string, version uint64, diagnostics []Diagnostic, overlay map[string][]byte) {
	server.mu.Lock()
	workspace := server.roots[workspaceURI]
	if workspace == nil || version != workspace.Version {
		server.mu.Unlock()
		return
	}
	workspace.Cancel = nil
	next := make(map[string][]map[string]any)
	reports := make(map[string][]Diagnostic)
	for _, diagnostic := range diagnostics {
		path := filepath.Join(workspace.Root, filepath.FromSlash(diagnostic.Primary.Path))
		uri := pathFileURI(path)
		contents := overlay[path]
		if contents == nil {
			contents, _ = os.ReadFile(path)
		}
		next[uri] = append(next[uri], lspDiagnostic(diagnostic, contents, workspace.Root))
		reports[uri] = append(reports[uri], diagnostic)
	}
	old := workspace.Diagnostics
	workspace.Diagnostics, workspace.Report = next, reports
	server.mu.Unlock()
	keys := make(map[string]bool)
	for uri := range old {
		keys[uri] = true
	}
	for uri := range next {
		keys[uri] = true
	}
	ordered := make([]string, 0, len(keys))
	for uri := range keys {
		ordered = append(ordered, uri)
	}
	sort.Strings(ordered)
	for _, uri := range ordered {
		items := next[uri]
		if items == nil {
			items = []map[string]any{}
		}
		_ = server.send(lspMessage{Method: "textDocument/publishDiagnostics", Params: lspJSON(map[string]any{"uri": uri, "diagnostics": items})})
	}
	server.refresh()
}

func lspDiagnostic(d Diagnostic, contents []byte, root string) map[string]any {
	severity := 3
	if d.Severity == SeverityError {
		severity = 1
	} else if d.Severity == SeverityWarning {
		severity = 2
	} else if d.Severity == SeverityInformation {
		severity = 4
	}
	related := make([]map[string]any, 0, len(d.Related))
	for _, item := range d.Related {
		related = append(related, map[string]any{"location": map[string]any{"uri": pathFileURI(filepath.Join(root, filepath.FromSlash(item.Range.Path))), "range": lspRange(item.Range, nil)}, "message": item.Message})
	}
	return map[string]any{"range": lspRange(d.Primary, contents), "severity": severity, "code": d.Rule, "codeDescription": map[string]any{"href": d.Documentation}, "source": lspSource, "message": d.Message, "relatedInformation": related, "data": map[string]any{"rule": d.Rule, "target": d.Target}}
}

func lspRange(value SourceRange, contents []byte) map[string]any {
	return map[string]any{"start": lspPoint(value.Start, contents), "end": lspPoint(value.End, contents)}
}

func lspPoint(point Point, contents []byte) map[string]int {
	character := point.Column - 1
	if len(contents) != 0 && point.Line > 0 {
		lines := bytes.Split(contents, []byte{'\n'})
		if point.Line <= len(lines) {
			limit := point.Column - 1
			if limit > len(lines[point.Line-1]) {
				limit = len(lines[point.Line-1])
			}
			character = len(utf16.Encode([]rune(string(lines[point.Line-1][:limit]))))
		}
	}
	return map[string]int{"line": max(0, point.Line-1), "character": max(0, character)}
}

func (server *lspServer) pullDiagnostics(id json.RawMessage, params json.RawMessage) {
	var value struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if json.Unmarshal(params, &value) != nil {
		server.respond(id, nil, &lspError{Code: -32602, Message: "invalid diagnostic parameters"})
		return
	}
	server.mu.Lock()
	workspace := server.workspaceForURI(value.TextDocument.URI)
	var items []map[string]any
	if workspace != nil {
		items = append(items, workspace.Diagnostics[value.TextDocument.URI]...)
	}
	server.mu.Unlock()
	if items == nil {
		items = []map[string]any{}
	}
	server.respond(id, map[string]any{"kind": "full", "items": items}, nil)
}

func (server *lspServer) codeActions(id json.RawMessage, params json.RawMessage) {
	var value struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if json.Unmarshal(params, &value) != nil {
		server.respond(id, nil, &lspError{Code: -32602, Message: "invalid code action parameters"})
		return
	}
	server.mu.Lock()
	workspace := server.workspaceForURI(value.TextDocument.URI)
	var reports []Diagnostic
	if workspace != nil {
		reports = append(reports, workspace.Report[value.TextDocument.URI]...)
	}
	server.mu.Unlock()
	actions := make([]map[string]any, 0)
	for _, diagnostic := range reports {
		for _, group := range diagnostic.Edits {
			changes := make(map[string][]map[string]any)
			for _, edit := range group.Edits {
				uri := pathFileURI(filepath.Join(workspace.Root, filepath.FromSlash(edit.Range.Path)))
				changes[uri] = append(changes[uri], map[string]any{"range": lspRange(edit.Range, nil), "newText": edit.NewText})
			}
			actions = append(actions, map[string]any{"title": group.Message, "kind": "quickfix", "isPreferred": true, "diagnostics": []map[string]any{lspDiagnostic(diagnostic, nil, workspace.Root)}, "edit": map[string]any{"changes": changes}})
		}
	}
	server.respond(id, actions, nil)
}

func (server *lspServer) ruleHelp(id json.RawMessage, params json.RawMessage) {
	var value struct {
		Rule RuleID `json:"rule"`
	}
	if json.Unmarshal(params, &value) != nil {
		server.respond(id, nil, &lspError{Code: -32602, Message: "invalid rule help parameters"})
		return
	}
	rule, ok := server.options.Catalog.Rule(value.Rule)
	if !ok {
		server.respond(id, nil, &lspError{Code: -32602, Message: "unknown analyzer rule"})
		return
	}
	server.respond(id, map[string]any{"rule": rule, "documentation": documentationFor(rule, "")}, nil)
}

func (server *lspServer) progress(workspace string, version uint64, progress Progress) {
	_ = server.send(lspMessage{Method: "$/progress", Params: lspJSON(map[string]any{"token": fmt.Sprintf("gopdsdk:%s:%d", workspace, version), "value": map[string]any{"kind": "report", "message": progress.Stage, "percentage": progress.Current * 100 / progress.Total}})})
}

func (server *lspServer) refresh() {
	_ = server.send(lspMessage{Method: "workspace/diagnostic/refresh", Params: lspJSON(map[string]any{})})
}

func lspJSON(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func (server *lspServer) workspaceForURI(uri string) *lspWorkspace {
	var best *lspWorkspace
	for rootURI, workspace := range server.roots {
		if strings.HasPrefix(strings.ToLower(uri), strings.ToLower(strings.TrimRight(rootURI, "/")+"/")) || strings.EqualFold(uri, rootURI) {
			if best == nil || len(rootURI) > len(best.URI) {
				best = workspace
			}
		}
	}
	return best
}

func fileURIPath(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "file" {
		return "", fmt.Errorf("not a file URI")
	}
	path, err := url.PathUnescape(parsed.Path)
	if err != nil {
		return "", err
	}
	if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return filepath.Clean(filepath.FromSlash(path)), nil
}
func pathFileURI(path string) string {
	path = filepath.ToSlash(path)
	if len(path) >= 2 && path[1] == ':' {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}
