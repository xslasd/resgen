package main

import (
	"context"
	"fmt"
	"mime/multipart"
	"os"
	"time"

	"github.com/xslasd/resgen/examples/resolver"
	"github.com/xslasd/resgen/examples/scalars"

	"github.com/gin-gonic/gin"
)

// --- 1. 实现 ServerContext 适配器 ---
type GinContext struct {
	GC *gin.Context
}

func (c *GinContext) Bind(gc *gin.Context) resolver.ServerContext[*gin.Context] {
	return &GinContext{GC: gc}
}

func (c *GinContext) Native() *gin.Context {
	return c.GC
}

func (c *GinContext) GetPath(name string) string {
	return c.GC.Param(name)
}

func (c *GinContext) GetQuery(name string) string {
	return c.GC.Query(name)
}

func (c *GinContext) GetHeader(name string) string {
	return c.GC.GetHeader(name)
}

func (c *GinContext) Payload(source resolver.BodySource, dest any) error {
	switch string(source) {
	case "json":
		return c.GC.ShouldBindJSON(dest)
	case "form", "multipart":
		return c.GC.ShouldBind(dest)
	default:
		s := string(source)
		if s == "application/json" {
			return c.GC.ShouldBindJSON(dest)
		}
		return fmt.Errorf("unsupported payload source: %s", source)
	}
}

func (c *GinContext) Field(source resolver.BodySource, name string, dest any) error {
	switch string(source) {
	case "form", "multipart":
		if d, ok := dest.(**multipart.FileHeader); ok {
			f, err := c.GC.FormFile(name)
			if err == nil {
				*d = f
			}
			return err
		}
		val := c.GC.PostForm(name)
		if d, ok := dest.(*string); ok {
			*d = val
		}
		return nil
	default:
		return fmt.Errorf("unsupported field source: %s", source)
	}
}

func (c *GinContext) RenderJson(code int, obj any) {
	c.GC.JSON(code, obj)
}

func (c *GinContext) RenderText(code int, obj any) {
	c.GC.String(code, "%v", obj)
}

func (c *GinContext) RenderRaw(code int, contentType string, body []byte) {
	c.GC.Data(code, contentType, body)
}

func (c *GinContext) RenderStream(code int, localFileDownload resolver.LocalFileDownload) {
	if localFileDownload.ContentType != "" {
		c.GC.Header("Content-Type", localFileDownload.ContentType)
	}
	c.GC.FileAttachment(localFileDownload.FilePath, localFileDownload.Filename)
}

func (c *GinContext) RenderXml(code int, obj any) {
	c.GC.XML(code, obj)
}

func (c *GinContext) SetHeader(name, value string) {
	c.GC.Header(name, value)
}

func (c *GinContext) Context() context.Context {
	return c.GC.Request.Context()
}

// --- 2. 实现 Responder 契约 ---
type MyResponder struct{}

func (r *MyResponder) ErrorToStatus(ctx *gin.Context, err error) int {
	if err == nil {
		return 200
	}
	return 500
}

func (r *MyResponder) BindResData(ctx *gin.Context, data any, err error) resolver.ResData {
	res := resolver.ResData{
		Code: 0,
		Msg:  "success",
		Data: data,
	}
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

func (r *MyResponder) BindListRes(ctx *gin.Context, data any, err error) resolver.ListRes {
	res := resolver.ListRes{
		Rows:  []any{data},
		Total: 1,
	}
	return res
}

func (r *MyResponder) BindTreeRes(ctx *gin.Context, data any, err error) resolver.TreeRes {
	res := resolver.TreeRes{
		Items: []any{data},
		Total: 1,
	}
	return res
}

func (r *MyResponder) BindPageData(ctx *gin.Context, data any, err error) resolver.PageData {
	res := resolver.PageData{
		List:  data,
		Total: 100,
		Page:  1,
	}
	return res
}

// --- 3. 业务逻辑处理器实现 ---

// AuthDemoHandler
type AuthDemoHandler struct{}

func (h *AuthDemoHandler) Register(ctx context.Context, input *resolver.RegisterInput) (*resolver.User, error) {
	return &resolver.User{Id: 1, Username: input.Username, Email: input.Email}, nil
}

func (h *AuthDemoHandler) SetPeriod(ctx context.Context, input *resolver.TaskPeriodInput) (*string, error) {
	s := "period set successfully"
	return &s, nil
}

func (h *AuthDemoHandler) BindLogin(request resolver.ServerContextBase, input *resolver.LoginArgs) error {
	fmt.Printf(">>> [Custom Bind] Login\n")
	if val := request.GetQuery("username"); val != "" {
		input.Username = &val
	}
	if val := request.GetQuery("password"); val != "" {
		input.Password = &val
	}
	return nil
}

func (h *AuthDemoHandler) ValidateLogin(ctx *gin.Context, input *resolver.LoginArgs) error {
	fmt.Printf(">>> [Custom Validate] Login\n")
	if input.Username == nil || *input.Username == "" {
		return fmt.Errorf("username is required")
	}
	return nil
}

func (h *AuthDemoHandler) Login(ctx context.Context, input *resolver.LoginArgs) (*resolver.Token, error) {
	return &resolver.Token{Token: "mock-token-abc", ExpiresAt: int(time.Now().Add(time.Hour).Unix())}, nil
}

func (h *AuthDemoHandler) GetMe(ctx context.Context) (*resolver.User, error) {
	return &resolver.User{Id: 1, Username: "admin", Email: "admin@example.com"}, nil
}

func (h *AuthDemoHandler) OnInvoke_CheckOwner_UpdateUser(ctx *gin.Context, info resolver.MethodInfo, input *resolver.UpdateInput) error {
	fmt.Printf(">>> [Specialized Decorator] CheckOwner for UpdateUser\n")
	return nil
}

func (h *AuthDemoHandler) UpdateUser(ctx context.Context, input *resolver.UpdateInput) (*resolver.User, error) {
	username := "updated_user"
	var email string
	if input.Email != nil {
		email = *input.Email
	}
	return &resolver.User{Id: input.Id, Username: username, Email: email}, nil
}

func (h *AuthDemoHandler) OnResponse_MaskEmail_UpdateUser(ctx *gin.Context, info resolver.MethodInfo, input *resolver.UpdateInput, result *resolver.User, err error) (*resolver.User, error) {
	fmt.Printf("<<< [Specialized Decorator] MaskEmail for UpdateUser\n")
	if result != nil {
		result.Email = "****@example.com"
	}
	return result, err
}

func (h *AuthDemoHandler) DeleteUser(ctx context.Context, id *int) (*string, error) {
	s := fmt.Sprintf("deleted user: %d", *id)
	return &s, nil
}

var _ resolver.WrapperDemoResolver[any] = (*WrapperDemoHandler)(nil)

// WrapperDemoHandler
type WrapperDemoHandler struct{}

func (h *WrapperDemoHandler) GetArticle(ctx context.Context, id *int) (*resolver.Article, error) {
	return &resolver.Article{Id: *id, Title: "Title", Content: "Content"}, nil
}

func (h *WrapperDemoHandler) ListArticles(ctx context.Context, input *resolver.ListArticlesArgs) (*[]*resolver.Article, error) {
	list := []*resolver.Article{
		{Id: 1, Title: "Article 1", Content: "Content 1"},
		{Id: 2, Title: "Article 2", Content: "Content 2"},
	}
	return &list, nil
}

func (h *WrapperDemoHandler) ListArticlesV2(ctx context.Context, input *resolver.ListArticlesV2Args) (*resolver.ListResArticle, error) {
	list := []resolver.Article{
		{Id: 1, Title: "Article 1 (V2)", Content: "Content 1 (V2)"},
		{Id: 2, Title: "Article 2 (V2)", Content: "Content 2 (V2)"},
	}
	return &resolver.ListResArticle{
		Rows:  list,
		Total: 2,
	}, nil
}

func (h *WrapperDemoHandler) GetCategoryTree(ctx context.Context) (*resolver.TreeResCategoryTreeNode, error) {
	child1 := resolver.CategoryTreeNode{
		Id:       101,
		ParentId: 1,
		Name:     "后端开发",
		Sort:     1,
	}
	child2 := resolver.CategoryTreeNode{
		Id:       102,
		ParentId: 1,
		Name:     "前端开发",
		Sort:     2,
	}
	root := resolver.CategoryTreeNode{
		Id:       1,
		ParentId: 0,
		Name:     "技术专栏",
		Sort:     1,
		Children: &[]resolver.CategoryTreeNode{child1, child2},
	}
	return &resolver.TreeResCategoryTreeNode{
		Items: []resolver.CategoryTreeNode{root},
		Total: 3,
	}, nil
}

func (h *WrapperDemoHandler) GetCategoryTreeRaw(ctx context.Context) (*resolver.CategoryTreeNode, error) {
	child := resolver.CategoryTreeNode{
		Id:       201,
		ParentId: 2,
		Name:     "分布式数据库",
		Sort:     1,
	}
	return &resolver.CategoryTreeNode{
		Id:       2,
		ParentId: 0,
		Name:     "数据存储",
		Sort:     1,
		Children: &[]resolver.CategoryTreeNode{child},
	}, nil
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

// ScalarDemoHandler
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
	list := []*resolver.Event{
		{Id: 1, Name: "Event 1", StartTime: start, EndTime: &now, CreatedAt: &now},
	}
	return &list, nil
}

func (h *ScalarDemoHandler) CreateEvent(ctx context.Context, input *resolver.CreateEventInput) (*resolver.Event, error) {
	now := scalars.IntTime(time.Now())
	return &resolver.Event{Id: 2, Name: input.Name, StartTime: input.StartTime, EndTime: &input.EndTime, CreatedAt: &now}, nil
}

// ContentTypeDemoHandler
type ContentTypeDemoHandler struct{}

func (h *ContentTypeDemoHandler) SubmitJson(ctx context.Context, input *resolver.JsonInput) (*string, error) {
	s := "json submitted: " + input.Title
	return &s, nil
}

func (h *ContentTypeDemoHandler) SubmitForm(ctx context.Context, input *resolver.FormInput) (*string, error) {
	s := "form submitted: " + input.Name
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
	return &resolver.Report{Title: "JSON Report", Summary: "JSON Summary"}, nil
}

func (h *ContentTypeDemoHandler) ExportXml(ctx context.Context) (*resolver.Report, error) {
	return &resolver.Report{Title: "XML Report", Summary: "XML Summary"}, nil
}

// StatusDemoHandler
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
	return &resolver.Product{Id: *id, Name: "Raw Product", Price: 10.0}, nil
}

func (h *StatusDemoHandler) GetRawProducts(ctx context.Context, page *int) (*[]*resolver.Product, error) {
	list := []*resolver.Product{{Id: 1, Name: "Raw P1", Price: 2.0}}
	return &list, nil
}

// FileDemoHandler
type FileDemoHandler struct{}

func (h *FileDemoHandler) UploadAvatar(ctx context.Context, input *resolver.UploadAvatarInput) (*resolver.UploadResult, error) {
	size := 0
	if input.Avatar != nil {
		size = int(input.Avatar.Size)
	}
	return &resolver.UploadResult{
		FileUrl:  "/uploads/avatar.png",
		FileSize: size,
		MimeType: "image/png",
	}, nil
}

func (h *FileDemoHandler) UploadDocument(ctx context.Context, input *resolver.UploadDocumentInput) (*resolver.UploadResult, error) {
	size := 0
	if input.Document != nil {
		size = int(input.Document.Size)
	}
	return &resolver.UploadResult{
		FileUrl:  "/uploads/document.pdf",
		FileSize: size,
		MimeType: "application/pdf",
	}, nil
}

func (h *FileDemoHandler) DownloadFile(ctx context.Context, id *int) (*resolver.LocalFileDownload, error) {
	filePath := "./temp/sample.pdf"
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		_ = os.MkdirAll("./temp", 0755)
		_ = os.WriteFile(filePath, []byte("%PDF-1.4 Mock PDF Content Generated by Resgen Demo"), 0644)
	}
	return &resolver.LocalFileDownload{
		FilePath: filePath,
		Filename: "sample.pdf",
	}, nil
}

func (h *FileDemoHandler) ExportCsv(ctx context.Context, ids *string) (*resolver.LocalFileDownload, error) {
	filePath := "./temp/export.csv"
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		_ = os.MkdirAll("./temp", 0755)
		_ = os.WriteFile(filePath, []byte("id,name,role\n1,Alice,admin\n2,Bob,editor\n"), 0644)
	}
	return &resolver.LocalFileDownload{
		FilePath: filePath,
		Filename: "export.csv",
	}, nil
}

func (h *FileDemoHandler) DownloadDynamic(ctx context.Context, id *int) (*resolver.LocalFileDownload, error) {
	filePath := "./temp/dynamic.bin"
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		_ = os.MkdirAll("./temp", 0755)
		_ = os.WriteFile(filePath, []byte("Binary dynamic payload data"), 0644)
	}
	return &resolver.LocalFileDownload{
		FilePath:    filePath,
		Filename:    "dynamic.bin",
		ContentType: "application/octet-stream",
	}, nil
}

func (h *FileDemoHandler) CreatePost(ctx context.Context, input *resolver.CreatePostInput) (*resolver.ContentPostItem, error) {
	fmt.Printf(">>> [CreatePost] Type: %s, Payload: %+v\n", input.Type, input.Payload)
	return &resolver.ContentPostItem{
		Id:      1,
		Type:    input.Type,
		Payload: input.Payload,
	}, nil
}

func (h *FileDemoHandler) BatchCreatePost(ctx context.Context, input *resolver.BatchCreatePostInput) (*resolver.BatchCreatePostResult, error) {
	fmt.Printf(">>> [BatchCreatePost] BatchName: %s, Items: %d\n", input.BatchName, len(input.Items))
	var items []resolver.ContentPostItem
	for i, it := range input.Items {
		items = append(items, resolver.ContentPostItem{
			Id:      i + 1,
			Type:    it.Type,
			Payload: it.Payload,
		})
	}
	return &resolver.BatchCreatePostResult{
		BatchName: input.BatchName,
		Total:     len(items),
		Items:     items,
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
	return &resolver.UserWithRole{ Role: &input.Role }, nil
}

func (h *EnumDemoHandler) QueryByRole(ctx context.Context, input *resolver.QueryByRoleArgs) (*resolver.UserWithRole, error) {
	return &resolver.UserWithRole{}, nil
}

// 装饰器与验证器实现
type MyDecorator struct{}

func (d *MyDecorator) Auth(ctx *gin.Context, info resolver.MethodInfo, Role string) error {
	fmt.Printf(">>> [Decorator] Auth: %s\n", Role)
	return nil
}

func (d *MyDecorator) LoginRequired(ctx *gin.Context, info resolver.MethodInfo) error {
	fmt.Printf(">>> [Decorator] LoginRequired\n")
	return nil
}

type MyValidator struct{}

func (v *MyValidator) Required(ctx *gin.Context, fieldName string, value any) error {
	return nil
}

func (v *MyValidator) Email(ctx *gin.Context, fieldName string, value any) error {
	return nil
}

func (v *MyValidator) Mobile(ctx *gin.Context, fieldName string, value any) error {
	return nil
}

func (v *MyValidator) Min(ctx *gin.Context, fieldName string, value any, Len int) error {
	return nil
}

func (v *MyValidator) Max(ctx *gin.Context, fieldName string, value any, Len int) error {
	return nil
}

func (v *MyValidator) TimeBefore(ctx *gin.Context, fieldName string, value any, targetField any) error {
	return nil
}

func (v *MyValidator) FileRule(ctx *gin.Context, fieldName string, value any, maxSize int, types []string, msg string) error {
	var file multipart.FileHeader
	var ok bool
	if fileVal, okVal := value.(multipart.FileHeader); okVal {
		file = fileVal
		ok = true
	} else if pFileVal, okPVal := value.(*multipart.FileHeader); okPVal && pFileVal != nil {
		file = *pFileVal
		ok = true
	}

	if !ok {
		return nil
	}

	// 1. 校验文件大小
	if file.Size > int64(maxSize) {
		if msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return fmt.Errorf("文件 [%s] 超过限制 (%d 字节)", fieldName, maxSize)
	}

	// 2. 校验文件类型 (MIME Type)
	contentType := file.Header.Get("Content-Type")
	found := false
	for _, t := range types {
		if contentType == t {
			found = true
			break
		}
	}
	if !found {
		if msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return fmt.Errorf("文件 [%s] 类型 [%s] 不被允许，仅支持: %v", fieldName, contentType, types)
	}

	return nil
}

func (v *MyValidator) EnumError(ctx *gin.Context, fieldName string, enumType string, value any) error {
	return &resolver.InvalidEnumError{
		FieldName: fieldName,
		EnumType:  enumType,
		Value:     value,
	}
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	// 初始化引擎
	en := resolver.NewEngine[*GinContext]().
		BindResponder(&MyResponder{}).
		BindRegister(func(e *resolver.Engine[*GinContext], info resolver.MethodInfo, handler resolver.HandlerFunc[*GinContext]) {
			fmt.Printf("%-6s %-30s --> %s \n", info.Method, info.Path, info.HandlerPos)
			r.Handle(info.Method, info.Path, func(ctx *gin.Context) {
				handler(&GinContext{GC: ctx}, info)
			})
		}).
		BindDecorator(&MyDecorator{}).
		BindValidator(&MyValidator{})

	// 挂载业务模块
	resolver.MountAuthDemo(en, &AuthDemoHandler{})
	resolver.MountWrapperDemo(en, &WrapperDemoHandler{})
	resolver.MountScalarDemo(en, &ScalarDemoHandler{})
	resolver.MountContentTypeDemo(en, &ContentTypeDemoHandler{})
	resolver.MountStatusDemo(en, &StatusDemoHandler{})
	resolver.MountFileDemo[*gin.Context, *GinContext](en, &FileDemoHandler{})
	resolver.MountEnumDemo[*gin.Context, *GinContext](en, &EnumDemoHandler{})

	fmt.Println("Resgen Gin Example running on :8080")
	r.Run(":8080")
}
