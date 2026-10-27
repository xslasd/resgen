package generator

import (
	"fmt"
	"path/filepath"
	"strings"
)

type ResSealInfo struct {
	InSealChain    []string `json:"inSealChain,omitempty"`
	InSealGoType   string   `json:"inSealGoType,omitempty"`
	InnerInputName string   `json:"innerInputName,omitempty"`
	OutSealChain   []string `json:"outSealChain,omitempty"`
	OutSealGoType  string   `json:"outSealGoType,omitempty"`
	OutPayloadDSL  string   `json:"outPayloadDSL,omitempty"`
}

type OpenAPIInfo struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// OpenAPIDocument represents the OpenAPI 3.0.3 root document
type OpenAPIDocument struct {
	OpenAPI    string                            `json:"openapi"`
	Info       OpenAPIInfo                       `json:"info"`
	Servers    []OpenAPIServer                   `json:"servers,omitempty"`
	Tags       []OpenAPITag                      `json:"tags,omitempty"`
	Paths      map[string]map[string]OpenAPIOperation `json:"paths"`
	Components OpenAPIComponents                 `json:"components"`

	// Specification Extensions for resgen compatibility and tools (prefixed with x-resgen-)
	Version    string                            `json:"x-resgen-version,omitempty"`
	Package    string                            `json:"x-resgen-package,omitempty"`
	Validators []MetaInfo                        `json:"x-resgen-validators,omitempty"`
	Scalars    map[string]*ScalarInfo            `json:"x-resgen-scalars,omitempty"`
}

type OpenAPIServer struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

type OpenAPITag struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type OpenAPIOperation struct {
	Tags          []string                    `json:"tags,omitempty"`
	Summary       string                      `json:"summary,omitempty"`
	Description   string                      `json:"description,omitempty"`
	OperationID   string                      `json:"operationId,omitempty"`
	Parameters    []OpenAPIParameter          `json:"parameters,omitempty"`
	RequestBody   *OpenAPIRequestBody         `json:"requestBody,omitempty"`
	Responses     map[string]OpenAPIResponse  `json:"responses"`
	
	// Resgen extensions
	ResFile             string                      `json:"x-res-file,omitempty"`
	ResGroup            string                      `json:"x-res-group,omitempty"`
	ResModule           string                      `json:"x-res-module,omitempty"`
	Permission          string                      `json:"x-res-permission,omitempty"`
	ReturnType          string                      `json:"x-res-return-type,omitempty"`
	ReturnTypeDSL       string                      `json:"x-res-return-type-dsl,omitempty"`
	ResSeal             *ResSealInfo                `json:"x-res-seal,omitempty"`
	ResInputName        string                      `json:"x-res-input-name,omitempty"`
	ContentType         string                      `json:"x-res-content-type,omitempty"`
	ResponseContentType string                      `json:"x-res-response-content-type,omitempty"`
	ErrorContentType    string                      `json:"x-res-error-content-type,omitempty"`
	ErrorType           string                      `json:"x-res-error-type,omitempty"`
	IsErrorWrapped      bool                        `json:"x-res-error-wrapped,omitempty"`
}

type OpenAPIParameter struct {
	Name        string         `json:"name"`
	In          string         `json:"in"` // "path", "query", "header", "cookie"
	Description string         `json:"description,omitempty"`
	Required    bool           `json:"required"`
	Schema      *OpenAPISchema `json:"schema,omitempty"`
}

type OpenAPIRequestBody struct {
	Description string                     `json:"description,omitempty"`
	Required    bool                       `json:"required"`
	Content     map[string]OpenAPIMediaType `json:"content"`
}

type OpenAPIMediaType struct {
	Schema *OpenAPISchema `json:"schema,omitempty"`
}

type OpenAPIResponse struct {
	Description string                     `json:"description"`
	Content     map[string]OpenAPIMediaType `json:"content,omitempty"`
}

type OpenAPIComponents struct {
	Schemas map[string]*OpenAPISchema `json:"schemas,omitempty"`
}

type OpenAPISchema struct {
	Type                 string                    `json:"type,omitempty"`
	Format               string                    `json:"format,omitempty"`
	Title                string                    `json:"title,omitempty"`
	Description          string                    `json:"description,omitempty"`
	Required             []string                  `json:"required,omitempty"`
	Properties           map[string]*OpenAPISchema `json:"properties,omitempty"`
	Items                *OpenAPISchema            `json:"items,omitempty"`
	Enum                 []any                     `json:"enum,omitempty"`
	Ref                  string                    `json:"$ref,omitempty"`
	AdditionalProperties any                       `json:"additionalProperties,omitempty"`
	
	// Resgen extensions
	ResFile              string                    `json:"x-res-file,omitempty"`
	ResEnumCases         []EnumCaseInfo            `json:"x-res-enum-cases,omitempty"`
	ResPayload           string                    `json:"x-res-payload,omitempty"`
}

// ConvertToOpenAPI3 converts the internal DataContext to OpenAPI 3.0.3 document
func ConvertToOpenAPI3(ctx *DataContext) *OpenAPIDocument {
	doc := &OpenAPIDocument{
		OpenAPI: "3.0.3",
		Info: OpenAPIInfo{
			Title:       ctx.Info.Title,
			Version:     ctx.Info.Version,
			Description: ctx.Info.Description,
		},
		Paths: make(map[string]map[string]OpenAPIOperation),
		Components: OpenAPIComponents{
			Schemas: make(map[string]*OpenAPISchema),
		},
		Version:    ctx.Version,
		Package:    ctx.Package,
		Validators: ctx.Validators,
		Scalars:    ctx.Scalars,
	}

	if ctx.Info.BaseURL != "" {
		doc.Servers = []OpenAPIServer{
			{URL: ctx.Info.BaseURL, Description: "API Server"},
		}
	}

	// 1. Build Schemas
	modelMap := make(map[string]*ModelInfo)
	for _, m := range ctx.Models {
		modelMap[m.Name] = m
	}

	for _, m := range ctx.Models {
		schema := &OpenAPISchema{
			Type:        "object",
			Title:       m.Name,
			Description: m.Doc,
			Properties:  make(map[string]*OpenAPISchema),
			ResFile:     m.SourceFile,
		}

		var reqList []string
		for _, f := range m.Fields {
			propSchema := buildTypeSchema(f.Type, ctx.Scalars, modelMap)
			if f.IsPayload {
				propSchema.ResPayload = f.PayloadArg
				codec := f.PayloadCodec
				if codec == "" {
					codec = "json"
				}
				desc := fmt.Sprintf("【载荷解封字段】物理传输格式为字符串，内部补充明文结构契约: %s (Codec: %s)", f.PayloadArg, codec)
				if f.Doc != "" {
					propSchema.Description = desc + "\n" + f.Doc
				} else {
					propSchema.Description = desc
				}
			} else if f.Doc != "" {
				propSchema.Description = f.Doc
			}
			name := f.Name
			if name == "" {
				name = f.JSONName
			}
			schema.Properties[name] = propSchema

			for _, v := range f.Validators {
				if strings.EqualFold(v.Name, "required") {
					reqList = append(reqList, name)
					break
				}
			}
		}
		if len(reqList) > 0 {
			schema.Required = reqList
		}
		doc.Components.Schemas[m.Name] = schema
	}

	// Enums to Schemas
	for _, e := range ctx.OrderedEnums {
		eSchema := &OpenAPISchema{
			Type:        "string",
			Title:       e.Name,
			Description: e.Doc,
			ResFile:     e.SourceFile,
		}
		if strings.EqualFold(e.BaseType, "int") || strings.EqualFold(e.BaseType, "integer") {
			eSchema.Type = "integer"
		}
		var enumVals []any
		var enumCases []EnumCaseInfo
		for _, c := range e.Cases {
			enumVals = append(enumVals, c.Value)
			enumCases = append(enumCases, EnumCaseInfo{
				Name:     c.Name,
				Value:    c.Value,
				Doc:      c.Doc,
				IsString: c.IsString,
			})
		}
		eSchema.Enum = enumVals
		eSchema.ResEnumCases = enumCases
		doc.Components.Schemas[e.Name] = eSchema
	}

	// 2. Build Paths and Operations
	tagSet := make(map[string]bool)
	for _, mod := range ctx.Modules {
		if !tagSet[mod.Name] {
			tagSet[mod.Name] = true
			doc.Tags = append(doc.Tags, OpenAPITag{
				Name:        mod.Name,
				Description: mod.Doc,
			})
		}

		for _, grp := range mod.Groups {
			for _, ep := range grp.Endpoints {
				openApiPath := convertToOpenAPIPath(ep.FullPath)
				if doc.Paths[openApiPath] == nil {
					doc.Paths[openApiPath] = make(map[string]OpenAPIOperation)
				}

				methodLower := strings.ToLower(ep.Method)
				op := OpenAPIOperation{
					Tags:                []string{mod.Name},
					Summary:             ep.Name,
					Description:         ep.Doc,
					OperationID:         ep.Name,
					ResFile:             ep.SourceFile,
					ResGroup:            grp.Path,
					ResModule:           mod.Name,
					Permission:          ep.Permission,
					ReturnType:          ep.ReturnType,
					ReturnTypeDSL:       ep.ReturnTypeDSL,
					ResInputName:        ep.InputName,
					ContentType:         ep.RequestContentType,
					ResponseContentType: ep.ResponseMimeType,
					ErrorContentType:    ep.ErrorMimeType,
					ErrorType:           ep.ErrorTypeBase,
					IsErrorWrapped:      ep.IsErrorWrapped,
					Responses:           make(map[string]OpenAPIResponse),
				}
				if len(ep.InSealChain) > 0 || len(ep.OutSealChain) > 0 {
					op.ResSeal = &ResSealInfo{
						InSealChain:    ep.InSealChain,
						InSealGoType:   ep.InSealGoType,
						InnerInputName: ep.InnerInputName,
						OutSealChain:   ep.OutSealChain,
						OutSealGoType:  ep.OutSealGoType,
						OutPayloadDSL:  ep.OutPayloadDSL,
					}
				}

				// Build Parameters
				isSealedInput := len(ep.InSealChain) > 0 && ep.InEnvelopeModel != nil

				for _, arg := range ep.Args {
					inType := strings.ToLower(arg.Source)
					if inType == "body" || inType == "form" {
						continue // Handled in requestBody
					}
					// 在信封加封场景下，未显式声明传输位置(如 @header/@path/@query)的参数为 Body 载荷，已被 seal 信封物理接管
					if isSealedInput && inType == "" {
						continue
					}
					// 在有 RequestBody 的非 GET 接口中，未显式声明位置的参数属于 RequestBody 字段，不应错误列入 Query 参数
					if inType == "" {
						mLower := strings.ToLower(ep.Method)
						if (mLower == "post" || mLower == "put" || mLower == "patch" || mLower == "delete") && ep.InputName != "" {
							continue
						}
						inType = "query"
					}
					isReq := inType == "path"
					for _, v := range arg.Validators {
						if strings.EqualFold(v.Name, "required") {
							isReq = true
							break
						}
					}
					param := OpenAPIParameter{
						Name:        arg.Name,
						In:          inType,
						Description: arg.Doc,
						Required:    isReq,
						Schema:      buildTypeSchema(arg.Type, ctx.Scalars, modelMap),
					}
					op.Parameters = append(op.Parameters, param)
				}

				// If inputName is specified and not mapped as individual query/path params
				reqMime := ep.MimeType
				if reqMime == "" {
					reqMime = "application/json"
				}

				if len(ep.InSealChain) > 0 && ep.InEnvelopeModel != nil {
					// 物理传输入参使用信封模型定义
					op.RequestBody = &OpenAPIRequestBody{
						Description: fmt.Sprintf("物理请求体：使用信封 %s 封装（解封后明文结构: %s）", ep.InEnvelopeModel.Name, ep.InnerInputName),
						Required:    true,
						Content: map[string]OpenAPIMediaType{
							reqMime: {
								Schema: &OpenAPISchema{
									Ref: "#/components/schemas/" + ep.InEnvelopeModel.Name,
								},
							},
						},
					}
				} else if ep.InputName != "" {
					mLower := strings.ToLower(ep.Method)
					if strings.Contains(mLower, "post") ||
						strings.Contains(mLower, "put") ||
						strings.Contains(mLower, "patch") ||
						strings.Contains(mLower, "delete") {
						var bodySchema *OpenAPISchema
						if inputMod := modelMap[ep.InputName]; inputMod != nil {
							bodySchema = &OpenAPISchema{
								Ref: "#/components/schemas/" + inputMod.Name,
							}
						} else {
							bodySchema = buildTypeSchema(ep.InputName, ctx.Scalars, modelMap)
						}
						op.RequestBody = &OpenAPIRequestBody{
							Required: true,
							Content: map[string]OpenAPIMediaType{
								reqMime: {
									Schema: bodySchema,
								},
							},
						}
					}
				}

				// Responses
				statusStr := "200"
				if ep.SuccessStatus > 0 {
					statusStr = fmt.Sprintf("%d", ep.SuccessStatus)
				}
				respMime := ep.ResponseMimeType
				if respMime == "" {
					respMime = "application/json"
				}

				successResp := OpenAPIResponse{
					Description: "Success",
					Content: map[string]OpenAPIMediaType{
						respMime: {
							Schema: buildTypeSchema(ep.InnerReturnType, ctx.Scalars, modelMap),
						},
					},
				}
				if len(ep.OutSealChain) > 0 && ep.OutEnvelopeModel != nil {
					// 物理传输出参使用信封模型定义
					successResp = OpenAPIResponse{
						Description: fmt.Sprintf("物理响应体：使用信封 %s 封装（解封后明文结构: %s）", ep.OutEnvelopeModel.Name, ep.OutPayloadDSL),
						Content: map[string]OpenAPIMediaType{
							respMime: {
								Schema: &OpenAPISchema{
									Ref: "#/components/schemas/" + ep.OutEnvelopeModel.Name,
								},
							},
						},
					}
				} else if ep.ReturnTypeBase != "" && doc.Components.Schemas[ep.ReturnTypeBase] != nil {
					successResp.Content[respMime] = OpenAPIMediaType{
						Schema: &OpenAPISchema{
							Ref: "#/components/schemas/" + ep.ReturnTypeBase,
						},
					}
				}
				op.Responses[statusStr] = successResp

				// Error response
				if ep.ErrorTypeBase != "" && doc.Components.Schemas[ep.ErrorTypeBase] != nil {
					errMime := ep.ErrorMimeType
					if errMime == "" {
						errMime = respMime
					}
					op.Responses["default"] = OpenAPIResponse{
						Description: "Error Response",
						Content: map[string]OpenAPIMediaType{
							errMime: {
								Schema: &OpenAPISchema{
									Ref: "#/components/schemas/" + ep.ErrorTypeBase,
								},
							},
						},
					}
				}

				doc.Paths[openApiPath][methodLower] = op
			}
		}
	}

	return doc
}

func convertToOpenAPIPath(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, ":") {
			parts[i] = "{" + strings.TrimPrefix(p, ":") + "}"
		}
	}
	return strings.Join(parts, "/")
}

func buildTypeSchema(t string, scalars map[string]*ScalarInfo, models map[string]*ModelInfo) *OpenAPISchema {
	clean := strings.TrimPrefix(t, "*")
	clean = strings.TrimSuffix(clean, "!")
	isArray := false
	if strings.HasPrefix(clean, "[]") {
		isArray = true
		clean = strings.TrimPrefix(clean, "[]")
		clean = strings.TrimPrefix(clean, "*")
		clean = strings.TrimSuffix(clean, "!")
	}

	var schema *OpenAPISchema
	switch strings.ToLower(clean) {
	case "string":
		schema = &OpenAPISchema{Type: "string"}
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		schema = &OpenAPISchema{Type: "integer"}
	case "float", "float32", "float64":
		schema = &OpenAPISchema{Type: "number"}
	case "bool", "boolean":
		schema = &OpenAPISchema{Type: "boolean"}
	case "time":
		schema = &OpenAPISchema{Type: "string", Format: "date-time"}
	case "file":
		schema = &OpenAPISchema{Type: "string", Format: "binary"}
	case "any":
		schema = &OpenAPISchema{Type: "object"}
	default:
		if models[clean] != nil {
			schema = &OpenAPISchema{Ref: "#/components/schemas/" + clean}
		} else if s, ok := scalars[clean]; ok {
			schema = buildTypeSchema(s.BaseType, scalars, models)
		} else {
			schema = &OpenAPISchema{Type: "string"}
		}
	}

	if isArray {
		return &OpenAPISchema{
			Type:  "array",
			Items: schema,
		}
	}
	return schema
}

func getBaseName(p string) string {
	if p == "" {
		return ""
	}
	return filepath.Base(p)
}
