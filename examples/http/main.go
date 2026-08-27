package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/xslasd/resgen/examples/resolver"
	"github.com/xslasd/resgen/examples/scalars"
)

// NativeCtx 包含标准库的 request 和 response writer
type NativeCtx struct {
	W http.ResponseWriter
	R *http.Request
}

// --- 1. 实现 ServerContext 适配器 ---
type HttpContext struct {
	NC *NativeCtx
}

func (c *HttpContext) Bind(nc *NativeCtx) resolver.ServerContext[*NativeCtx] {
	return &HttpContext{NC: nc}
}

func (c *HttpContext) Native() *NativeCtx {
	return c.NC
}

func (c *HttpContext) GetPath(name string) string {
	// 简单实现，如果 Go版本>=1.22，可用 r.PathValue(name)
	return c.NC.R.PathValue(name)
}

func (c *HttpContext) GetQuery(name string) string {
	return c.NC.R.URL.Query().Get(name)
}

func (c *HttpContext) GetHeader(name string) string {
	return c.NC.R.Header.Get(name)
}

func (c *HttpContext) Payload(source resolver.BodySource, dest any) error {
	if source == "json" || source == "application/json" {
		defer c.NC.R.Body.Close()
		return json.NewDecoder(c.NC.R.Body).Decode(dest)
	}
	return fmt.Errorf("unsupported payload source: %s", source)
}

func (c *HttpContext) Field(source resolver.BodySource, name string, dest any) error {
	c.NC.R.ParseMultipartForm(32 << 20)
	if d, ok := dest.(**multipart.FileHeader); ok {
		_, h, err := c.NC.R.FormFile(name)
		if err == nil {
			*d = h
		}
		return err
	}
	val := c.NC.R.FormValue(name)
	if d, ok := dest.(*string); ok {
		*d = val
	}
	return nil
}

func (c *HttpContext) RenderJson(code int, obj any) {
	c.NC.W.Header().Set("Content-Type", "application/json")
	c.NC.W.WriteHeader(code)
	json.NewEncoder(c.NC.W).Encode(obj)
}

func (c *HttpContext) RenderText(code int, obj any) {
	c.NC.W.Header().Set("Content-Type", "text/plain")
	c.NC.W.WriteHeader(code)
	fmt.Fprintf(c.NC.W, "%v", obj)
}

func (c *HttpContext) RenderRaw(code int, contentType string, body []byte) {
	c.NC.W.Header().Set("Content-Type", contentType)
	c.NC.W.WriteHeader(code)
	c.NC.W.Write(body)
}

func (c *HttpContext) RenderStream(code int, localFileDownload resolver.LocalFileDownload) {
	if localFileDownload.ContentType != "" {
		c.NC.W.Header().Set("Content-Type", localFileDownload.ContentType)
	} else {
		c.NC.W.Header().Set("Content-Type", "application/octet-stream")
	}
	c.NC.W.Header().Set("Content-Disposition", `attachment; filename="`+localFileDownload.Filename+`"`)
	c.NC.W.WriteHeader(code)
	c.NC.W.Write([]byte("mock file content of " + localFileDownload.FilePath))
}

func (c *HttpContext) RenderXml(code int, obj any) {
	c.NC.W.Header().Set("Content-Type", "application/xml; charset=utf-8")
	c.NC.W.WriteHeader(code)
	b, err := xml.Marshal(obj)
	if err != nil {
		fmt.Fprintf(c.NC.W, "<error>%v</error>", err)
		return
	}
	c.NC.W.Write(b)
}

func (c *HttpContext) SetHeader(name, value string) {
	c.NC.W.Header().Set(name, value)
}

func (c *HttpContext) Context() context.Context {
	return c.NC.R.Context()
}

// --- 2. 实现 Responder 契约 ---
type MyResponder struct{}

func (r *MyResponder) ErrorToStatus(ctx *NativeCtx, err error) int {
	if err == nil {
		return 200
	}
	return 500
}

func (r *MyResponder) BindResData(ctx *NativeCtx, data any, err error) resolver.ResData {
	res := resolver.ResData{Code: 0, Msg: "success", Data: data}
	if err != nil {
		res.Code = 500
		if ce, ok := err.(resolver.CodedError); ok {
			res.Code = ce.Code()
		}
		res.Msg = err.Error()
		res.Data = nil
	}
	return res
}

func (r *MyResponder) BindListRes(ctx *NativeCtx, data any, err error) resolver.ListRes {
	return resolver.ListRes{Rows: []any{data}, Total: 1}
}

func (r *MyResponder) BindTreeRes(ctx *NativeCtx, data any, err error) resolver.TreeRes {
	return resolver.TreeRes{Items: []any{data}, Total: 1}
}

func (r *MyResponder) BindPageData(ctx *NativeCtx, data any, err error) resolver.PageData {
	return resolver.PageData{List: data, Total: 100, Page: 1}
}

// --- 3. 业务逻辑处理器实现 (复用，仅修改上下文类型) ---

type AuthDemoHandler struct{}

func (h *AuthDemoHandler) Register(ctx context.Context, input *resolver.RegisterInput) (*resolver.User, error) {
	return &resolver.User{Id: 1, Username: input.Username, Email: input.Email}, nil
}

func (h *AuthDemoHandler) SetPeriod(ctx context.Context, input *resolver.TaskPeriodInput) (*string, error) {
	s := "period set successfully"
	return &s, nil
}

func (h *AuthDemoHandler) BindLogin(request resolver.ServerContextBase, input *resolver.LoginArgs) error {
	return nil
}

func (h *AuthDemoHandler) ValidateLogin(ctx *NativeCtx, input *resolver.LoginArgs) error {
	return nil
}

func (h *AuthDemoHandler) Login(ctx context.Context, input *resolver.LoginArgs) (*resolver.Token, error) {
	return &resolver.Token{Token: "mock-token-abc", ExpiresAt: int(time.Now().Add(time.Hour).Unix())}, nil
}

func (h *AuthDemoHandler) GetMe(ctx context.Context) (*resolver.User, error) {
	return &resolver.User{Id: 1, Username: "admin", Email: "admin@example.com"}, nil
}

func (h *AuthDemoHandler) OnInvoke_CheckOwner_UpdateUser(ctx *NativeCtx, info resolver.MethodInfo, input *resolver.UpdateInput) error {
	return nil
}

func (h *AuthDemoHandler) UpdateUser(ctx context.Context, input *resolver.UpdateInput) (*resolver.User, error) {
	var email string
	if input.Email != nil {
		email = *input.Email
	}
	return &resolver.User{Id: input.Id, Username: "updated", Email: email}, nil
}

func (h *AuthDemoHandler) OnResponse_MaskEmail_UpdateUser(ctx *NativeCtx, info resolver.MethodInfo, input *resolver.UpdateInput, result *resolver.User, err error) (*resolver.User, error) {
	if result != nil {
		result.Email = "****@example.com"
	}
	return result, err
}

func (h *AuthDemoHandler) DeleteUser(ctx context.Context, id *int) (*string, error) {
	s := fmt.Sprintf("deleted user: %d", *id)
	return &s, nil
}

type WrapperDemoHandler struct{}

func (h *WrapperDemoHandler) GetArticle(ctx context.Context, id *int) (*resolver.Article, error) {
	return &resolver.Article{Id: *id, Title: "Title", Content: "Content"}, nil
}

func (h *WrapperDemoHandler) ListArticles(ctx context.Context, input *resolver.ListArticlesArgs) (*[]*resolver.Article, error) {
	return &[]*resolver.Article{{Id: 1, Title: "A1", Content: "C1"}}, nil
}

func (h *WrapperDemoHandler) ListArticlesV2(ctx context.Context, input *resolver.ListArticlesV2Args) (*resolver.ListResArticle, error) {
	return &resolver.ListResArticle{Rows: []resolver.Article{{Id: 1, Title: "A1", Content: "C1"}}, Total: 1}, nil
}

func (h *WrapperDemoHandler) GetCategoryTree(ctx context.Context) (*resolver.TreeResCategoryTreeNode, error) {
	child := resolver.CategoryTreeNode{Id: 101, ParentId: 1, Name: "后端开发", Sort: 1}
	root := resolver.CategoryTreeNode{Id: 1, ParentId: 0, Name: "技术分类", Sort: 1, Children: &[]resolver.CategoryTreeNode{child}}
	return &resolver.TreeResCategoryTreeNode{Items: []resolver.CategoryTreeNode{root}, Total: 2}, nil
}

func (h *WrapperDemoHandler) GetCategoryTreeRaw(ctx context.Context) (*resolver.CategoryTreeNode, error) {
	return &resolver.CategoryTreeNode{Id: 1, ParentId: 0, Name: "原生分类", Sort: 1}, nil
}

func (h *WrapperDemoHandler) CreateArticle(ctx context.Context, input *resolver.CreateArticleArgs) (*resolver.Article, error) {
	return &resolver.Article{Id: 100, Title: input.Title, Content: input.Content}, nil
}

func (h *WrapperDemoHandler) GetArticleRaw(ctx context.Context, id *int) (*resolver.Article, error) {
	return &resolver.Article{Id: *id, Title: "Raw Title", Content: "Raw Content"}, nil
}

func (h *WrapperDemoHandler) Logout(ctx context.Context) error {
	return nil
}

type ScalarDemoHandler struct{}

func (h *ScalarDemoHandler) GetEventByTime(ctx context.Context, startTime *scalars.IntTime) (*resolver.Event, error) {
	now := scalars.IntTime(time.Now())
	return &resolver.Event{Id: 1, Name: "Time Event", StartTime: *startTime, EndTime: &now, CreatedAt: &now}, nil
}

func (h *ScalarDemoHandler) ListEvents(ctx context.Context, input *resolver.QueryEventsInput) (*[]*resolver.Event, error) {
	now := scalars.IntTime(time.Now())
	var start scalars.IntTime
	if input.After != nil {
		start = *input.After
	} else {
		start = now
	}
	return &[]*resolver.Event{{Id: 1, Name: "Event 1", StartTime: start, EndTime: &now, CreatedAt: &now}}, nil
}

func (h *ScalarDemoHandler) CreateEvent(ctx context.Context, input *resolver.CreateEventInput) (*resolver.Event, error) {
	now := scalars.IntTime(time.Now())
	return &resolver.Event{Id: 2, Name: input.Name, StartTime: input.StartTime, EndTime: &input.EndTime, CreatedAt: &now}, nil
}

type ContentTypeDemoHandler struct{}

func (h *ContentTypeDemoHandler) SubmitJson(ctx context.Context, input *resolver.JsonInput) (*string, error) {
	s := "json submitted: " + input.Title
	return &s, nil
}
func (h *ContentTypeDemoHandler) SubmitForm(ctx context.Context, input *resolver.FormInput) (*string, error) {
	s := "form submitted"
	return &s, nil
}
func (h *ContentTypeDemoHandler) SubmitNestedForm(ctx context.Context, input *resolver.NestedFormInput) (*string, error) {
	s := fmt.Sprintf("nested form submitted: name=%s, age=%d, city=%s, street=%s", input.Name, input.Age, input.Address.City, input.Address.Street)
	return &s, nil
}
func (h *ContentTypeDemoHandler) SubmitMultipart(ctx context.Context, title string) (*string, error) {
	s := "multipart submitted"
	return &s, nil
}
func (h *ContentTypeDemoHandler) ExportText(ctx context.Context) (*string, error) {
	s := "raw text content"
	return &s, nil
}
func (h *ContentTypeDemoHandler) ExportJson(ctx context.Context) (*resolver.Report, error) {
	return &resolver.Report{Title: "JSON Report"}, nil
}
func (h *ContentTypeDemoHandler) ExportXml(ctx context.Context) (*resolver.Report, error) {
	return &resolver.Report{Title: "XML Report"}, nil
}

type StatusDemoHandler struct{}

func (h *StatusDemoHandler) GetProduct(ctx context.Context, id *int) (*resolver.Product, error) {
	return &resolver.Product{Id: *id, Name: "Product", Price: 9.9}, nil
}
func (h *StatusDemoHandler) CreateProduct(ctx context.Context, input *resolver.CreateProductInput) (*resolver.Product, error) {
	return &resolver.Product{Id: 2, Name: input.Name, Price: input.Price}, nil
}
func (h *StatusDemoHandler) BatchUpdate(ctx context.Context, ids []int) (*string, error) {
	s := "updated"
	return &s, nil
}
func (h *StatusDemoHandler) DeleteProduct(ctx context.Context, id *int) (*string, error) {
	s := "deleted product"
	return &s, nil
}
func (h *StatusDemoHandler) ListProducts(ctx context.Context, input *resolver.ListProductsArgs) ([]resolver.Product, error) {
	return []resolver.Product{{Id: 1, Name: "P1", Price: 1.0}}, nil
}
func (h *StatusDemoHandler) GetRawProduct(ctx context.Context, id *int) (*resolver.Product, error) {
	return &resolver.Product{Id: *id, Name: "Raw", Price: 10.0}, nil
}
func (h *StatusDemoHandler) GetRawProducts(ctx context.Context, page *int) (*[]*resolver.Product, error) {
	return &[]*resolver.Product{{Id: 1, Name: "Raw P1", Price: 2.0}}, nil
}

type FileDemoHandler struct{}

func (h *FileDemoHandler) UploadAvatar(ctx context.Context, input *resolver.UploadAvatarInput) (*resolver.UploadResult, error) {
	return &resolver.UploadResult{FileUrl: "/uploads/avatar.png"}, nil
}
func (h *FileDemoHandler) UploadDocument(ctx context.Context, input *resolver.UploadDocumentInput) (*resolver.UploadResult, error) {
	return &resolver.UploadResult{FileUrl: "/uploads/document.pdf"}, nil
}
func (h *FileDemoHandler) DownloadFile(ctx context.Context, id *int) (*resolver.LocalFileDownload, error) {
	return &resolver.LocalFileDownload{
		FilePath: "./temp/sample.pdf",
		Filename: "sample.pdf",
	}, nil
}
func (h *FileDemoHandler) ExportCsv(ctx context.Context, ids *string) (*resolver.LocalFileDownload, error) {
	return &resolver.LocalFileDownload{
		FilePath: "./temp/export.csv",
		Filename: "export.csv",
	}, nil
}
func (h *FileDemoHandler) DownloadDynamic(ctx context.Context, id *int) (*resolver.LocalFileDownload, error) {
	return &resolver.LocalFileDownload{
		FilePath:    "./temp/dynamic.bin",
		Filename:    "dynamic.bin",
		ContentType: "application/octet-stream",
	}, nil
}
func (h *FileDemoHandler) CreatePost(ctx context.Context, input *resolver.CreatePostInput) (*resolver.ContentPostItem, error) {
	return &resolver.ContentPostItem{
		Id:      1,
		Type:    input.Type,
		Payload: input.Payload,
	}, nil
}

func (h *FileDemoHandler) GetPost(ctx context.Context, id *int) (*resolver.ContentPostItem, error) {
	return &resolver.ContentPostItem{
		Id:   *id,
		Type: resolver.UnionKind_article,
		Payload: &resolver.UnionArticle{
			Title:   "Sample Article",
			Content: "This is a sample article returned as a union payload.",
		},
	}, nil
}

type EnumDemoHandler struct{}

func (h *EnumDemoHandler) CreateUser(ctx context.Context, input *resolver.CreateUserInput) (*resolver.UserWithRole, error) {
	return &resolver.UserWithRole{Role: &input.Role}, nil
}

func (h *EnumDemoHandler) QueryByRole(ctx context.Context, input *resolver.QueryByRoleArgs) (*resolver.UserWithRole, error) {
	return &resolver.UserWithRole{}, nil
}

type MyDecorator struct{}
func (d *MyDecorator) Auth(ctx *NativeCtx, info resolver.MethodInfo, Role string) error { return nil }
func (d *MyDecorator) LoginRequired(ctx *NativeCtx, info resolver.MethodInfo) error { return nil }

type MyValidator struct{}
func (v *MyValidator) Required(ctx *NativeCtx, fieldName string, value any) error { return nil }
func (v *MyValidator) Email(ctx *NativeCtx, fieldName string, value any) error { return nil }
func (v *MyValidator) Mobile(ctx *NativeCtx, fieldName string, value any) error { return nil }
func (v *MyValidator) Min(ctx *NativeCtx, fieldName string, value any, Len int) error { return nil }
func (v *MyValidator) Max(ctx *NativeCtx, fieldName string, value any, Len int) error { return nil }
func (v *MyValidator) TimeBefore(ctx *NativeCtx, fieldName string, value any, targetField any) error { return nil }
func (v *MyValidator) FileRule(ctx *NativeCtx, fieldName string, value any, maxSize int, types []string, msg string) error { return nil }
func (v *MyValidator) EnumError(ctx *NativeCtx, fieldName string, enumType string, value any) error {
	return &resolver.InvalidEnumError{
		FieldName: fieldName,
		EnumType:  enumType,
		Value:     value,
	}
}


func convertPath(path string) string {
	// Simple convert /events/:startTime to /events/{startTime} for Go 1.22
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, ":") {
			parts[i] = "{" + p[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}

func main() {
	mux := http.NewServeMux()
	
	en := resolver.NewEngine[*HttpContext]().
		BindResponder(&MyResponder{}).
		BindRegister(func(e *resolver.Engine[*HttpContext], info resolver.MethodInfo, handler resolver.HandlerFunc[*HttpContext]) {
			httpPath := convertPath(info.Path)
			routePattern := info.Method + " " + httpPath
			fmt.Printf("%-6s %-30s --> %s \n", info.Method, httpPath, info.HandlerPos)
			mux.HandleFunc(routePattern, func(w http.ResponseWriter, r *http.Request) {
				handler(&HttpContext{NC: &NativeCtx{W: w, R: r}}, info)
			})
		}).
		BindDecorator(&MyDecorator{}).
		BindValidator(&MyValidator{})

	resolver.MountAuthDemo[*NativeCtx, *HttpContext](en, &AuthDemoHandler{})
	resolver.MountWrapperDemo[*NativeCtx, *HttpContext](en, &WrapperDemoHandler{})
	resolver.MountScalarDemo[*NativeCtx, *HttpContext](en, &ScalarDemoHandler{})
	resolver.MountContentTypeDemo[*NativeCtx, *HttpContext](en, &ContentTypeDemoHandler{})
	resolver.MountStatusDemo[*NativeCtx, *HttpContext](en, &StatusDemoHandler{})
	resolver.MountFileDemo[*NativeCtx, *HttpContext](en, &FileDemoHandler{})
	resolver.MountEnumDemo[*NativeCtx, *HttpContext](en, &EnumDemoHandler{})

	fmt.Println("Resgen HTTP Example running on :8081")
	http.ListenAndServe(":8081", mux)
}
