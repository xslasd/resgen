package lsp

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	Kind     string
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
		capabilities.HoverProvider = true
		capabilities.TextDocumentSync = protocol.TextDocumentSyncKindFull

		return protocol.InitializeResult{
			Capabilities: capabilities,
			ServerInfo: &protocol.InitializeResultServerInfo{
				Name:    "resgen-lsp",
				Version: &version,
			},
		}, nil
	}

	handler.TextDocumentHover = func(context *glsp.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
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

		var hoverText string
		switch strings.ToLower(ident) {
		case "alias":
			hoverText = "### Built-in Directive: `@alias`\n\n用于指定模型字段或接口入参在传输层（JSON/Form/Query/Path/Header）使用的自定义别名，方便兼容老系统或不规范的接口命名规范。\n\n**示例**:\n```res\ninput QueryInput {\n    startTime: IntTime @alias(\"st_time\")\n}\n```"
		case "path":
			hoverText = "### Built-in Directive: `@path`\n\n标记参数来源于 URL 路径变量 (Path Variable)。"
		case "query":
			hoverText = "### Built-in Directive: `@query`\n\n标记参数来源于 URL 查询字符串 (Query String)。"
		case "header":
			hoverText = "### Built-in Directive: `@header`\n\n标记参数来源于 HTTP 请求头 (Header)。"
		case "required":
			hoverText = "### Built-in Directive: `@required`\n\n标记字段或形参必填且非空。"
		case "custombind":
			hoverText = "### Built-in Directive: `@customBind`\n\n接管参数绑定逻辑，由业务层在 Resolver 中手动实现 Bind 方法。"
		case "customvalidate":
			hoverText = "### Built-in Directive: `@customValidate`\n\n接管参数校验逻辑，由业务层在 Resolver 中手动实现 Validate 方法。"
		}

		if hoverText != "" {
			return &protocol.Hover{
				Contents: protocol.MarkupContent{
					Kind:  protocol.MarkupKindMarkdown,
					Value: hoverText,
				},
			}, nil
		}

		return nil, nil
	}

	handler.TextDocumentDidOpen = func(context *glsp.Context, params *protocol.DidOpenTextDocumentParams) error {
		filesMu.Lock()
		files[params.TextDocument.URI] = params.TextDocument.Text
		filesMu.Unlock()
		
		go publishDiagnostics(context, params.TextDocument.URI, params.TextDocument.Text)
		return nil
	}

	handler.TextDocumentDidChange = func(context *glsp.Context, params *protocol.DidChangeTextDocumentParams) error {
		filesMu.Lock()
		var text string
		if len(params.ContentChanges) > 0 {
			switch change := params.ContentChanges[0].(type) {
			case protocol.TextDocumentContentChangeEventWhole:
				files[params.TextDocument.URI] = change.Text
				text = change.Text
			case protocol.TextDocumentContentChangeEvent:
				files[params.TextDocument.URI] = change.Text
				text = change.Text
			case map[string]any:
				if t, ok := change["text"].(string); ok {
					files[params.TextDocument.URI] = t
					text = t
				}
			}
		}
		filesMu.Unlock()
		
		if text != "" {
			go publishDiagnostics(context, params.TextDocument.URI, text)
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
				Kind:     "Model",
				Filename: filePath,
				Line:     decl.Model.Pos.Line,
				Column:   decl.Model.Pos.Column,
			}
		}
		if decl.Scalar != nil {
			symbols[decl.Scalar.Name] = SymbolInfo{
				Name:     decl.Scalar.Name,
				Kind:     "Scalar",
				Filename: filePath,
				Line:     decl.Scalar.Pos.Line,
				Column:   decl.Scalar.Pos.Column,
			}
		}
		if decl.Decorator != nil {
			symbols[decl.Decorator.Name] = SymbolInfo{
				Name:     decl.Decorator.Name,
				Kind:     "Decorator",
				Filename: filePath,
				Line:     decl.Decorator.Pos.Line,
				Column:   decl.Decorator.Pos.Column,
			}
		}
		if decl.Enum != nil {
			symbols[decl.Enum.Name] = SymbolInfo{
				Name:     decl.Enum.Name,
				Kind:     "Enum",
				Filename: filePath,
				Line:     decl.Enum.Pos.Line,
				Column:   decl.Enum.Pos.Column,
			}
		}
		if decl.Union != nil {
			symbols[decl.Union.Name] = SymbolInfo{
				Name:     decl.Union.Name,
				Kind:     "Union",
				Filename: filePath,
				Line:     decl.Union.Pos.Line,
				Column:   decl.Union.Pos.Column,
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

	visitDirective := func(d parser.DirectiveUsage) bool {
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
		if decl.Enum != nil {
			if inRange(decl.Enum.Pos, decl.Enum.Name) {
				return decl.Enum.Name
			}
		}
		if decl.Union != nil {
			if inRange(decl.Union.Pos, decl.Union.Name) {
				return decl.Union.Name
			}
			for _, c := range decl.Union.Cases {
				if inRange(c.Pos, c.Type) {
					return c.Type
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
				if ep.ReturnType != nil && visitTypeRef(*ep.ReturnType) {
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

func publishDiagnostics(context *glsp.Context, uri, content string) {
	filename := uriToPath(uri)
	schema, err := parser.ParseFileContent(filename, content)
	var diagnostics []protocol.Diagnostic

	if err != nil {
		errStr := err.Error()
		re := regexp.MustCompile(`^.*?:(\d+):(\d+):\s*(.*)`)
		matches := re.FindStringSubmatch(errStr)

		var line, col uint32
		msg := errStr
		if len(matches) == 4 {
			if l, e := strconv.ParseUint(matches[1], 10, 32); e == nil {
				line = uint32(l)
			}
			if c, e := strconv.ParseUint(matches[2], 10, 32); e == nil {
				col = uint32(c)
			}
			msg = matches[3]
		}
		
		if line > 0 {
			line-- 
		}
		if col > 0 {
			col--
		}

		severity := protocol.DiagnosticSeverityError
		source := "resgen"
		diagnostics = append(diagnostics, protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{Line: line, Character: col},
				End:   protocol.Position{Line: line, Character: col + 1},
			},
			Severity: &severity,
			Source:   &source,
			Message:  msg,
		})
	} else if schema != nil {
		severity := protocol.DiagnosticSeverityError
		source := "resgen"
		
		modelMap := make(map[string]*parser.ModelDecl)
		for _, decl := range schema.Declarations {
			if decl.Model != nil {
				modelMap[decl.Model.Name] = decl.Model
			}
		}

		var checkHasFile func(t parser.TypeRef) bool
		checkHasFile = func(t parser.TypeRef) bool {
			if t.Name == "File" {
				return true
			}
			if m, ok := modelMap[t.Name]; ok {
				for _, prop := range m.Properties {
					if checkHasFile(prop.Type) {
						return true
					}
				}
			}
			for _, arg := range t.TypeArgs {
				if checkHasFile(arg) {
					return true
				}
			}
			return false
		}

		// 校验指令使用（如 @alias 参数必须有效）
		checkDirectives := func(dirs []parser.DirectiveUsage) {
			for _, d := range dirs {
				if strings.EqualFold(d.Name, "alias") {
					aliasVal := ""
					if len(d.Args) > 0 {
						arg := d.Args[0]
						if arg.Value.String != nil {
							aliasVal = *arg.Value.String
						} else if arg.Value.Ident != nil {
							aliasVal = *arg.Value.Ident
						} else {
							diagnostics = append(diagnostics, protocol.Diagnostic{
								Range: protocol.Range{
									Start: protocol.Position{Line: uint32(d.Pos.Line - 1), Character: uint32(d.Pos.Column - 1)},
									End:   protocol.Position{Line: uint32(d.Pos.Line - 1), Character: uint32(d.Pos.Column + len(d.Name) + 1)},
								},
								Severity: &severity,
								Source:   &source,
								Message:  "语义错误：@alias 指令的参数必须是字符串或标识符，例如 @alias(\"st_time\")",
							})
							continue
						}
					}
					for _, m := range d.Meta {
						if strings.EqualFold(m.Key, "name") || strings.EqualFold(m.Key, "alias") {
							aliasVal = m.Value.MetaStr()
						}
					}
					if strings.TrimSpace(aliasVal) == "" {
						diagnostics = append(diagnostics, protocol.Diagnostic{
							Range: protocol.Range{
								Start: protocol.Position{Line: uint32(d.Pos.Line - 1), Character: uint32(d.Pos.Column - 1)},
								End:   protocol.Position{Line: uint32(d.Pos.Line - 1), Character: uint32(d.Pos.Column + len(d.Name) + 1)},
							},
							Severity: &severity,
							Source:   &source,
							Message:  "语义错误：@alias 指令必须包含一个非空的别名参数，例如 @alias(\"st_time\")",
						})
					}
				}
			}
		}

		for _, decl := range schema.Declarations {
			if decl.Model != nil {
				for _, prop := range decl.Model.Properties {
					checkDirectives(prop.Directives)
				}
			}
			if decl.Group != nil {
				for _, ep := range decl.Group.Endpoints {
					for _, arg := range ep.Args {
						checkDirectives(arg.Directives)
					}
				}
			}
			if decl.Scalar != nil {
				if decl.Scalar.Base == "File" {
					diagnostics = append(diagnostics, protocol.Diagnostic{
						Range: protocol.Range{
							Start: protocol.Position{Line: uint32(decl.Scalar.Pos.Line - 1), Character: uint32(decl.Scalar.Pos.Column - 1)},
							End:   protocol.Position{Line: uint32(decl.Scalar.Pos.Line - 1), Character: uint32(decl.Scalar.Pos.Column + len(decl.Scalar.Name))},
						},
						Severity: &severity,
						Source:   &source,
						Message:  fmt.Sprintf("语义错误：自定义标量 '%s' 不能继承自 File 类型", decl.Scalar.Name),
					})
				}
			}
			if decl.Union != nil {
				for _, c := range decl.Union.Cases {
					if c.Type == "File" {
						diagnostics = append(diagnostics, protocol.Diagnostic{
							Range: protocol.Range{
								Start: protocol.Position{Line: uint32(c.Pos.Line - 1), Character: uint32(c.Pos.Column - 1)},
								End:   protocol.Position{Line: uint32(c.Pos.Line - 1), Character: uint32(c.Pos.Column + len(c.Type))},
							},
							Severity: &severity,
							Source:   &source,
							Message:  fmt.Sprintf("语义错误：联合类型分支 '%s' 不能使用 File 类型", c.Key),
						})
					}
				}
				symbols := buildSymbolTable(filename)
				sym, ok := symbols[decl.Union.ParamName]
				if !ok || sym.Kind != "Enum" {
					diagnostics = append(diagnostics, protocol.Diagnostic{
						Range: protocol.Range{
							Start: protocol.Position{Line: uint32(decl.Union.Pos.Line - 1), Character: uint32(decl.Union.Pos.Column - 1)},
							End:   protocol.Position{Line: uint32(decl.Union.Pos.Line - 1), Character: uint32(decl.Union.Pos.Column + len(decl.Union.Name))},
						},
						Severity: &severity,
						Source:   &source,
						Message:  fmt.Sprintf("语义错误：联合类型 '%s' 的判别器 '%s' 无效，必须是一个枚举 (Enum) 类型", decl.Union.Name, decl.Union.ParamName),
					})
				}
			}

			if decl.Model != nil {
				for _, prop := range decl.Model.Properties {
					if prop.Type.Name == "Field" {
						diagnostics = append(diagnostics, protocol.Diagnostic{
							Range: protocol.Range{
								Start: protocol.Position{Line: uint32(prop.Pos.Line - 1), Character: uint32(prop.Pos.Column - 1)},
								End:   protocol.Position{Line: uint32(prop.Pos.Line - 1), Character: uint32(prop.Pos.Column + len(prop.Name))},
							},
							Severity: &severity,
							Source:   &source,
							Message:  fmt.Sprintf("语义错误：模型属性 '%s.%s' 不能使用 'Field' 类型。'Field' 是专属于校验器形参的字段引用元类型，绝对不能作为普通属性类型！若需表达动态或任意结构数据，请选用 'Any' 类型", decl.Model.Name, prop.Name),
						})
					}
					if decl.Model.Keyword != "input" && prop.Type.Name == "File" {
						diagnostics = append(diagnostics, protocol.Diagnostic{
							Range: protocol.Range{
								Start: protocol.Position{Line: uint32(prop.Pos.Line - 1), Character: uint32(prop.Pos.Column - 1)},
								End:   protocol.Position{Line: uint32(prop.Pos.Line - 1), Character: uint32(prop.Pos.Column + len(prop.Name))},
							},
							Severity: &severity,
							Source:   &source,
							Message:  fmt.Sprintf("语义错误：输出模型 '%s.%s' 不能包含 'File' 属性字段。文件流无法在 JSON/XML 结构体内部嵌套序列化返回；若接口需返回/下载文件，请将接口的返回值直接声明为 'File' (例如 => Download(): File [ctype=stream])", decl.Model.Name, prop.Name),
						})
					}
				}
			}
			if decl.Group != nil {
				for _, ep := range decl.Group.Endpoints {
					if len(ep.Args) > 1 {
						for _, arg := range ep.Args {
							if arg.Name == "" {
								diagnostics = append(diagnostics, protocol.Diagnostic{
									Range: protocol.Range{
										Start: protocol.Position{Line: uint32(arg.Pos.Line - 1), Character: uint32(arg.Pos.Column - 1)},
										End:   protocol.Position{Line: uint32(arg.Pos.Line - 1), Character: uint32(arg.Pos.Column + len(arg.Type.Name))},
									},
									Severity: &severity,
									Source:   &source,
									Message:  fmt.Sprintf("端点 %s 语义错误: 匿名参数 (顶级 Payload) 只能作为唯一参数使用，不可与其他参数混用", ep.Name),
								})
								break
							}
						}
					}

					if ep.ReturnType != nil {
						var checkFileArray func(t parser.TypeRef) bool
						checkFileArray = func(t parser.TypeRef) bool {
							if t.Name == "File" && t.IsArray {
								return true
							}
							for _, arg := range t.TypeArgs {
								if checkFileArray(arg) {
									return true
								}
							}
							return false
						}
						if checkFileArray(*ep.ReturnType) {
							diagnostics = append(diagnostics, protocol.Diagnostic{
								Range: protocol.Range{
									Start: protocol.Position{Line: uint32(ep.ReturnType.Pos.Line - 1), Character: uint32(ep.ReturnType.Pos.Column - 1)},
									End:   protocol.Position{Line: uint32(ep.ReturnType.Pos.Line - 1), Character: uint32(ep.ReturnType.Pos.Column + len(ep.ReturnType.Name) + 2)},
								},
								Severity: &severity,
								Source:   &source,
								Message:  fmt.Sprintf("语义错误：接口 [%s %s] 的出参不能声明为文件数组 '[File]'。单个 HTTP 响应无法承载多个独立文件流。推荐方案：1) 打包为 ZIP 压缩流返回 (返回类型设为 File [ctype=zip])；2) 或返回包含下载地址的文件列表结构 (例如 ResData<[FileItem]>)。", ep.Method, ep.Path),
							})
						}
					}

					hasFile := false
					for _, arg := range ep.Args {
						if checkHasFile(arg.Type) {
							hasFile = true
							break
						}
					}
					if hasFile {
						ctype := ""
						for _, meta := range ep.RequestMeta {
							if strings.ToLower(meta.Key) == "ctype" && meta.Value.Str != nil {
								ctype = strings.ToLower(*meta.Value.Str)
							}
						}
						if ctype == "" {
							for _, meta := range ep.ResponseMeta {
								if strings.ToLower(meta.Key) == "ctype" && meta.Value.Str != nil {
									ctype = strings.ToLower(*meta.Value.Str)
								}
							}
						}
						isJsonCtype := ctype == "json" || ctype == "application/json"
						isFormCtype := ctype == "form" || ctype == "application/x-www-form-urlencoded"
						
						if isJsonCtype {
							diagnostics = append(diagnostics, protocol.Diagnostic{
								Range: protocol.Range{
									Start: protocol.Position{Line: uint32(ep.Pos.Line - 1), Character: uint32(ep.Pos.Column - 1)},
									End:   protocol.Position{Line: uint32(ep.Pos.Line - 1), Character: uint32(ep.Pos.Column + len(ep.Name))},
								},
								Severity: &severity,
								Source:   &source,
								Message:  fmt.Sprintf("语义错误：接口 [%s %s] 的 input 中包含 File 字段，不能与 ctype=json 同时使用。File 字段必须通过 multipart/form-data 传输，请将 ctype 改为 multipart", ep.Method, ep.Path),
							})
						} else if isFormCtype {
							diagnostics = append(diagnostics, protocol.Diagnostic{
								Range: protocol.Range{
									Start: protocol.Position{Line: uint32(ep.Pos.Line - 1), Character: uint32(ep.Pos.Column - 1)},
									End:   protocol.Position{Line: uint32(ep.Pos.Line - 1), Character: uint32(ep.Pos.Column + len(ep.Name))},
								},
								Severity: &severity,
								Source:   &source,
								Message:  fmt.Sprintf("语义错误：接口 [%s %s] 的 input 中包含 File 字段，不能与 ctype=form（application/x-www-form-urlencoded）同时使用。文件上传必须使用 multipart/form-data，请将 ctype 改为 multipart", ep.Method, ep.Path),
							})
						}
					}
				}
			}
		}
	}

	context.Notify(protocol.ServerTextDocumentPublishDiagnostics, protocol.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})
}
