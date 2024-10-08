package lspserver

var ParamBackend *string
var ParamPromptFile *string
var ParamConnectTest *bool
var ParamRetryPromptFile *string
/* Backend agnostic methods */
type LspBackend interface {
	Start() error
	AnalyseDocument(string, string) (string, error)
	CompleteCode(string, string, string) ([]string, error)
	GenerateCode(string, string, string, string) (string, error)
	RefactorCodeLine(line string) (string, error)
	ExplainCodeIssue(line string) (string, error)
}
