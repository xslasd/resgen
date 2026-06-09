package lsp

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
	"github.com/xslasd/resgen/internal/formatter"
	"github.com/xslasd/resgen/internal/parser"
)

var (
	// files 存储在内存中的虚拟文件缓存
	filesMu sync.RWMutex
	files   = make(map[string]string)
)

type SymbolInfo struct {
	Name     string
	Filename string
	Line     int
	Column   int
}

func initLog() {}

func writeLog(format string, a ...any) {}

func RunServer(version string) {
	initLog()
	writeLog("RunServer started with version: %s", version)

	handler := protocol.Handler{}

	handler.Initialize = func(context *glsp.Context, params *protocol.InitializeParams) (any, error) {
		writeLog("Initialize called. Client name: %s", params.ClientInfo.Name)
		capabilities := handler.CreateServerCapabilities()
		capabilities.DocumentFormattingProvider = true
		capabilities.DefinitionProvider = true
		capabilities.TextDocumentSync = protocol.TextDocumentSyncKindFull

		return protocol.InitializeResult{
			Capabilities: capabilities,
			ServerInfo: &protocol.InitializeResultServerInfo{
				Name:    "resgen-lsp",
				Version: &version,
			},
		}, nil
	}

	handler.TextDocumentDidOpen = func(context *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
		filesMu.Lock()
		defer filesMu.Unlock()
		files[params.TextDocument.URI] = params.TextDocument.Text
		return nil
	}

	handler.TextDocumentDidChange = func(context *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
		filesMu.Lock()
		defer filesMu.Unlock()
		if len(params.ContentChanges) > 0 {
			switch change := params.ContentChanges[0].(type) {
			case protocol.TextDocumentContentChangeEventWhole:
				files[params.TextDocument.URI] = change.Text
			case protocol.TextDocumentContentChangeEvent:
				files[params.TextDocument.URI] = change.Text
			case map[string]any:
				if text, ok := change["text"].(string); ok {
					files[params.TextDocument.URI] = text
				}
			}
		}
		return nil
	}

	handler.TextDocumentDidClose = func(context *glsp.Context, params *protocol.DidCloseTextDocumentParams) error {
		filesMu.Lock()
		defer filesMu.Unlock()
		delete(files, params.TextDocument.URI)
		return nil
	}

	handler.TextDocumentFormatting = func(context *glsp.Context, params *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
		writeLog("TextDocumentFormatting called for URI: %s", params.TextDocument.URI)
		filesMu.RLock()
		content, ok := files[params.TextDocument.URI]
		filesMu.RUnlock()
		if !ok {
			writeLog("TextDocumentFormatting error: file not found in memory: %s", params.TextDocument.URI)
			return nil, fmt.Errorf("file not found in memory: %s", params.TextDocument.URI)
		}

		filename := uriToPath(params.TextDocument.URI)
		schema, err := parser.ParseFileContent(filename, content)
		if err != nil {
			return nil, err
		}

		tabSize := 4
		if val, ok := params.Options["tabSize"]; ok {
			if f, ok := val.(float64); ok {
				tabSize = int(f)
			} else if i, ok := val.(int); ok {
				tabSize = i
			}
		}

		var sb strings.Builder
		f := formatter.NewFormatter(tabSize)
		if err := f.Format(schema, &sb); err != nil {
			return nil, err
		}

		formatted := sb.String()

		lines := strings.Split(content, "\n")
		lineCount := len(lines)
		lastLineLen := 0
		if lineCount > 0 {
			lastLineLen = len(lines[lineCount-1])
		}

		return []protocol.TextEdit{
			{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 0},
					End:   protocol.Position{Line: uint32(lineCount), Character: uint32(lastLineLen)},
				},
				NewText: formatted,
			},
		}, nil
	}

	handler.TextDocumentDefinition = func(context *glsp.Context, params *protocol.DefinitionParams) (any, error) {
		filesMu.RLock()
		content, ok := files[params.TextDocument.URI]
		filesMu.RUnlock()
		if !ok {
			return nil, nil
		}

		filename := uriToPath(params.TextDocument.URI)
		schema, err := parser.ParseFileContent(filename, content)
		if err != nil {
			return nil, nil
		}

		targetLine := int(params.Position.Line) + 1
		targetCol := int(params.Position.Character) + 1

		ident := findIdentifierAt(schema, targetLine, targetCol)
		if ident == "" {
			return nil, nil
		}

		symbols := buildSymbolTable(filename)
		sym, found := symbols[ident]
		if !found {
			return nil, nil
		}

		return protocol.Location{
			URI: pathToURI(sym.Filename),
			Range: protocol.Range{
				Start: protocol.Position{Line: uint32(sym.Line - 1), Character: uint32(sym.Column - 1)},
				End:   protocol.Position{Line: uint32(sym.Line - 1), Character: uint32(sym.Column - 1 + len(sym.Name))},
			},
		}, nil
	}

	s := server.NewServer(&handler, "resgen-lsp", false)
	s.RunStdio()
}

func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	if u.Scheme != "file" {
		return uri
	}
	path := u.Path
	if len(path) > 0 && path[0] == '/' {
		if len(path) > 2 && path[2] == ':' {
			path = path[1:]
		}
	}
	return filepath.Clean(path)
}

func pathToURI(path string) string {
	absPath, err := filepath.Abs(path)
	if err == nil {
		path = absPath
	}
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "file://" + path
}

func buildSymbolTable(currentFile string) map[string]SymbolInfo {
	symbols := make(map[string]SymbolInfo)
	dir := filepath.Dir(currentFile)

	// 1. 扫描内存中的文件（如果在同一个目录下）
	filesMu.RLock()
	for cachedURI, cachedContent := range files {
		filePath := uriToPath(cachedURI)
		if filepath.Dir(filePath) == dir {
			schema, err := parser.ParseFileContent(filePath, cachedContent)
			if err == nil {
				addSymbols(schema, filePath, symbols)
			}
		}
	}
	filesMu.RUnlock()

	// 2. 扫描磁盘上的文件（排除内存中已有的，避免重复解析）
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".res" {
				continue
			}
			filePath := filepath.Join(dir, entry.Name())
			
			isCached := false
			filesMu.RLock()
			for cachedURI := range files {
				if uriToPath(cachedURI) == filePath {
					isCached = true
					break
				}
			}
			filesMu.RUnlock()

			if isCached {
				continue
			}

			schema, err := parser.ParseFile(filePath)
			if err == nil {
				addSymbols(schema, filePath, symbols)
			}
		}
	}

	return symbols
}

func addSymbols(schema *parser.Schema, filePath string, symbols map[string]SymbolInfo) {
	for _, decl := range schema.Declarations {
		if decl.Model != nil {
			symbols[decl.Model.Name] = SymbolInfo{
				Name:     decl.Model.Name,
				Filename: filePath,
				Line:     decl.Model.Pos.Line,
				Column:   decl.Model.Pos.Column,
			}
		}
		if decl.Scalar != nil {
			symbols[decl.Scalar.Name] = SymbolInfo{
				Name:     decl.Scalar.Name,
				Filename: filePath,
				Line:     decl.Scalar.Pos.Line,
				Column:   decl.Scalar.Pos.Column,
			}
		}
		if decl.Decorator != nil {
			symbols[decl.Decorator.Name] = SymbolInfo{
				Name:     decl.Decorator.Name,
				Filename: filePath,
				Line:     decl.Decorator.Pos.Line,
				Column:   decl.Decorator.Pos.Column,
			}
		}
	}
}

func findIdentifierAt(schema *parser.Schema, line, col int) string {
	var foundIdent string

	inRange := func(pos lexer.Position, name string) bool {
		if pos.Line != line {
			return false
		}
		return col >= pos.Column && col <= pos.Column+len(name)
	}

	var visitTypeRef func(t parser.TypeRef) bool
	visitTypeRef = func(t parser.TypeRef) bool {
		if inRange(t.Pos, t.Name) {
			foundIdent = t.Name
			return true
		}
		for _, arg := range t.TypeArgs {
			if visitTypeRef(arg) {
				return true
			}
		}
		return false
	}

	var visitDirective func(d parser.DirectiveUsage) bool
	visitDirective = func(d parser.DirectiveUsage) bool {
		if col >= d.Pos.Column && col <= d.Pos.Column+len(d.Name)+1 {
			foundIdent = d.Name
			return true
		}
		return false
	}

	for _, decl := range schema.Declarations {
		if decl.Model != nil {
			m := decl.Model
			for _, prop := range m.Properties {
				if visitTypeRef(prop.Type) {
					return foundIdent
				}
				for _, dir := range prop.Directives {
					if visitDirective(dir) {
						return foundIdent
					}
				}
			}
			for _, dir := range m.Directives {
				if visitDirective(dir) {
					return foundIdent
				}
			}
		}
		if decl.Group != nil {
			g := decl.Group
			for _, dir := range g.Directives {
				if visitDirective(dir) {
					return foundIdent
				}
			}
			for _, ep := range g.Endpoints {
				for _, dir := range ep.Directives {
					if visitDirective(dir) {
						return foundIdent
					}
				}
				if visitTypeRef(ep.ReturnType) {
					return foundIdent
				}
				for _, arg := range ep.Args {
					for _, dir := range arg.Directives {
						if visitDirective(dir) {
							return foundIdent
						}
					}
					if visitTypeRef(arg.Type) {
						return foundIdent
					}
				}
			}
		}
	}

	return foundIdent
}
