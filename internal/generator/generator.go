package generator

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"

	"github.com/xslasd/resgen/internal/config"
	"github.com/xslasd/resgen/internal/parser"

	"golang.org/x/tools/imports"
)

//go:embed templates/engine.tmpl
var engineTmpl string

//go:embed templates/module.tmpl
var moduleTmpl string

//go:embed templates/api.html
var apiHtmlTmpl string

func formatTagName(name, strategy string) string {
	switch strategy {
	case "snake":
		return toSnakeCase(name)
	case "camel":
		return toCamelCase(name)
	case "lower":
		return strings.ToLower(name)
	default:
		return name
	}
}

func toSnakeCase(s string) string {
	var res []rune
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			res = append(res, '_')
		}
		res = append(res, unicode.ToLower(r))
	}
	return string(res)
}

func toCamelCase(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// capitalize 将字符串首字母大写，同时剥离可能的 @ 前缀
func capitalize(s string) string {
	s = strings.TrimPrefix(s, "@")
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// metaGet 从 MetaEntry 列表中按 key 查找字符串值（不区分大小写）
func metaGet(entries []parser.MetaEntry, key string) (string, bool) {
	for _, e := range entries {
		if strings.EqualFold(e.Key, key) {
			if e.Value.Str != nil {
				return *e.Value.Str, true
			}
			if e.Value.Int != nil {
				return fmt.Sprintf("%d", *e.Value.Int), true
			}
		}
	}
	return "", false
}

// metaGetInt 从 MetaEntry 列表中按 key 查找整数值（不区分大小写）
func metaGetInt(entries []parser.MetaEntry, key string) (int, bool) {
	for _, e := range entries {
		if strings.EqualFold(e.Key, key) {
			if e.Value.Int != nil {
				return int(*e.Value.Int), true
			}
		}
	}
	return 0, false
}

// metaGetBool 从 MetaEntry 列表中按 key 查找布尔值（不区分大小写）
func metaGetBool(entries []parser.MetaEntry, key string) (bool, bool) {
	for _, e := range entries {
		if strings.EqualFold(e.Key, key) {
			if e.Value.Str != nil {
				v := strings.ToLower(*e.Value.Str)
				return v == "true", true
			}
		}
	}
	return false, false
}

// ToGoType 将 DSL 的类型引用转换为 Go 类型字符串
func ToGoType(t parser.TypeRef, conf *config.Config, extraImports *[]string, context string, modelMap map[string]*ModelInfo) string {


	if conf != nil && conf.Scalars != nil {
		if mapping, ok := conf.Scalars[t.Name]; ok && mapping.Model != "" {
			rawType := mapping.Model
			if mapping.Target != "" {
				rawType = mapping.Target
			}
			goBaseType, importPath := parseCustomType(rawType)
			if importPath != "" && extraImports != nil {
				addImport(extraImports, importPath)
			}
			return applyTypeModifiers(goBaseType, t)
		}
	}



	goBaseType := t.Name
	switch t.Name {
	case "String":
		goBaseType = "string"
	case "Int":
		goBaseType = "int"
	case "Float":
		goBaseType = "float64"
	case "Boolean":
		goBaseType = "bool"
	case "File":
		goBaseType = "*multipart.FileHeader"
	case "Any", "Field":
		goBaseType = "any"
	}

	if len(t.TypeArgs) > 0 {
		isWrapper := false
		if modelMap != nil {
			if m, ok := modelMap[t.Name]; ok && m.IsWrapper {
				isWrapper = true
			}
		}

		if !isWrapper {
			var args []string
			for _, arg := range t.TypeArgs {
				args = append(args, ToGoType(arg, conf, extraImports, context, modelMap))
			}
			goBaseType += "[" + strings.Join(args, ", ") + "]"
		}
	}

	return applyTypeModifiers(goBaseType, t)
}

func formatTypeRef(t parser.TypeRef) string {
	res := t.Name
	if len(t.TypeArgs) > 0 {
		var args []string
		for _, arg := range t.TypeArgs {
			args = append(args, formatTypeRef(arg))
		}
		res += "<" + strings.Join(args, ", ") + ">"
	}
	if t.IsArray {
		res = "[" + res + "]"
	}
	return res
}

func addImport(imports *[]string, path string) {
	for _, imp := range *imports {
		if imp == path {
			return
		}
	}
	*imports = append(*imports, path)
}

func parseCustomType(raw string) (typeName string, importPath string) {
	lastSlash := strings.LastIndex(raw, "/")
	if lastSlash == -1 {
		return raw, ""
	}
	lastDot := strings.LastIndex(raw, ".")
	if lastDot == -1 || lastDot < lastSlash {
		return raw[lastSlash+1:], raw
	}
	importPath = raw[:lastDot]
	typeName = raw[lastSlash+1:]
	return typeName, importPath
}

func applyTypeModifiers(base string, t parser.TypeRef) string {
	res := base
	if !t.ItemNotNull && t.Name != "File" && t.Name != "Any" && t.Name != "Field" && !strings.HasPrefix(res, "*") && !strings.Contains(res, "any") {
		res = "*" + res
	}
	if t.IsArray {
		res = "[]" + res
		if !t.ArrNotNull {
			res = "*" + res
		}
	}
	return res
}

type ModelField struct {
	Name         string     `json:"name"`
	Doc          string     `json:"doc,omitempty"`
	Type         string     `json:"type"`
	GoType       string     `json:"-"`
	BaseGoType   string     `json:"-"`
	IsScalar     bool       `json:"-"`
	JSONName     string     `json:"jsonName"`
	OriginalType string     `json:"originalType"`
	GoValue      string     `json:"value,omitempty"`
	Validators   []MetaInfo `json:"validators,omitempty"`
	Tag          string     `json:"-"`
	Source       string     `json:"-"` // Added: Path, Query, Header, Body
	RefModel     *ModelInfo `json:"-"`
	GoTypeDTO    string     `json:"-"`
	IsArray      bool       `json:"-"`
	IsPointer    bool       `json:"-"`
	IsArrayElementPointer bool       `json:"-"`
	ScalarModel           string     `json:"-"`
}

type ScalarInfo struct {
	Name     string `json:"name"`
	Doc      string `json:"doc,omitempty"`
	BaseType string `json:"baseType"`
	GoType   string `json:"goType"`
}

type ModelInfo struct {
	Name       string       `json:"name"`
	Doc        string       `json:"doc,omitempty"`
	Module     string       `json:"module,omitempty"`
	IsInput    bool         `json:"isInput"`
	IsWrapper  bool         `json:"isWrapper"`
	HasScalar  bool         `json:"-"`
	TypeParams []string     `json:"typeParams,omitempty"`
	Fields     []ModelField `json:"fields"`
}

type ModuleInfo struct {
	Name                  string       `json:"name"`
	Doc                   string       `json:"doc,omitempty"`
	Groups                []GroupInfo  `json:"groups"`
	Models                []*ModelInfo `json:"models"`
	SpecializedDecorators []MetaInfo   `json:"specializedDecorators,omitempty"`
}

type GroupInfo struct {
	Name               string       `json:"name"`
	Doc                string       `json:"doc,omitempty"`
	Path               string       `json:"path"`
	RequestDecorators  []MetaInfo   `json:"-"`
	InvokeDecorators   []MetaInfo   `json:"-"`
	ResponseDecorators []MetaInfo   `json:"-"`
	Endpoints          []MethodInfo `json:"endpoints"`
}

type MethodInfo struct {
	Name               string         `json:"name"`
	Doc                string         `json:"doc,omitempty"`
	Method             string         `json:"method"`
	Path               string         `json:"path"`
	FullPath           string         `json:"fullPath"`
	InputName          string         `json:"inputName,omitempty"`
	InputModel         *ModelInfo     `json:"-"`
	ReturnType         string         `json:"returnType"`
	ReturnModel        *ModelInfo     `json:"-"`
	InnerReturnModel   *ModelInfo     `json:"-"`
	ReturnTypeDSL      string         `json:"returnTypeDSL,omitempty"`
	InnerReturnType    string         `json:"innerReturnType,omitempty"`
	IsReturnWrapped    bool           `json:"isReturnWrapped"`
	IsReturnArray      bool           `json:"isReturnArray"`
	ReturnTypeBase     string         `json:"returnTypeBase,omitempty"`
	ErrorType          string         `json:"errorType,omitempty"`
	IsErrorWrapped     bool           `json:"isErrorWrapped"`
	ErrorTypeBase      string         `json:"errorTypeBase,omitempty"`
	SuccessStatus      int            `json:"successStatus"`
	RequestDecorators  []MetaInfo     `json:"-"`
	InvokeDecorators   []MetaInfo     `json:"-"`
	ResponseDecorators []MetaInfo     `json:"-"`
	Args               []ArgumentInfo `json:"args,omitempty"`
	IsArgsWrapped      bool           `json:"isArgsWrapped"`
	ContentType        string         `json:"-"`
	MimeType           string         `json:"contentType"`
	ResponseMimeType   string         `json:"responseContentType"`
	ErrorMimeType      string         `json:"errorContentType,omitempty"`
	ResponseRenderFunc string         `json:"-"` // e.g. "Json", "Text"
	ErrorRenderFunc    string         `json:"-"` // e.g. "Json", "Text"
	HasValidation      bool           `json:"-"`
	HasScalar          bool           `json:"-"`
	HasInput           bool           `json:"-"`
	CustomBind         bool           `json:"-"`
	CustomValidate     bool           `json:"-"`
}

type ArgumentInfo struct {
	Name       string     `json:"name"`
	Doc        string     `json:"doc,omitempty"`
	Type       string     `json:"type"`
	GoType     string     `json:"-"`
	BaseGoType   string     `json:"-"`
	IsScalar     bool       `json:"-"`
	GoName     string     `json:"-"`
	Source     string     `json:"source"`
	Validators []MetaInfo `json:"validators,omitempty"`
	RefModel   *ModelInfo `json:"-"`
	ScalarModel string     `json:"-"`
}

type MetaInfo struct {
	Name          string       `json:"name"`
	Doc           string       `json:"doc,omitempty"`
	Stage         string       `json:"stage,omitempty"` // request, invoke, response
	Scope         string       `json:"scope,omitempty"` // global, specialized
	IsSpecialized bool         `json:"isSpecialized,omitempty"`
	MethodName    string       `json:"methodName,omitempty"` // 仅用于特化调用
	InputType     string       `json:"inputType,omitempty"`  // 仅用于特化调用
	ReturnType    string       `json:"returnType,omitempty"` // 仅用于特化调用
	Args          []ModelField `json:"args,omitempty"`
}

type RenderFuncInfo struct {
	Name     string `json:"-"` // e.g. "Json", "Text"
	MimeType string `json:"-"` // e.g. "application/json"
}

type DataContext struct {
	Package                     string                `json:"package"`
	Info                        ApiInfo               `json:"info"` // OpenAPI info
	Validators                  []MetaInfo            `json:"validators,omitempty"`
	Decorators                  []MetaInfo            `json:"-"`
	Modules                     []ModuleInfo          `json:"modules"`
	Models                      []*ModelInfo          `json:"models"`
	ModelMap                    map[string]*ModelInfo `json:"-"`
	Scalars                     map[string]*ScalarInfo `json:"scalars,omitempty"`
	Config                      *config.Config        `json:"-"`
	BodySources                 []BodySourceInfo      `json:"-"`
	ExtraImports                []string              `json:"-"`
	RenderFuncs                 []RenderFuncInfo      `json:"-"` // 收集用到的所有渲染函数
	ModuleSpecializedDecorators map[string][]MetaInfo `json:"-"` // 按模块收集特化装饰器
}

type ApiInfo struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	BaseURL     string `json:"baseUrl,omitempty"`
}

type BodySourceInfo struct {
	Name  string `json:"name"`
	Alias string `json:"alias"`
}

type ModuleRenderContext struct {
	Package      string
	Config       *config.Config
	Module       ModuleInfo
	ExtraImports []string
}

func generateTags(fieldName string, conf *config.Config) string {
	var tags []string
	for _, t := range conf.Generator.StructTags {
		val := formatTagName(fieldName, t.Case)
		tags = append(tags, fmt.Sprintf("%s:%q", t.Name, val))
	}
	if len(tags) == 0 {
		return ""
	}
	return "`" + strings.Join(tags, " ") + "`"
}

func Generate(schema *parser.Schema, targetDir string, conf *config.Config) error {
	ctx := &DataContext{
		Package:  "resolver",
		ModelMap: make(map[string]*ModelInfo),
		Scalars:  make(map[string]*ScalarInfo),
		Config:   conf,
		Info: ApiInfo{
			Title:   "Resgen Generated API",
			Version: "1.0.0",
			BaseURL: conf.Generator.BaseURL,
		},
		ModuleSpecializedDecorators: make(map[string][]MetaInfo),
	}

	if conf.Generator.Package != "" {
		ctx.Package = conf.Generator.Package
	}

	for alias := range conf.Generator.ContentTypeAliases {
		ctx.BodySources = append(ctx.BodySources, BodySourceInfo{
			Name:  capitalize(alias),
			Alias: alias,
		})
	}

	// 始终注册默认 Content-Type 对应的 Render 函数
	defaultMime := resolveMimeType(conf.Generator.DefaultContentType, conf)
	addRenderFunc(ctx, defaultMime)
	// 也始终注册 JSON（因为错误响应通常是 JSON）
	addRenderFunc(ctx, "application/json")

	validatorMap := make(map[string]MetaInfo)

	decoratorMap := make(map[string]MetaInfo)

	requiredInfo := MetaInfo{Name: "Required"}
	validatorMap["required"] = requiredInfo
	ctx.Validators = append(ctx.Validators, requiredInfo)

	var currentModule string
	for _, decl := range schema.Declarations {
		if decl.Module != nil {
			currentModule = decl.Module.Name
			if decl.Module.Doc != "" {
				ctx.Info.Description = decl.Module.Doc
			}
		}
		if decl.Decorator != nil {
			info := MetaInfo{
				Name: capitalize(decl.Decorator.Name),
				Doc:  decl.Decorator.Doc,
			}
			if stage, ok := metaGet(decl.Decorator.Meta, "stage"); ok {
				info.Stage = strings.ToLower(stage)
			} else {
				info.Stage = "request" // 默认阶段
			}

			if scope, ok := metaGet(decl.Decorator.Meta, "scope"); ok {
				info.Scope = strings.ToLower(scope)
			} else {
				// 默认作用域逻辑：invoke/response 默认为 specialized, request 默认为 global
				if info.Stage == "request" {
					info.Scope = "global"
				} else {
					info.Scope = "specialized"
				}
			}
			if info.Scope == "specialized" {
				info.IsSpecialized = true
			}
			for _, arg := range decl.Decorator.Args {
				// 强制使用非指针类型作为装饰器/验证器的参数
				argType := arg.Type
				argType.ItemNotNull = true
				info.Args = append(info.Args, ModelField{
					Name: capitalize(arg.Name),
					Type: ToGoType(argType, ctx.Config, &ctx.ExtraImports, "", ctx.ModelMap),
				})
			}
			if decl.Decorator.IsDec {
				if !info.IsSpecialized {
					ctx.Decorators = append(ctx.Decorators, info)
				}
				decoratorMap[strings.ToLower(decl.Decorator.Name)] = info
			} else {
				if strings.ToLower(decl.Decorator.Name) != "required" {
					ctx.Validators = append(ctx.Validators, info)
					validatorMap[strings.ToLower(decl.Decorator.Name)] = info
				}
			}
		}
		if decl.Scalar != nil {
			rawType := decl.Scalar.Name
			var mapping *config.ScalarConfig
			if conf != nil && conf.Scalars != nil {
				if m, ok := conf.Scalars[decl.Scalar.Name]; ok && m.Model != "" {
					mapping = &m
					rawType = m.Model
				}
			}
			resolvedGoType, importPath := parseCustomType(rawType)
			if importPath != "" {
				addImport(&ctx.ExtraImports, importPath)
			}

			baseType := "string"
			if decl.Scalar.Base != "" {
				baseType = decl.Scalar.Base
			}

			// 🌟 奇迹时刻：自动进行 AST 物理分析、契约验证与 Target 类型智能推导！
			if mapping != nil {
				res, err := AnalyzeScalar(mapping.Model, baseType)
				if err != nil {
					// ❌ 静态语义强校验报错！直接返回，优雅终止代码生成
					return fmt.Errorf("自定义标量 '%s' 物理类型校验失败！\n%v", decl.Scalar.Name, err)
				}
				if res != nil {
					// 智能写回物理目标类型
					if res.TargetType != "" {
						mapping.Target = res.TargetType
						// 动态更新 conf 中的 Scalars 映射，以让后面的 ToGoType 直接取得推导出的 target
						conf.Scalars[decl.Scalar.Name] = *mapping
					}
					// 智能自动导入目标类型的物理包
					if res.ImportPath != "" {
						addImport(&ctx.ExtraImports, res.ImportPath)
					}
				}
			}

			ctx.Scalars[decl.Scalar.Name] = &ScalarInfo{
				Name:     decl.Scalar.Name,
				Doc:      decl.Scalar.Doc,
				BaseType: baseType,
				GoType:   resolvedGoType,
			}
		}
		if decl.Model != nil {
			mName := currentModule
			if mName == "" {
				mName = "Default"
			}
			isWrapper := decl.Model.Keyword == "wrap"
			if !isWrapper {
				for _, d := range decl.Model.Directives {
					if strings.ToLower(d.Name) == "wrapper" {
						isWrapper = true
						break
					}
				}
			}
			m := &ModelInfo{
				Name:       decl.Model.Name,
				IsInput:    decl.Model.Keyword == "input",
				IsWrapper:  isWrapper,
				Module:     mName,
				Doc:        decl.Model.Doc,
				TypeParams: decl.Model.TypeParams,
			}
			ctx.Models = append(ctx.Models, m)
			ctx.ModelMap[decl.Model.Name] = m
		}
	}

	for _, decl := range schema.Declarations {
		if decl.Model != nil {
			m := ctx.ModelMap[decl.Model.Name]
			for _, field := range decl.Model.Properties {
				if field.Type.Name == "Field" {
					return fmt.Errorf("语义错误：模型属性 '%s.%s' 不能使用 'Field' 类型。'Field' 是专属于校验器形参的字段引用元类型，绝对不能作为普通属性类型！若需表达动态或任意结构数据，请选用 'Any' 类型", decl.Model.Name, field.Name)
				}
				fieldType := ToGoType(field.Type, ctx.Config, &ctx.ExtraImports, m.Name+"."+field.Name, ctx.ModelMap)
				goType := fieldType
				if m.IsWrapper {
					for _, tp := range m.TypeParams {
						if field.Type.Name == tp {
							goType = "any"
							if field.Type.IsArray {
								goType = "[]any"
							}
							break
						}
					}
				}
				m.Fields = append(m.Fields, ModelField{
					Name:         capitalize(field.Name),
					JSONName:     field.Name,
					Doc:          field.Doc,
					Type:         fieldType,
					GoType:       goType,
					IsScalar:     ctx.Scalars[field.Type.Name] != nil,
					ScalarModel: func() string {
						if s, ok := ctx.Config.Scalars[field.Type.Name]; ok && s.Model != "" {
							base, _ := parseCustomType(s.Model)
							return base
						}
						return ""
					}(),
					BaseGoType:   func() string { if s := ctx.Scalars[field.Type.Name]; s != nil { return s.BaseType } else { return goType } }(),
					OriginalType: field.Type.Name,
					Tag:          generateTags(field.Name, ctx.Config),
					Source:       "Body", // Default to Body
					RefModel:     ctx.ModelMap[field.Type.Name],
				})
				if ctx.Scalars[field.Type.Name] != nil {
					m.HasScalar = true
				}
			}
		}
	}

	// 冒泡传递 HasScalar 属性（解决多级嵌套结构体的 HasScalar 传递）
	changed := true
	for changed {
		changed = false
		for _, m := range ctx.Models {
			if m.HasScalar {
				continue
			}
			for _, f := range m.Fields {
				if f.RefModel != nil && f.RefModel.HasScalar {
					m.HasScalar = true
					changed = true
					break
				}
			}
		}
	}

	// 计算每个字段的 GoTypeDTO、IsArray、IsPointer 辅助属性
	for _, m := range ctx.Models {
		for i := range m.Fields {
			field := &m.Fields[i]
			field.IsArray = strings.Contains(field.GoType, "[]")
			field.IsPointer = strings.HasPrefix(field.GoType, "*")
			field.IsArrayElementPointer = field.IsArray && strings.HasPrefix(strings.TrimPrefix(field.GoType, "*"), "[]*")

			if field.IsScalar {
				goTypeDTO := field.BaseGoType
				if strings.HasPrefix(field.GoType, "*") && !strings.HasPrefix(goTypeDTO, "*") {
					goTypeDTO = "*" + goTypeDTO
				}
				field.GoTypeDTO = goTypeDTO
			} else if field.RefModel != nil && field.RefModel.HasScalar {
				field.GoTypeDTO = strings.ReplaceAll(field.GoType, field.RefModel.Name, field.RefModel.Name+"DTO")
			} else {
				field.GoTypeDTO = field.GoType
			}
		}
	}

	for _, decl := range schema.Declarations {
		if decl.Model != nil {
			m := ctx.ModelMap[decl.Model.Name]
			for i, field := range decl.Model.Properties {
				if field.Type.ItemNotNull {
					m.Fields[i].Validators = append(m.Fields[i].Validators, requiredInfo)
				}
				for _, d := range field.Directives {
					switch strings.ToLower(d.Name) {
					case "path", "query", "header":
						m.Fields[i].Source = capitalize(d.Name)
						continue
					case "required":
						continue
					}
					vInfo, _ := validatorMap[strings.ToLower(d.Name)]
					m.Fields[i].Validators = append(m.Fields[i].Validators, MetaInfo{
						Name: capitalize(d.Name),
						Doc:  vInfo.Doc,
						Args: extractAndPadArgs(d.Name, d.Args, validatorMap, &ctx.Validators, m, "input.", ctx.ModelMap),
					})
				}
			}
		}
	}

	currentModule = ""
	for _, decl := range schema.Declarations {
		if decl.Module != nil {
			currentModule = decl.Module.Name
		}
		if decl.Group != nil {
			modName := currentModule
			if modName == "" {
				modName = "Default"
			}

			gName := decl.Group.Name
			if gName == "" {
				gName = strings.TrimPrefix(decl.Group.Path, "/")
				if gName == "" {
					gName = "Root"
				}
			}
			group := GroupInfo{
				Path: decl.Group.Path,
				Name: capitalize(gName),
				Doc:  decl.Group.Doc,
			}
			groupErrorType := ctx.Config.Generator.DefaultWrap
			groupSuccessStatus := ctx.Config.Generator.DefaultOkStatus
			// 从组级 Meta 中读取 wrap 设置
			if v, ok := metaGet(decl.Group.Meta, "wrap"); ok {
				groupErrorType = v
			}
			if v, ok := metaGetInt(decl.Group.Meta, "state"); ok {
				groupSuccessStatus = v
			}
			// 组级普通装饰器
			for _, d := range decl.Group.Directives {
				dInfo, _ := decoratorMap[strings.ToLower(d.Name)]
				meta := MetaInfo{
					Name:  capitalize(d.Name),
					Doc:   dInfo.Doc,
					Stage: dInfo.Stage,
					Args:  extractAndPadArgs(d.Name, d.Args, decoratorMap, &ctx.Decorators, nil, "", ctx.ModelMap),
				}
				switch dInfo.Stage {
				case "invoke":
					group.InvokeDecorators = append(group.InvokeDecorators, meta)
				case "response":
					group.ResponseDecorators = append(group.ResponseDecorators, meta)
				default:
					group.RequestDecorators = append(group.RequestDecorators, meta)
				}
			}
			for _, ep := range decl.Group.Endpoints {
				fullReturnType := ToGoType(ep.ReturnType, ctx.Config, &ctx.ExtraImports, ep.Name+".Return", ctx.ModelMap)
				innerReturnType := fullReturnType

				isReturnWrapped := false
				returnTypeBase := ""
				isReturnArray := false
				if baseModel, ok := ctx.ModelMap[ep.ReturnType.Name]; ok && baseModel.IsWrapper {
					isReturnWrapped = true
					returnTypeBase = baseModel.Name
					if len(ep.ReturnType.TypeArgs) > 0 {
						innerReturnType = ToGoType(ep.ReturnType.TypeArgs[0], ctx.Config, &ctx.ExtraImports, ep.Name+".InnerReturn", ctx.ModelMap)
						isReturnArray = ep.ReturnType.TypeArgs[0].IsArray
					}
				} else {
					isReturnArray = ep.ReturnType.IsArray
				}

				// 从接口级 ResponseMeta 读取 wrap/state；接口优先于组级
				errorType := groupErrorType
				successStatus := groupSuccessStatus
				if v, ok := metaGet(ep.ResponseMeta, "wrap"); ok {
					errorType = v
				}
				if v, ok := metaGetInt(ep.ResponseMeta, "state"); ok {
					successStatus = v
				}

				// 接口级装饰器（组级已统一继承）
				var filteredDirectives []parser.DirectiveUsage
				for _, d := range decl.Group.Directives {
					filteredDirectives = append(filteredDirectives, d)
				}
				for _, d := range ep.Directives {
					filteredDirectives = append(filteredDirectives, d)
				}

				isErrorWrapped := false
				errorTypeBase := ""
				if baseModel, ok := ctx.ModelMap[errorType]; ok && baseModel.IsWrapper {
					isErrorWrapped = true
					errorTypeBase = baseModel.Name
				}

				method := MethodInfo{
					Name:            ep.Name,
					Doc:             ep.Doc,
					Method:          strings.ToUpper(ep.Method),
					Path:            ep.Path,
					FullPath:        decl.Group.Path + ep.Path,
					ReturnType:      fullReturnType,
					ReturnTypeDSL:   formatTypeRef(ep.ReturnType),
					InnerReturnType: innerReturnType,
					IsReturnWrapped: isReturnWrapped,
					ReturnTypeBase:  returnTypeBase,
					InnerReturnModel: func() *ModelInfo {
						if isReturnWrapped && len(ep.ReturnType.TypeArgs) > 0 {
							return ctx.ModelMap[ep.ReturnType.TypeArgs[0].Name]
						}
						return nil
					}(),
					ErrorType:       errorType,
					IsErrorWrapped:  isErrorWrapped,
					ErrorTypeBase:   errorTypeBase,
					IsReturnArray:   isReturnArray,
					SuccessStatus:   successStatus,
					IsArgsWrapped:   true,
				}

				// 响应 MIME 类型：优先接口 ResponseMeta[ctype]，无则 fallback 到 default_content_type
				if v, ok := metaGet(ep.ResponseMeta, "ctype"); ok {
					method.ResponseMimeType = resolveMimeType(v, ctx.Config)
				} else {
					method.ResponseMimeType = resolveMimeType(ctx.Config.Generator.DefaultContentType, ctx.Config)
				}
				method.ResponseRenderFunc = addRenderFunc(ctx, method.ResponseMimeType)

				// 错误响应 MIME 类型：ResponseMeta[etype]，无则 fallback 到 default_content_type
				if v, ok := metaGet(ep.ResponseMeta, "etype"); ok {
					method.ErrorMimeType = resolveMimeType(v, ctx.Config)
				} else {
					method.ErrorMimeType = resolveMimeType(ctx.Config.Generator.DefaultContentType, ctx.Config)
				}
				method.ErrorRenderFunc = addRenderFunc(ctx, method.ErrorMimeType)

				var args []ArgumentInfo
				for _, arg := range ep.Args {
					goType := ToGoType(arg.Type, ctx.Config, &ctx.ExtraImports, ep.Name+"."+arg.Name, ctx.ModelMap)
					argInfo := ArgumentInfo{
						Name: arg.Name, GoName: capitalize(arg.Name), Type: formatTypeRef(arg.Type), GoType: goType, Source: "Body", Doc: arg.Doc,
						IsScalar: ctx.Scalars[arg.Type.Name] != nil,
						ScalarModel: func() string {
							if s, ok := ctx.Config.Scalars[arg.Type.Name]; ok && s.Model != "" {
								base, _ := parseCustomType(s.Model)
								return base
							}
							return ""
						}(),
						BaseGoType: func() string { if s := ctx.Scalars[arg.Type.Name]; s != nil { return s.BaseType } else { return goType } }(),
					}
					if method.Method == "GET" {
						argInfo.Source = "Query"
						if ref, ok := ctx.ModelMap[arg.Type.Name]; ok {
							for _, f := range ref.Fields {
								if fRef, ok := ctx.ModelMap[f.OriginalType]; ok && len(fRef.Fields) > 0 {
									fmt.Printf("❌ 错误: GET 方法 [%s] 的参数 [%s] 包含嵌套结构体 [%s]。GET 参数必须是扁平结构。\n", ep.Name, arg.Name, f.Name)
									os.Exit(1)
								}
							}
						}
					}
					if arg.Type.Name == "File" {
						argInfo.Source = "Form"
					}
					if arg.Type.ItemNotNull {
						argInfo.Validators = append(argInfo.Validators, requiredInfo)
					}

					for _, d := range arg.Directives {
						switch strings.ToLower(d.Name) {
						case "path", "query", "header", "form":
							argInfo.Source = capitalize(d.Name)
						case "required":
							continue
						default:
							vInfo, _ := validatorMap[strings.ToLower(d.Name)]
							argInfo.Validators = append(argInfo.Validators, MetaInfo{
								Name: capitalize(d.Name),
								Doc:  vInfo.Doc,
								Args: extractAndPadArgs(d.Name, d.Args, validatorMap, &ctx.Validators, nil, "input.", ctx.ModelMap),
							})
						}
					}
					if ref, ok := ctx.ModelMap[arg.Type.Name]; ok && ref.IsInput {
						argInfo.RefModel = ref
					}
					args = append(args, argInfo)
				}

				if len(args) == 1 && args[0].RefModel != nil && (args[0].Source == "Body" || (ep.Method == "GET" && !args[0].IsScalar)) {
					method.InputName = args[0].RefModel.Name
					method.IsArgsWrapped = false
					method.Args = args
				} else if len(args) > 1 {
					inputModelName := ep.Name + "Args"
					inputModel := &ModelInfo{Name: inputModelName, IsInput: true, Module: modName}
					for _, arg := range args {
						inputModel.Fields = append(inputModel.Fields, ModelField{
							Name:         arg.GoName,
							JSONName:     arg.Name,
							Doc:          arg.Doc,
							Type:         arg.Type,
							GoType:       arg.GoType,
							IsScalar:     arg.IsScalar,
							ScalarModel:  arg.ScalarModel,
							BaseGoType:   arg.BaseGoType,
							OriginalType: arg.Type,
							Tag:          generateTags(arg.Name, ctx.Config),
						})
						if arg.IsScalar {
							inputModel.HasScalar = true
						}
					}
					method.InputName = inputModelName
					method.Args = args
					method.IsArgsWrapped = true
					ctx.Models = append(ctx.Models, inputModel)
					ctx.ModelMap[inputModelName] = inputModel
				} else if len(args) == 1 {
					method.InputName = ""
					method.Args = args
					method.IsArgsWrapped = false
				} else {
					method.InputName = ""
					method.Args = nil
					method.IsArgsWrapped = false
				}

				// 检测是否有输入参数（排除空结构体）
				method.HasInput = false
				if inputModel, ok := ctx.ModelMap[method.InputName]; ok {
					method.InputModel = inputModel
					if len(inputModel.Fields) > 0 {
						method.HasInput = true
					}
				}
				if returnModel, ok := ctx.ModelMap[method.InnerReturnType]; ok {
					method.ReturnModel = returnModel
				} else {
					// InnerReturnType is the go type name, maybe pointer. We should use ep.ReturnType.Name for lookup
					if returnModel, ok := ctx.ModelMap[ep.ReturnType.Name]; ok {
						method.ReturnModel = returnModel
					} else if len(ep.ReturnType.TypeArgs) > 0 {
						if returnModel, ok := ctx.ModelMap[ep.ReturnType.TypeArgs[0].Name]; ok {
							method.ReturnModel = returnModel
						}
					}
				}

				for _, d := range filteredDirectives {
					nameLower := strings.ToLower(d.Name)
					if nameLower == "custombind" {
						method.CustomBind = true
						continue
					}
					if nameLower == "customvalidate" {
						method.CustomValidate = true
						continue
					}

					if nameLower == "consumes" && len(d.Args) > 0 {
						if val := d.Args[0].Value.String; val != nil {
							method.ContentType = *val
						}
						continue
					}
					dInfo, ok := decoratorMap[nameLower]
					if !ok {
						continue
					}

					meta := MetaInfo{
						Name:          capitalize(d.Name),
						Doc:           dInfo.Doc,
						Stage:         dInfo.Stage,
						IsSpecialized: dInfo.IsSpecialized,
						Args:          extractAndPadArgs(d.Name, d.Args, decoratorMap, &ctx.Decorators, nil, "", ctx.ModelMap),
					}

					// 检查使用处是否覆盖了 scope
					if scope, ok := metaGet(d.Meta, "scope"); ok {
						if strings.ToLower(scope) == "specialized" {
							meta.IsSpecialized = true
						}
					}

					if meta.IsSpecialized {
						meta.MethodName = ep.Name
						meta.InputType = method.InputName
						meta.ReturnType = method.InnerReturnType

						// 按模块收集特化装饰器
						ctx.ModuleSpecializedDecorators[modName] = append(ctx.ModuleSpecializedDecorators[modName], meta)
					}

					switch dInfo.Stage {
					case "invoke":
						method.InvokeDecorators = append(method.InvokeDecorators, meta)
					case "response":
						method.ResponseDecorators = append(method.ResponseDecorators, meta)
					default:
						method.RequestDecorators = append(method.RequestDecorators, meta)
					}
				}

				// 请求 Content-Type：RequestMeta[ctype] > 自动推断（含文件检测）> 默认
				sourceSymbol := ""
				if v, ok := metaGet(ep.RequestMeta, "ctype"); ok {
					sourceSymbol = v
				}
				if sourceSymbol == "" || sourceSymbol == "json" {
					m := method.Method
					if m == "POST" || m == "PUT" || m == "PATCH" {
						hasFile := false
						for _, arg := range method.Args {
							if arg.Type == "*multipart.FileHeader" || (arg.RefModel != nil && hasFileInModel(arg.RefModel, ctx.ModelMap)) {
								hasFile = true
								break
							}
						}
						if hasFile {
							sourceSymbol = "multipart"
						}
					}
				}
				if sourceSymbol == "" {
					sourceSymbol = conf.Generator.DefaultContentType
				}

				switch strings.ToLower(sourceSymbol) {
				case "json", "application/json":
					method.ContentType = "SourceJson"
					method.MimeType = "application/json"
				case "form", "application/x-www-form-urlencoded":
					method.ContentType = "SourceForm"
					method.MimeType = "application/x-www-form-urlencoded"
				case "multipart", "multipart/form-data":
					method.ContentType = "SourceMultipart"
					method.MimeType = "multipart/form-data"
				default:
					found := false
					for alias, mime := range conf.Generator.ContentTypeAliases {
						if alias == sourceSymbol {
							method.ContentType = "Source" + capitalize(alias)
							method.MimeType = mime
							found = true
							break
						}
					}
					if !found {
						method.ContentType = fmt.Sprintf("engine.BodySource(%q)", sourceSymbol)
						method.MimeType = sourceSymbol
					}
				}
				// 检查是否有任何校验逻辑，决定是否生成校验区块和校验方法调用
				hasValidation := false
				for _, arg := range method.Args {
					if len(arg.Validators) > 0 {
						hasValidation = true
						break
					}
					if arg.RefModel != nil {
						for _, f := range arg.RefModel.Fields {
							if len(f.Validators) > 0 {
								hasValidation = true
								break
							}
						}
					}
					if hasValidation {
						break
					}
				}
				method.HasValidation = hasValidation

				method.HasScalar = method.HasScalar || (isReturnWrapped && method.InnerReturnModel != nil && method.InnerReturnModel.HasScalar)
				method.HasScalar = method.HasScalar || (method.ReturnModel != nil && method.ReturnModel.HasScalar)

				// 检查参数和输入模型中是否有标量
				if method.InputModel != nil && method.InputModel.HasScalar {
					method.HasScalar = true
				}
				for _, arg := range method.Args {
					if arg.IsScalar {
						method.HasScalar = true
					}
				}

				group.Endpoints = append(group.Endpoints, method)
			}

			found := false
			for i := range ctx.Modules {
				if ctx.Modules[i].Name == modName {
					ctx.Modules[i].Groups = append(ctx.Modules[i].Groups, group)
					found = true
					break
				}
			}
			if !found {
				ctx.Modules = append(ctx.Modules, ModuleInfo{Name: modName, Groups: []GroupInfo{group}})
			}
		}
	}

	// 将收集到的特化装饰器分配到各模块
	for i := range ctx.Modules {
		if decs, ok := ctx.ModuleSpecializedDecorators[ctx.Modules[i].Name]; ok {
			// 去重
			seen := make(map[string]bool)
			for _, d := range decs {
				key := fmt.Sprintf("%s_%s_%s", d.Stage, d.Name, d.MethodName)
				if !seen[key] {
					ctx.Modules[i].SpecializedDecorators = append(ctx.Modules[i].SpecializedDecorators, d)
					seen[key] = true
				}
			}
		}
	}

	return renderAll(ctx, targetDir)
}

func extractAndPadArgs(name string, dArgs []parser.DirectiveArg, defMap map[string]MetaInfo, defSlice *[]MetaInfo, modelContext *ModelInfo, prefix string, fullModelMap map[string]*ModelInfo) []ModelField {
	def, ok := defMap[strings.ToLower(name)]
	if !ok {
		var res []ModelField
		for _, a := range dArgs {
			paramType := "string"
			res = append(res, ModelField{Name: capitalize(a.Name), GoValue: formatValue(a.Value, "", modelContext, prefix, fullModelMap), Type: paramType})
		}
		newMeta := MetaInfo{Name: capitalize(name), Args: res}
		defMap[strings.ToLower(name)] = newMeta
		if defSlice != nil {
			*defSlice = append(*defSlice, newMeta)
		}
		return res
	}

	var res []ModelField
	for _, defArg := range def.Args {
		found := false
		for _, a := range dArgs {
			if strings.EqualFold(a.Name, defArg.Name) || a.Name == "" {
				res = append(res, ModelField{Name: defArg.Name, GoValue: formatValue(a.Value, defArg.Type, modelContext, prefix, fullModelMap)})
				found = true
				break
			}
		}
		if !found {
			val := "nil"
			if !strings.HasPrefix(defArg.Type, "*") {
				switch defArg.Type {
				case "string":
					val = "\"\""
				case "int":
					val = "0"
				case "bool":
					val = "false"
				case "float64":
					val = "0.0"
				}
			}
			res = append(res, ModelField{Name: defArg.Name, GoValue: val})
		}
	}
	return res
}

func formatValue(val parser.Value, targetType string, modelContext *ModelInfo, prefix string, fullModelMap map[string]*ModelInfo) string {
	var raw string
	if val.String != nil {
		raw = *val.String
	} else if val.Ident != nil {
		raw = *val.Ident
	} else if val.Int != nil {
		return fmt.Sprintf("%d", *val.Int)
	} else if val.List != nil {
		var items []string
		for _, item := range val.List {
			items = append(items, formatValue(*item, "", modelContext, prefix, fullModelMap))
		}
		res := "[]string{" + strings.Join(items, ", ") + "}"
		if strings.HasPrefix(targetType, "*[]") {
			return "&" + res
		}
		return res
	} else {
		return "nil"
	}

	if strings.Contains(targetType, "any") && modelContext != nil && raw != "" {
		if modelContext != nil {
			parts := strings.Split(raw, ".")
			currentModel := modelContext
			goPath := prefix

			valid := true
			for i, part := range parts {
				foundField := false
				for _, f := range currentModel.Fields {
					if strings.EqualFold(f.Name, part) {
						goPath += f.Name
						if i < len(parts)-1 {
							goPath += "."
							if nextModel, ok := fullModelMap[strings.TrimLeft(f.OriginalType, "*[]")]; ok {
								currentModel = nextModel
								foundField = true
								break
							} else {
								valid = false
								break
							}
						}
						foundField = true
						break
					}
				}
				if !foundField {
					valid = false
					break
				}
			}
			if valid {
				return goPath
			}
		}
	}

	if val.String != nil {
		return "\"" + *val.String + "\""
	}
	return raw
}

func renderAll(ctx *DataContext, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	// Step 0: Generate API Doc JSON & HTML if requested
	if ctx.Config.Generator.EnableApiDocs {
		if err := generateApiDocs(ctx, targetDir); err != nil {
			return err
		}
	}

	for _, m := range ctx.Models {
		if m.IsWrapper {
			continue
		}
		for i := range ctx.Modules {
			if ctx.Modules[i].Name == m.Module {
				ctx.Modules[i].Models = append(ctx.Modules[i].Models, m)
				break
			}
		}
	}

	funcMap := template.FuncMap{
		"ToLower":    strings.ToLower,
		"Title":      capitalize,
		"capitalize": capitalize,
		"HasPrefix":  strings.HasPrefix,
		"Replace":    strings.ReplaceAll,
		"ParseParam": func(target, val, goType string, errHandler ...string) string {
			isPointer := strings.HasPrefix(goType, "*")
			baseType := goType
			if isPointer {
				baseType = goType[1:]
			}

			// 默认使用 "return err"
			failStmt := "return err"
			if len(errHandler) > 0 && errHandler[0] != "" {
				failStmt = errHandler[0]
			}

			var parseFunc string
			var typeCast string
			switch baseType {
			case "string":
				if isPointer {
					return fmt.Sprintf("%s = &%s", target, val)
				}
				return fmt.Sprintf("%s = %s", target, val)
			case "int":
				parseFunc = "IntFromParam"
			case "int32":
				parseFunc = "Int64FromParam"
				typeCast = "int32"
			case "int64":
				parseFunc = "Int64FromParam"
			case "uint":
				parseFunc = "Int64FromParam"
				typeCast = "uint"
			case "uint64":
				parseFunc = "Int64FromParam"
				typeCast = "uint64"
			case "float64":
				parseFunc = "FloatFromParam"
			case "bool":
				parseFunc = "BoolFromParam"
			default:
				// Fallback to error for unknown types
				return fmt.Sprintf("// 不支持的基础数据类型: %s\n%s", baseType, strings.ReplaceAll(failStmt, "err", fmt.Sprintf("fmt.Errorf(\"未支持的参数物理类型: %%s\", %q)", baseType)))
			}

			if isPointer {
				if typeCast != "" {
					return fmt.Sprintf("if v, err := %s(%s); err == nil { v2 := %s(v); %s = &v2 } else { %s }", parseFunc, val, typeCast, target, failStmt)
				}
				return fmt.Sprintf("if v, err := %s(%s); err == nil { v2 := v; %s = &v2 } else { %s }", parseFunc, val, target, failStmt)
			}
			if typeCast != "" {
				return fmt.Sprintf("if v, err := %s(%s); err == nil { %s = %s(v) } else { %s }", parseFunc, val, target, typeCast, failStmt)
			}
			return fmt.Sprintf("if v, err := %s(%s); err == nil { %s = v } else { %s }", parseFunc, val, target, failStmt)
		},
	}

	engineT, err := template.New("engine").Funcs(funcMap).Parse(engineTmpl)
	if err != nil {
		return fmt.Errorf("解析引擎核心模板失败: %v", err)
	}
	var buf bytes.Buffer
	if err := engineT.Execute(&buf, ctx); err != nil {
		return fmt.Errorf("渲染引擎核心模板失败: %v", err)
	}
	formatted, err := imports.Process(filepath.Join(targetDir, "engine.gen.go"), buf.Bytes(), nil)
	if err != nil {
		formatted = buf.Bytes()
	}
	os.WriteFile(filepath.Join(targetDir, "engine.gen.go"), formatted, 0644)

	modT, err := template.New("module").Funcs(funcMap).Parse(moduleTmpl)
	if err != nil {
		return fmt.Errorf("解析模块生成模板失败: %v", err)
	}
	for _, mod := range ctx.Modules {
		modCtx := &ModuleRenderContext{
			Package:      ctx.Package,
			Config:       ctx.Config,
			Module:       mod,
			ExtraImports: append(ctx.ExtraImports, "strconv"),
		}
		var buf bytes.Buffer
		if err := modT.Execute(&buf, modCtx); err != nil {
			return fmt.Errorf("渲染模块 '%s' 模板失败: %v", mod.Name, err)
		}
		formatted, err := imports.Process(filepath.Join(targetDir, strings.ToLower(mod.Name)+".gen.go"), buf.Bytes(), nil)
		if err != nil {
			formatted = buf.Bytes()
		}
		os.WriteFile(filepath.Join(targetDir, strings.ToLower(mod.Name)+".gen.go"), formatted, 0644)
	}

	os.Remove(filepath.Join(targetDir, "models.gen.go"))
	os.Remove(filepath.Join(targetDir, "binders.gen.go"))
	os.Remove(filepath.Join(targetDir, "resolvers.gen.go"))
	os.Remove(filepath.Join(targetDir, "decorators.gen.go"))
	os.Remove(filepath.Join(targetDir, "validators.gen.go"))
	os.Remove(filepath.Join(targetDir, "sources.gen.go"))

	return nil
}

func hasFileInModel(m *ModelInfo, modelMap map[string]*ModelInfo) bool {
	for _, f := range m.Fields {
		if f.Type == "*multipart.FileHeader" || f.Type == "[]*multipart.FileHeader" {
			return true
		}
		if ref, ok := modelMap[f.OriginalType]; ok {
			if hasFileInModel(ref, modelMap) {
				return true
			}
		}
	}
	return false
}

func resolveMimeType(symbol string, conf *config.Config) string {
	symbol = strings.ToLower(symbol)
	switch symbol {
	case "json", "application/json":
		return "application/json"
	case "form", "application/x-www-form-urlencoded":
		return "application/x-www-form-urlencoded"
	case "multipart", "multipart/form-data":
		return "multipart/form-data"
	case "xml", "application/xml":
		return "application/xml"
	case "text", "text/plain":
		return "text/plain"
	case "html", "text/html":
		return "text/html"
	}
	for alias, mime := range conf.Generator.ContentTypeAliases {
		if strings.ToLower(alias) == symbol {
			return mime
		}
	}
	return symbol
}

// mimeToFuncName 将 MIME 类型转换为 CamelCase 函数名后缀，如 "application/json" -> "Json"
func mimeToFuncName(mime string) string {
	mime = strings.ToLower(mime)
	switch mime {
	case "application/json":
		return "Json"
	case "text/plain":
		return "Text"
	case "text/html":
		return "Html"
	case "application/xml":
		return "Xml"
	case "application/x-www-form-urlencoded":
		return "Form"
	case "multipart/form-data":
		return "Multipart"
	}
	// 对于自定义 MIME，取 "/" 后的部分并 PascalCase
	parts := strings.Split(mime, "/")
	last := parts[len(parts)-1]
	last = strings.ReplaceAll(last, "-", "_")
	parts2 := strings.Split(last, "_")
	var sb strings.Builder
	for _, p := range parts2 {
		if p != "" {
			r := []rune(p)
			r[0] = unicode.ToUpper(r[0])
			sb.WriteString(string(r))
		}
	}
	return sb.String()
}

// addRenderFunc 幂等地往 DataContext 添加一个 RenderFuncInfo
func addRenderFunc(ctx *DataContext, mime string) string {
	name := mimeToFuncName(mime)
	for _, rf := range ctx.RenderFuncs {
		if rf.MimeType == mime {
			return name
		}
	}
	ctx.RenderFuncs = append(ctx.RenderFuncs, RenderFuncInfo{Name: name, MimeType: mime})
	return name
}

// generateApiDocs 处理 API 文档的 JSON 和 HTML 生成逻辑
func generateApiDocs(ctx *DataContext, targetDir string) error {
	docsDir := filepath.Join(targetDir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		return fmt.Errorf("创建文档输出目录失败: %v", err)
	}

	// 创建一个用于文档生成的上下文副本，以应用 DocCase 转换（保持数据源原始性，仅改变显示名）
	docCtx := *ctx
	docCtx.Models = make([]*ModelInfo, len(ctx.Models))
	for i, m := range ctx.Models {
		mCopy := *m
		mCopy.Fields = make([]ModelField, len(m.Fields))
		for j, f := range m.Fields {
			fCopy := f
			fCopy.Name = formatTagName(f.Name, ctx.Config.Generator.DocCase)
			mCopy.Fields[j] = fCopy
		}
		docCtx.Models[i] = &mCopy
	}

	docCtx.Modules = make([]ModuleInfo, len(ctx.Modules))
	for i, mod := range ctx.Modules {
		modCopy := mod
		modCopy.Groups = make([]GroupInfo, len(mod.Groups))
		for j, group := range mod.Groups {
			groupCopy := group
			groupCopy.Endpoints = make([]MethodInfo, len(group.Endpoints))
			for k, method := range group.Endpoints {
				methodCopy := method
				methodCopy.Args = make([]ArgumentInfo, len(method.Args))
				for l, arg := range method.Args {
					argCopy := arg
					argCopy.Name = formatTagName(arg.Name, ctx.Config.Generator.DocCase)
					methodCopy.Args[l] = argCopy
				}
				groupCopy.Endpoints[k] = methodCopy
			}
			modCopy.Groups[j] = groupCopy
		}
		docCtx.Modules[i] = modCopy
	}

	// 1. 生成 api.json
	docData, err := json.MarshalIndent(docCtx, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 API 文档 JSON 失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "api.json"), docData, 0644); err != nil {
		return fmt.Errorf("写入 api.json 文件失败: %v", err)
	}

	// 2. 生成 api.html
	t, err := template.New("apihtml").Parse(apiHtmlTmpl)
	if err != nil {
		return fmt.Errorf("解析交互式文档 api.html 模板失败: %v", err)
	}
	var htmlBuf bytes.Buffer
	if err := t.Execute(&htmlBuf, struct{ ApiJson string }{string(docData)}); err != nil {
		return fmt.Errorf("渲染交互式文档 api.html 失败: %v", err)
	}
	return os.WriteFile(filepath.Join(docsDir, "api.html"), htmlBuf.Bytes(), 0644)
}
