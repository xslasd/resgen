package generator

import (
	"fmt"
	"path/filepath"
	"strings"
)

// OpenAPIDocument represents the OpenAPI 3.0.3 root document
type OpenAPIDocument struct {
	OpenAPI    string                            `json:"openapi"`
	Info       ApiInfo                           `json:"info"`
	Servers    []OpenAPIServer                   `json:"servers,omitempty"`
	Tags       []OpenAPITag                      `json:"tags,omitempty"`
	Paths      map[string]map[string]OpenAPIOperation `json:"paths"`
	Components OpenAPIComponents                 `json:"components"`

	// Specification Extensions for resgen compatibility and tools
	Version    string                            `json:"version,omitempty"`
	Package    string                            `json:"package,omitempty"`
	Validators []MetaInfo                        `json:"validators,omitempty"`
	Scalars    map[string]*ScalarInfo            `json:"scalars,omitempty"`
	Modules    []ModuleInfo                      `json:"modules,omitempty"`
	Models     []*ModelInfo                      `json:"models,omitempty"`
	Enums      []*EnumInfo                       `json:"enums,omitempty"`
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
	Tags        []string                    `json:"tags,omitempty"`
	Summary     string                      `json:"summary,omitempty"`
	Description string                      `json:"description,omitempty"`
	OperationID string                      `json:"operationId,omitempty"`
	Parameters  []OpenAPIParameter          `json:"parameters,omitempty"`
	RequestBody *OpenAPIRequestBody         `json:"requestBody,omitempty"`
	Responses   map[string]OpenAPIResponse  `json:"responses"`
	
	// Resgen extensions
	ResFile     string                      `json:"x-res-file,omitempty"`
	ResGroup    string                      `json:"x-res-group,omitempty"`
	ResModule   string                      `json:"x-res-module,omitempty"`
	Permission  string                      `json:"x-res-permission,omitempty"`
	ReturnType  string                      `json:"x-res-return-type,omitempty"`
	ReturnTypeDSL string                    `json:"x-res-return-type-dsl,omitempty"`
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
	
	// Resgen extension
	ResFile              string                    `json:"x-res-file,omitempty"`
}

// ConvertToOpenAPI3 converts the internal DataContext to OpenAPI 3.0.3 document
func ConvertToOpenAPI3(ctx *DataContext) *OpenAPIDocument {
	doc := &OpenAPIDocument{
		OpenAPI:    "3.0.3",
		Info:       ctx.Info,
		Paths:      make(map[string]map[string]OpenAPIOperation),
		Components: OpenAPIComponents{
			Schemas: make(map[string]*OpenAPISchema),
		},
		Version:    ctx.Version,
		Package:    ctx.Package,
		Validators: ctx.Validators,
		Scalars:    ctx.Scalars,
		Modules:    ctx.Modules,
		Models:     ctx.Models,
		Enums:      ctx.OrderedEnums,
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
			if f.Doc != "" {
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
		for _, c := range e.Cases {
			enumVals = append(enumVals, c.Value)
		}
		eSchema.Enum = enumVals
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
					Tags:          []string{mod.Name},
					Summary:       ep.Name,
					Description:   ep.Doc,
					OperationID:   ep.Name,
					ResFile:       ep.SourceFile,
					ResGroup:      grp.Path,
					ResModule:     mod.Name,
					Permission:    ep.Permission,
					ReturnType:    ep.ReturnType,
					ReturnTypeDSL: ep.ReturnTypeDSL,
					Responses:     make(map[string]OpenAPIResponse),
				}

				// Build Parameters
				for _, arg := range ep.Args {
					inType := strings.ToLower(arg.Source)
					if inType == "body" {
						continue // Handled in requestBody
					}
					if inType == "" {
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

				if ep.InputName != "" && modelMap[ep.InputName] != nil {
					inputMod := modelMap[ep.InputName]
					if strings.Contains(strings.ToLower(ep.Method), "post") ||
						strings.Contains(strings.ToLower(ep.Method), "put") ||
						strings.Contains(strings.ToLower(ep.Method), "patch") {
						op.RequestBody = &OpenAPIRequestBody{
							Required: true,
							Content: map[string]OpenAPIMediaType{
								reqMime: {
									Schema: &OpenAPISchema{
										Ref: "#/components/schemas/" + inputMod.Name,
									},
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
				if ep.ReturnTypeBase != "" && doc.Components.Schemas[ep.ReturnTypeBase] != nil {
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
	isArray := false
	if strings.HasPrefix(clean, "[]") {
		isArray = true
		clean = strings.TrimPrefix(clean, "[]")
		clean = strings.TrimPrefix(clean, "*")
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
