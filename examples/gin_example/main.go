package main

import (
	"context"
	"fmt"
	"mime/multipart"

	"github.com/xslasd/resgen/examples/resolver"

	_ "embed"
	"time"

	"github.com/gin-gonic/gin"
)

// --- 1. 实现 ServerContext 适配器 ---
type GinContext struct {
	GC *gin.Context // 改为命名字段，避免嵌入带来的同名冲突
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
		// 如果是单个文件绑定
		if d, ok := dest.(**multipart.FileHeader); ok {
			f, err := c.GC.FormFile(name)
			if err == nil {
				*d = f
			}
			return err
		}
		// 如果是普通 Form 字段
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

// --- 3. 业务逻辑处理器实现 ---
type UserHandler struct{}

func (u *UserHandler) GetProfile(ctx context.Context) (*resolver.User, error) {
	return &resolver.User{Username: "resgen_user"}, nil
}
func (u *UserHandler) Login(ctx context.Context, input *resolver.LoginArgs) (*resolver.Token, error) {
	return &resolver.Token{Token: "abc"}, nil
}

func (u *UserHandler) BindLogin(request resolver.ServerContextBase, input *resolver.LoginArgs) error {
	fmt.Printf(">>> [Custom Bind] Login\n")
	// 手动绑定
	if val := request.GetQuery("username"); val != "" {
		input.Username = &val
	}
	if val := request.GetQuery("password"); val != "" {
		input.Password = &val
	}
	return nil
}

func (u *UserHandler) ValidateLogin(ctx *gin.Context, input *resolver.LoginArgs) error {
	fmt.Printf(">>> [Custom Validate] Login\n")
	if input.Username == nil || *input.Username == "" {
		return fmt.Errorf("username is required")
	}
	return nil
}

func (u *UserHandler) UserCopy(ctx context.Context, id string) (*resolver.User, error) {
	return &resolver.User{Username: "copy_" + id}, nil
}

func (u *UserHandler) GetUsers(ctx context.Context, input *resolver.GetUsersArgs) (*[]*resolver.User, error) {
	return &[]*resolver.User{{Username: "user1"}}, nil
}
func (u *UserHandler) GetUser(ctx context.Context, id *int) (*resolver.User, error) {
	return &resolver.User{Id: *id}, nil
}
func (u *UserHandler) CreateUser(ctx context.Context, input *resolver.CreateUserInput) (*resolver.User, error) {
	return &resolver.User{Username: input.Username}, nil
}

func (u *UserHandler) OnInvoke_CheckOwner_UpdateUser(ctx *gin.Context, info resolver.MethodInfo, input *resolver.UpdateUserArgs) error {
	fmt.Printf(">>> [Specialized Decorator] CheckOwner for UpdateUser\n")
	return nil
}

func (u *UserHandler) UpdateUser(ctx context.Context, input *resolver.UpdateUserArgs) (*resolver.User, error) {
	return &resolver.User{Username: "updated_user"}, nil
}

func (u *UserHandler) OnResponse_MaskEmail_UpdateUser(ctx *gin.Context, info resolver.MethodInfo, input *resolver.UpdateUserArgs, result *resolver.User, err error) (*resolver.User, error) {
	fmt.Printf("<<< [Specialized Decorator] MaskEmail for UpdateUser\n")
	if result != nil {
		result.Email = "****@example.com"
	}
	return result, err
}
func (u *UserHandler) UpdateProfile(ctx context.Context, input *resolver.ProfileUpdateInput) (*string, error) {
	s := "ok"
	return &s, nil
}
func (u *UserHandler) UploadAvatar(ctx context.Context, input *resolver.UploadAvatarInput) (*string, error) {
	s := "uploaded"
	return &s, nil
}
func (u *UserHandler) DeleteUser(ctx context.Context, id *int) (*string, error) {
	s := "deleted"
	return &s, nil
}
func (u *UserHandler) ExportUsers(ctx context.Context) (*string, error) {
	s := "ID,Name,Email\n1,Alice,alice@example.com"
	return &s, nil
}
func (u *UserHandler) TestValid(ctx context.Context, input *resolver.ValidGET) (any, error) {
	return input, nil
}

// --- TaskCenter 业务逻辑 ---
type TaskHandler struct{}

func (t *TaskHandler) CreateTask(ctx context.Context, input *resolver.CreateTaskInput) (*string, error) {
	s := "task created: " + input.Title
	return &s, nil
}

func (t *TaskHandler) GetTasks(ctx context.Context, input *resolver.GetTasksInput) (*string, error) {
	s := "task list"
	return &s, nil
}

// --- ScalarCenter 业务逻辑 ---
type ScalarHandler struct{}

func (s *ScalarHandler) TestScalar(ctx context.Context, time *time.Time) (*resolver.ScalarOutput, error) {
	return &resolver.ScalarOutput{Time: time}, nil
}

func (s *ScalarHandler) TestScalarBody(ctx context.Context, input *resolver.ScalarInput) (*resolver.ScalarOutput, error) {
	return &resolver.ScalarOutput{Time: input.Body_time}, nil
}

// 装饰器与验证器实现 (模拟)
type MyDecorator struct{}

func (d *MyDecorator) Auth(ctx *gin.Context, role string) error                       { return nil }
func (d *MyDecorator) Middleware(ctx *gin.Context, names []string) error              { return nil }
func (d *MyDecorator) LoginRequired(ctx *gin.Context, info resolver.MethodInfo) error { return nil }

type MyValidator struct{}

func (v *MyValidator) Required(ctx *gin.Context, fieldName string, value any) error { return nil }
func (v *MyValidator) MinLen(ctx *gin.Context, fieldName string, value any, val int) error {
	return nil
}
func (v *MyValidator) Min(ctx *gin.Context, fieldName string, value any, Len int) error { return nil }
func (v *MyValidator) Max(ctx *gin.Context, fieldName string, value any, Len int) error { return nil }
func (v *MyValidator) Email(ctx *gin.Context, fieldName string, value any) error        { return nil }
func (v *MyValidator) Mobile(ctx *gin.Context, fieldName string, value any) error       { return nil }
func (v *MyValidator) Phone(ctx *gin.Context, fieldName string, value any, msg *string) error {
	return nil
}
func (v *MyValidator) FileRule(ctx *gin.Context, fieldName string, value any, maxSize int, types []string, msg string) error {
	file, ok := value.(*multipart.FileHeader)
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
func (v *MyValidator) EqField(ctx *gin.Context, fieldName string, value any, other any) error {
	return nil
}
func (v *MyValidator) GtField(ctx *gin.Context, fieldName string, value any, other any) error {
	return nil
}
func (v *MyValidator) GeField(ctx *gin.Context, fieldName string, value any, other any) error {
	return nil
}

func main() {
	r := gin.Default()

	// 初始化引擎
	en := resolver.NewEngine[*GinContext]().
		BindResponder(&MyResponder{}).
		BindRegister(func(e *resolver.Engine[*GinContext], info resolver.MethodInfo, handler resolver.HandlerFunc[*GinContext]) {
			r.Handle(info.Method, info.Path, func(ctx *gin.Context) {
				handler(&GinContext{GC: ctx}, info)
			})
		}).
		BindDecorator(&MyDecorator{}).
		BindValidator(&MyValidator{})

	// 挂载业务模块
	resolver.MountUserCenter(en, &UserHandler{})
	resolver.MountTaskCenter(en, &TaskHandler{})
	resolver.MountScalarCenter(en, &ScalarHandler{})

	fmt.Println("Resgen Gin Example running on :8080")
	r.Run(":8080")
}
