/*
* This file contains the LSP server implementation.
* Author: Zwane Mwaikambo
 */

package lspserver

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/TobiasYin/go-lsp/jsonrpc"
	"github.com/TobiasYin/go-lsp/logs"
	"github.com/TobiasYin/go-lsp/lsp"
	"github.com/TobiasYin/go-lsp/lsp/defines"
)

func (s *lspServer) Handle(ctx context.Context, conn *jsonrpc.Conn, req *jsonrpc.RequestMessage) {
	logs.Printf("Handling Method...\n")
	if req.Method == "window/showGeneratedCode" {
		fmt.Println("Received a request for someMethod")
	} else if req.Method == "analysisStarted" {
		fmt.Println("Analysis Started")
	} else if req.Method == "analysisDone" {
		fmt.Println("Analysis Done")
	}
}

// Keep the lsp protocol implementation separate from the rest of the application
type LspServer interface {
	Start(ctx context.Context) error
	OnInitialized(ctx context.Context, req *defines.InitializeParams) error
	OnDidOpenTextDocument(ctx context.Context, req *defines.DidOpenTextDocumentParams) error
	OnDidChangeTextDocument(ctx context.Context, req *defines.DidChangeTextDocumentParams) error
	OnDidSaveTextDocument(ctx context.Context, req *defines.DidSaveTextDocumentParams) error
	OnHover(ctx context.Context, req *defines.HoverParams) (result *defines.Hover, err error)
	OnDiagnostic(ctx context.Context, req *defines.DocumentDiagnosticParams) (*defines.FullDocumentDiagnosticReport, error)
	OnCompletion(ctx context.Context, req *defines.CompletionParams) (result *[]defines.CompletionItem, err error)
	OnCodeActionWithSliceCodeAction(ctx context.Context, req *defines.CodeActionParams) (*[]defines.CodeAction, error)
	OnCodeActionResolve(ctx context.Context, req *defines.CodeAction) (result *defines.CodeAction, err error)
}

type lspServer struct {
	name      string
	server    *lsp.Server
	backend   LspBackend
	documents LspDocuments
	conn      *jsonrpc.Conn
	mutex     sync.Mutex // Ensure thread safety when sending messages
}

func (l *lspServer) SendNotification(ctx context.Context, method string, params interface{}) error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.conn == nil {
		logs.Println("Connection is nil, cannot send notification")
		return fmt.Errorf("connection is nil")
	}

	// Use the Notify function to send the notification
	if err := l.conn.Notify(ctx, method, params); err != nil {
		logs.Printf("Failed to send notification: %v\n", err)
		return fmt.Errorf("failed to send notification: %w", err)
	}

	logs.Println("Notification sent successfully")
	return nil
}

func NewLspServer(name string) LspServer {
	return &lspServer{
		name: name,
	}
}

func (l *lspServer) Start(ctx context.Context) error {
	logs.Printf("LspServer starting...")

	switch *ParamBackend {
	case "openai":
		l.backend = NewOpenAiBackend()
	case "ollama":
		l.backend = NewOllamaBackend()
	default:
		logs.Printf("Invalid backend: %s", *ParamBackend)
		os.Exit(1)
	}

	l.documents = NewLspDocuments()

	cr := jsonrpc.NewFakeCloserReader(os.Stdin)
	cw := jsonrpc.NewFakeCloserWriter(os.Stdout)

	l.conn = jsonrpc.NewConn(cr, cw) // Server is the handler
	logs.Printf("[+] New LSP Document [ %s ] ", l.documents)
	return l.backend.Start()
}

func (l *lspServer) Shutdown() {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.conn != nil {
		logs.Println("Closing connection")
		l.conn.Close()
		l.conn = nil
	}
}

/*
* OnInitialized is called when the client is ready to receive requests.
* At this point the client has sent the initialize request and received the
* response and capabilities of the server.
*
* @param ctx The context of the request.
* @param req The initialize params.
* @return error Any error that occurred during the request
 */
func (l *lspServer) OnInitialized(ctx context.Context, req *defines.InitializeParams) error {
	logs.Printf("OnInitialized: %s", req)
	l.server.OnDidOpenTextDocument(l.OnDidOpenTextDocument)
	notificationMethod := "analysisDone"
	// Notification handle code can come here
	if err := l.NotifyGeneratedCode(ctx, "AnalysisDone", notificationMethod); err != nil {
		fmt.Println("Error sending notification: \n", err)
	}
	logs.Printf("[+] Notification Sent!\n")
	return nil
}

/*
* updateDocumentStore is helper for updating internal state whenever the document is opened
* or saved by the client.
*
* @param ctx The context of the request.
* @param req The open text document params.
* @return error Any error that occurred during the request
 */

func (l *lspServer) updateDocumentStore(uri string, text string) error {
	var analysis string
	var diagnostics []LspDiagnostic
	logs.Printf("=> URI: [%s] TEXT: [%s]", uri, text)
	err := l.documents.Store(uri, text)
	if err != nil {
		// This is ok, the document may already be stored
		return nil
	}

	const maxRetries = 5
	instruction := ""

	for attempts := 1; attempts <= maxRetries; attempts++ {
		analysis, err = l.backend.AnalyseDocument(uri, instruction+text)
		if err != nil {
			return err
		}
		err = l.documents.StoreAnalysis(uri, analysis)
		if err != nil {
			return err
		}
		diagnostics, err = DiagnosticsUnmarshal(uri, analysis)
		if err != nil {
			if attempts < maxRetries {
				logs.Printf("AnalyseDocument attempt %d/%d failed: %v. Retrying...", attempts, maxRetries, err)
			} else {
				logs.Printf("AnalyseDocument attempt %d/%d failed: %v. No more retries.", attempts, maxRetries, err)
				return err
			}
			var temp []byte
			temp, err = LoadPrompt(*ParamRetryPromptFile)
			instruction = string(temp)
		} else {
			break
		}
	}

	if err != nil {
		logs.Printf("Failed to analyze document after %d attempts: %v\n", maxRetries, err)
		return err
	}

	err = l.documents.UpdateDiagnostics(uri, diagnostics)
	if err != nil {
		logs.Printf("Failed to update diagnostics: %v\n", err)
		return err
	}

	logs.Printf("Diagnostics successfully updated for URI: %s", uri)
	return nil
}

/*
* OnDidOpenTextDocument is called when a text document is opened in a client.
*
* @param ctx The context of the request.
* @param req The open text document params from the client.
* @return error Any error that occurred during the request
 */

func (l *lspServer) OnDidOpenTextDocument(ctx context.Context, req *defines.DidOpenTextDocumentParams) error {
	logs.Printf("OnDidOpenTextDocument:\n%s", req)
	notificationMethod := "analysisStarted"
	// Notification handle code can come here
	if err := l.NotifyGeneratedCode(ctx, "AnalysisStarted", notificationMethod); err != nil {
		fmt.Println("Error sending notification: \n", err)
	}
	logs.Printf("[+] Notification Sent!\n")
	return l.updateDocumentStore(string(req.TextDocument.Uri), req.TextDocument.Text)
}

// ConvertFileURIToPath converts a file URI to a system-specific file path
func ConvertFileURIToPath(uri string) (string, error) {
	parsedURI, err := url.Parse(uri)
	if err != nil {
		return "", err
	}

	// Replace 'file:///' with ''
	path := strings.Replace(parsedURI.Path, "file:///", "", 1)

	// Handle Windows file paths (e.g., /C:/Users/... to C:/Users/...)
	if strings.HasPrefix(path, "/") && strings.Contains(path, ":") {
		path = path[1:]
	}

	return path, nil
}

func ReadFileContent(filePath string) (string, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (l *lspServer) OnDidChangeTextDocument(ctx context.Context, req *defines.DidChangeTextDocumentParams) error {
	uri := req.TextDocument.TextDocumentIdentifier.Uri

	logs.Printf("[+] OnDidChangeTextDocument: %s", string(uri))

	filePath, err := ConvertFileURIToPath(string(uri))

	if err != nil {
		logs.Printf("Error converting URI to file path: %v\n", err)
		return err
	}
	// var content string
	// var err error
	documentContent, err := ReadFileContent(filePath)
	// Fetch the latest content of the document
	// content, err := os.ReadFile(string(uri))
	if err != nil {
		logs.Printf("Error loading document content: %s", err)
		return err
	}
	notificationMethod := "analysisStarted"
	// Notification handle code can come here
	if err := l.NotifyGeneratedCode(ctx, "AnalysisStarted", notificationMethod); err != nil {
		fmt.Println("Error sending notification: \n", err)
	}
	logs.Printf("[+] Notification Sent!\n")
	// Analyze the document content
	analysis, err := l.backend.AnalyseDocument(string(uri), string(documentContent))
	if err != nil {
		logs.Printf("Error analyzing document: %s", err)
		return err
	}

	// Update the analysis in the document store
	if err = l.documents.StoreAnalysis(string(uri), analysis); err != nil {
		logs.Printf("Error storing document analysis: %s", err)
		return err
	}

	// Unmarshal diagnostics from the analysis
	diagnostics, err := DiagnosticsUnmarshal(string(uri), analysis)
	if err != nil {
		logs.Printf("Error unmarshalling diagnostics: %s", err)
		return err
	}

	err = l.documents.UpdateDiagnostics(string(uri), diagnostics)

	// Update diagnostics in the document store
	if err != nil {
		logs.Printf("Error updating diagnostics: %s", err)
		return err
	}
	// l.documents.UpdateDiagnostics(uri, diagnostics)
	// Send diagnostics to the client
	l.updateDocumentStore(string(uri), string(documentContent))
	notificationMethod = "analysisDone"
	// Notification handle code can come here
	if err := l.NotifyGeneratedCode(ctx, "AnalysisDone", notificationMethod); err != nil {
		fmt.Println("Error sending notification: \n", err)
	}
	logs.Printf("[+] Notification Sent!\n")
	return nil
}

func (l *lspServer) OnCodeActionWithSliceCodeAction(ctx context.Context, req *defines.CodeActionParams) (*[]defines.CodeAction, error) {
	logs.Printf("onCodeActionWithSliceCodeAction")

	// Get diagnostics for the document
	diagnostics, err := l.documents.GetDiagnostics(string(req.TextDocument.Uri))
	if err != nil {
		logs.Printf("Error getting diagnostics: %v\n", err)
		return nil, err
	}

	// Filter diagnostics to find those affecting the current cursor line
	var relevantDiagnostics []LspDiagnostic
	cursorLine := req.Range.Start.Line + 1
	for _, diag := range diagnostics {
		if diag.LineNumber == int(cursorLine) {
			relevantDiagnostics = append(relevantDiagnostics, diag)
		}
	}

	// Only create code actions if there are relevant diagnostics
	if len(relevantDiagnostics) == 0 {
		logs.Printf("No relevant diagnostics on this line, no code action created")
		return nil, nil
	}

	var actions []defines.CodeAction
	// Create a variable for the kind to take its address
	refactorKind := defines.CodeActionKindRefactorRewrite
	// Create a refactor action
	actionData := map[string]interface{}{
		"uri":   req.TextDocument.Uri,
		"range": req.Range,
	}
	refactorAction := defines.CodeAction{
		Title: "Ask LLM for Fix [FuzzLSP]",
		Kind:  &refactorKind,
		Data:  actionData,
	}
	// Add the action to the list
	actions = append(actions, refactorAction)

	quickKind := defines.CodeActionKindQuickFix
	refactorAction = defines.CodeAction{
		Title: "Explain issue [FuzzLSP]",
		Kind:  &quickKind,
		Data:  actionData,
	}
	// Add the action to the list
	actions = append(actions, refactorAction)

	return &actions, nil
}

func (l *lspServer) OnCodeActionResolve(ctx context.Context, req *defines.CodeAction) (*defines.CodeAction, error) {
	logs.Printf("OnCodeActionResolve")

	// Cast Data to map[string]interface{}
	actionData, ok := req.Data.(map[string]interface{})
	if !ok {
		logs.Printf("Failed to cast Data to map[string]interface{}")
		return nil, fmt.Errorf("failed to cast Data to map[string]interface{}")
	}

	// Extract URI
	documentURI := actionData["uri"].(string)

	// Extract Range from map
	rangeMap, ok := actionData["range"].(map[string]interface{})
	if !ok {
		logs.Printf("Failed to cast Range to map[string]interface{}")
		return nil, fmt.Errorf("failed to cast Range to map[string]interface{}")
	}

	// Convert rangeMap to defines.Range
	actionRange := defines.Range{
		Start: defines.Position{
			Line:      uint(rangeMap["start"].(map[string]interface{})["line"].(float64)),
			Character: uint(rangeMap["start"].(map[string]interface{})["character"].(float64)),
		},
		End: defines.Position{
			Line:      uint(rangeMap["end"].(map[string]interface{})["line"].(float64)),
			Character: uint(rangeMap["end"].(map[string]interface{})["character"].(float64)),
		},
	}

	// Convert URI to file path
	filePath, err := ConvertFileURIToPath(documentURI)
	if err != nil {
		logs.Printf("Error converting URI to file path: %v\n", err)
		return nil, err
	}

	// Load document content
	documentContent, err := ReadFileContent(filePath)
	if err != nil {
		logs.Printf("Error loading document content: %s", err)
		return nil, err
	}

	// Extract the line text based on the provided range
	lines := strings.Split(documentContent, "\n")
	cursorLine := int(actionRange.Start.Line)
	if cursorLine >= len(lines) {
		logs.Printf("Invalid cursor line: %d", cursorLine)
		return nil, fmt.Errorf("invalid cursor line: %d", cursorLine)
	}

	lineText := lines[cursorLine]

	// Handle the specific action (refactor or explain)
	if req.Kind != nil && *req.Kind == defines.CodeActionKindRefactorRewrite {
		refactoredText, err := l.backend.RefactorCodeLine(lineText)
		if err != nil {
			logs.Printf("LLM error for refactor: %v", err)
			return nil, err
		}

		// Apply the refactored text to the current line
		changes := map[string][]defines.TextEdit{
			documentURI: {
				{
					Range:   actionRange,
					NewText: refactoredText,
				},
			},
		}
		req.Edit = &defines.WorkspaceEdit{
			Changes: &changes,
		}
	}

	// Handle "Explain issue" action
	if req.Kind != nil && *req.Kind == defines.CodeActionKindQuickFix {
		explanation, err := l.backend.ExplainCodeIssue(lineText)
		if err != nil {
			logs.Printf("LLM error for explanation: %v", err)
			return nil, err
		}

		// Insert explanation as a comment above the line
		changes := map[string][]defines.TextEdit{
			documentURI: {
				{
					Range:   actionRange,
					NewText: "/* " + explanation + " */\n" + lineText,
				},
			},
		}
		req.Edit = &defines.WorkspaceEdit{
			Changes: &changes,
		}
	}

	return req, nil
}




func (l *lspServer) OnDidSaveTextDocument(ctx context.Context, req *defines.DidSaveTextDocumentParams) error {

	logs.Printf("OnDidSaveTextDocument:\n%s", req)

	logs.Printf("URI: %s | Text: %s ", string(req.TextDocument.Uri), req.Text)
	filePath, err := ConvertFileURIToPath(string(req.TextDocument.Uri))

	if err != nil {
		logs.Printf("Error converting URI to file path: %v\n", err)
		return err
	}
	// var content string
	// var err error
	documentContent, err := ReadFileContent(filePath)
	if err != nil {
		logs.Printf("Error loading document content: %s", err)
		return err
	}
	// TODO: Add IncludeText to server capabilities
	if documentContent != "" {
		return l.updateDocumentStore(string(req.TextDocument.Uri), documentContent)
	}

	return nil
}

/*
* OnDiagnostic is called when a text document is opened in a client.
* The client will send a notification to the server requesting diagnostics (Pull Diagnostics)
*
* @param ctx The context of the request.
* @param req The diagnostic document param from the client.
* @return report The full diagnostic report
* @return error Any error that occurred during the request
 */

func (l *lspServer) OnDiagnostic(ctx context.Context, req *defines.DocumentDiagnosticParams) (*defines.FullDocumentDiagnosticReport, error) {
	logs.Printf("OnDiagnostic called for URI: %s", req.TextDocument.Uri)

	diagnostics := []defines.Diagnostic{}
	report := defines.FullDocumentDiagnosticReport{}

	docDiagnostics, err := l.documents.GetDiagnostics(string(req.TextDocument.Uri))
	if err != nil {
		logs.Printf("Error getting diagnostics for URI %s: %v\n", req.TextDocument.Uri, err)
		return &report, nil
	}

	for _, d := range docDiagnostics {
		var diagnostic defines.Diagnostic
		var severity defines.DiagnosticSeverity
		message := DiagnosticToPrettyText(d)

		switch d.Severity {
		case "advisory":
			severity = defines.DiagnosticSeverityWarning
		case "mandatory":
			severity = defines.DiagnosticSeverityError
		default:
			severity = defines.DiagnosticSeverityHint
		}

		diagRange := defines.Range{
			Start: defines.Position{Line: uint(d.LineNumber - 1), Character: 0},
			End:   defines.Position{Line: uint(d.LineNumber - 1), Character: 5},
		}

		relatedInfo := []defines.DiagnosticRelatedInformation{
			{
				Location: defines.Location{
					Uri:   req.TextDocument.Uri,
					Range: diagRange,
				},
				Message: message,
			},
		}

		searchUrl := fmt.Sprintf("https://bing.com/search?=\"%s\"", d.Source)
		diagnostic = defines.Diagnostic{
			Range:              diagRange,
			Severity:           &severity,
			Code:               d.Source + " " + d.Rule,
			Source:             &l.name,
			Message:            d.Description,
			CodeDescription:    &defines.CodeDescription{Href: defines.URI(searchUrl)},
			RelatedInformation: &relatedInfo,
		}

		diagnostics = append(diagnostics, diagnostic)
	}

	var items []interface{}
	for _, d := range diagnostics {
		items = append(items, d)
	}
	report = defines.FullDocumentDiagnosticReport{
		Kind:  defines.DocumentDiagnosticReportKindFull,
		Items: items,
	}

	logs.Printf("Diagnostics report created with %d items for URI %s\n", len(items), req.TextDocument.Uri)
	return &report, nil
}

/*
* OnHover is called when a user hovers over a token in the editor. This method is then sent to the server
* which will return a Hover object to the client.
*
* @param ctx The context of the request.
* @param req The hover params from the client.
* @return The hover object sent to the client
* @return error Any error that occurred during the request
 */

func (l *lspServer) OnHover(ctx context.Context, req *defines.HoverParams) (result *defines.Hover, err error) {
	logs.Printf("OnHover: %s", req)
	var value string

	diagnostics, err := l.documents.GetDiagnostics(string(req.TextDocument.Uri))
	if err != nil {
		return nil, err
	}

	for _, d := range diagnostics {
		if d.LineNumber-1 != int(req.Position.Line) {
			continue
		}

		value, err = DiagnosticToJsonMarkup(d)
		if err != nil {
			break
		}
	}

	return &defines.Hover{
		Contents: defines.MarkupContent{
			Kind:  defines.MarkupKindMarkdown,
			Value: value,
		},
	}, nil
}

func strPtr(str string) *string {
	return &str
}

func kindPtr(kind defines.CompletionItemKind) *defines.CompletionItemKind {
	return &kind
}

func (s *lspServer) NotifyGeneratedCode(ctx context.Context, generatedCode string, notificationMethod string) error {
	if s.conn == nil {
		return fmt.Errorf("connection is nil, cannot send notification")
	}
	logs.Printf("NotifyGeneratedCode")
	// Send notification to the client
	err := s.conn.Notify(ctx, notificationMethod, generatedCode)
	if err != nil {
		return fmt.Errorf("failed to send generated code notification: %v", err)
	}
	logs.Printf("Notified!")
	return nil
}

func (l *lspServer) OnCompletion(ctx context.Context, req *defines.CompletionParams) (result *[]defines.CompletionItem, err error) {
	logs.Printf("Code Completion n Suggestion: %v", req)

	// Define the system prompt for code completion
	systemPrompt := "You are a coding assistant. Provide the best possible code completions based on the given context."

	// Fetch the document content
	filePath, err := ConvertFileURIToPath(string(req.TextDocument.Uri))
	if err != nil {
		logs.Printf("Error converting URI to file path: %v\n", err)
		return nil, err
	}

	documentContent, err := ReadFileContent(filePath)
	if err != nil {
		logs.Printf("Error reading file content: %v\n", err)
		return nil, err
	}

	if documentContent == "" {
		return nil, fmt.Errorf("failed to retrieve document content")
	}

	// Determine the position in the document
	line := int(req.Position.Line)
	character := int(req.Position.Character)

	// Split the document content into lines
	lines := strings.Split(documentContent, "\n")

	// Ensure the line number is within the valid range
	if line >= len(lines) {
		return nil, fmt.Errorf("line number out of range")
	}

	// Handle the prefix (previous 3 lines)
	startLine := line - 3
	if startLine < 0 {
		startLine = 0 // Adjust to the start of the document if fewer lines exist
	}
	prefixLines := lines[startLine:line]
	prefix := strings.Join(prefixLines, "\n")

	// Extract the prefix up to the current character on the current line
	if character > 0 && character <= len(lines[line]) {
		currentLinePrefix := lines[line][:character]
		prefix = prefix + "\n" + currentLinePrefix
	} else if character > len(lines[line]) {
		// Handle case where the character position exceeds the current line length
		return nil, fmt.Errorf("character position out of range")
	}

	// Handle the suffix (next 3 lines)
	endLine := line + 3
	if endLine > len(lines) {
		endLine = len(lines) // Adjust to the end of the document if fewer lines exist
	}
	suffixLines := lines[line+1 : endLine]
	suffix := strings.Join(suffixLines, "\n")

	// Extract the suffix after the current character on the current line
	if character >= 0 && character < len(lines[line]) {
		currentLineSuffix := lines[line][character:]
		suffix = currentLineSuffix + "\n" + suffix
	}

	// Call the backend to get completions with the custom system prompt
	completions, err := l.backend.CompleteCode(string(req.TextDocument.Uri), prefix, systemPrompt)
	if err != nil {
		logs.Printf("Error getting code completions: %v\n", err)
		return nil, err
	}
	logs.Println("Completion Done:", completions)
	// Generate additional code using the backend
	generatedCode, err := l.backend.GenerateCode(string(req.TextDocument.Uri), prefix, suffix, systemPrompt)
	if err != nil {
		logs.Printf("Error generating code: %v\n", err)
		return nil, err
	}
	logs.Println("Code Generated:", generatedCode)
	// Escape special characters in the generated code to make it JSON-safe
	escapedGeneratedCode := strings.ReplaceAll(generatedCode, "\n", "\\n")
	escapedGeneratedCode = strings.ReplaceAll(escapedGeneratedCode, "\"", "\\\"")
	logs.Printf("[+] escapedGeneratedCode! %s\n", escapedGeneratedCode)

	// Prepare the notification params with the generated code
	notificationMethod := "window/showGeneratedCode"
	// Notification handle code can come here
	if err := l.NotifyGeneratedCode(ctx, escapedGeneratedCode, notificationMethod); err != nil {
		fmt.Println("Error sending notification: \n", err)
	}

	logs.Printf("[+] Notification Sent!\n")
	// Map completions to CompletionItems
	var completionItems []defines.CompletionItem
	for _, comp := range completions {
		insertTextFormat := defines.InsertTextFormatPlainText
		completionItems = append(completionItems, defines.CompletionItem{
			Label:            comp,
			Kind:             kindPtr(defines.CompletionItemKindText),
			InsertText:       strPtr(comp),
			InsertTextFormat: &insertTextFormat,
		})
	}

	// Also include the generated code as a completion item (displayed in italic grey)
	insertTextFormat := defines.InsertTextFormatPlainText
	completionItems = append(completionItems, defines.CompletionItem{
		Label:            generatedCode,
		Kind:             kindPtr(defines.CompletionItemKindText),
		InsertText:       strPtr(generatedCode),
		InsertTextFormat: &insertTextFormat,
		Documentation:    strPtr("Generated suggestion"),
	})

	return &completionItems, nil
}

func Serve(name string) {
	lspserver := lspServer{name: name}
	lspserver.server = lsp.NewServer(&lsp.Options{
		CompletionProvider: &defines.CompletionOptions{
			TriggerCharacters: &[]string{"."},
		},
		CodeActionProvider: &defines.CodeActionOptions{
			ResolveProvider: &[]bool{true}[0],
		},
	})

	if lspserver.server == nil {
		panic("Error creating LspServer")
	}
	ctx := context.Background()
	err := lspserver.Start(ctx)
	if err != nil {
		logs.Printf("start failed: %v", err)
		os.Exit(1)
		// TODO: handle retrying
	}
	logs.Printf("Initializing!")
	lspserver.server.OnInitialized(lspserver.OnInitialized)
	lspserver.server.OnDidOpenTextDocument(lspserver.OnDidOpenTextDocument)
	lspserver.server.OnDidChangeTextDocument(lspserver.OnDidChangeTextDocument)
	lspserver.server.OnDidSaveTextDocument(lspserver.OnDidSaveTextDocument)
	lspserver.server.OnHover(lspserver.OnHover)
	lspserver.server.OnDiagnostic(lspserver.OnDiagnostic)
	logs.Printf("Doing Completion!")
	lspserver.server.OnCompletion(lspserver.OnCompletion)
	lspserver.server.OnCodeActionWithSliceCodeAction(lspserver.OnCodeActionWithSliceCodeAction)
	lspserver.server.OnCodeActionResolve(lspserver.OnCodeActionResolve)
	lspserver.server.Run()
}
