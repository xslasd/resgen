package main

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"github.com/xslasd/resgen/examples/http_example/resolver"
	"strconv"
	"time"
)

// 1. 实现通用 ServerContext
type NativeHttpContext struct {
	W *ResponseWriterWrapper
	R *http.Request
}

func (c *NativeHttpContext) Bind(ctx *NativeHttpContext) resolver.ServerContext[*NativeHttpContext] {
	return ctx
}

func (c *NativeHttpContext) Native() *NativeHttpContext {
	return c
}

// ResponseWriterWrapper 用于捕获状态码（可选，此处为了演示 Render）
type ResponseWriterWrapper struct {
	http.ResponseWriter
	Status int
}

func (w *ResponseWriterWrapper) WriteHeader(code int) {
	w.Status = code
	w.ResponseWriter.WriteHeader(code)
}

// Responder 实现
type MyResponder struct{}

func (r *MyResponder) ErrorToStatus(ctx *NativeHttpContext, err error) int {
	if err == nil {
		return 200
	}
	return 500
}

func (r *MyResponder) BindResData(ctx *NativeHttpContext, data any, err error) resolver.ResData {
	res := resolver.ResData{Code: 0, Msg: "success", Data: data}
	if err != nil {
		res.Code = 500
		res.Msg = err.Error()
	}
	return res
}

func (c *NativeHttpContext) RenderJson(code int, obj any) {
	c.W.Header().Set("Content-Type", "application/json")
	c.W.WriteHeader(code)
	if obj != nil {
		json.NewEncoder(c.W).Encode(obj)
	}
}

func (c *NativeHttpContext) RenderText(code int, obj any) {
	c.W.Header().Set("Content-Type", "text/plain")
	c.W.WriteHeader(code)
	if obj != nil {
		fmt.Fprintf(c.W, "%v", obj)
	}
}

func (c *NativeHttpContext) RenderRaw(code int, contentType string, body []byte) {
	if contentType != "" {
		c.W.Header().Set("Content-Type", contentType)
	}
	c.W.WriteHeader(code)
	c.W.Write(body)
}

func (c *NativeHttpContext) SetHeader(name, value string) {
	c.W.Header().Set(name, value)
}

// 绑定实现
func (c *NativeHttpContext) Query(name string, dest any) error {
	return bindString(c.R.URL.Query().Get(name), dest)
}
func (c *NativeHttpContext) Path(name string, dest any) error {
	return bindString(c.R.PathValue(name), dest)
}
func (c *NativeHttpContext) Header(name string, dest any) error {
	return bindString(c.R.Header.Get(name), dest)
}

func (c *NativeHttpContext) Payload(source resolver.BodySource, dest any) error {
	switch string(source) {
	case "json":
		return json.NewDecoder(c.R.Body).Decode(dest)
	case "form", "multipart":
		// 标准库的结构化绑定比较原始，通常需要手动解析或使用第三方库
		// 这里简化处理，仅支持 JSON 结构化绑定
		return fmt.Errorf("structural binding for form/multipart is not implemented in this simple std adapter")
	default:
		return fmt.Errorf("unsupported payload source: %s", source)
	}
}

func (c *NativeHttpContext) Field(source resolver.BodySource, name string, dest any) error {
	switch string(source) {
	case "form", "multipart":
		// 如果是单个文件绑定
		if d, ok := dest.(**multipart.FileHeader); ok {
			_, fh, err := c.R.FormFile(name)
			if err == nil {
				*d = fh
			}
			return err
		}
		// 如果是普通字段
		return bindString(c.R.FormValue(name), dest)
	default:
		return fmt.Errorf("unsupported field source: %s", source)
	}
}

func (c *NativeHttpContext) Context() context.Context { return c.R.Context() }

// 辅助绑定函数
func bindString(val string, dest any) error {
	if val == "" {
		return nil
	}
	switch d := dest.(type) {
	case *string:
		*d = val
	case **string:
		*d = &val
	case *int:
		i, _ := strconv.Atoi(val)
		*d = i
	case **int:
		i, _ := strconv.Atoi(val)
		*d = &i
	}
	return nil
}

// 2. 模拟业务逻辑处理器
type UserHandler struct{}

func (u *UserHandler) GetProfile(ctx context.Context) (*resolver.User, error) {
	return &resolver.User{}, nil
}
func (u *UserHandler) UserCopy(ctx context.Context, input *resolver.UserCopyArgs) (*resolver.User, error) {
	return &resolver.User{}, nil
}
func (u *UserHandler) Login(ctx context.Context, input *resolver.LoginArgs) (*resolver.Token, error) {
	return &resolver.Token{}, nil
}
func (u *UserHandler) BindLogin(request resolver.ServerContextBase, input *resolver.LoginArgs) error {
	return nil
}
func (u *UserHandler) ValidateLogin(ctx *NativeHttpContext, input *resolver.LoginArgs) error {
	return nil
}
func (u *UserHandler) UpdateUser(ctx context.Context, input *resolver.UpdateUserArgs) (*resolver.User, error) {
	return &resolver.User{}, nil
}
func (u *UserHandler) OnInvoke_CheckOwner_UpdateUser(ctx *NativeHttpContext, info resolver.MethodInfo, input *resolver.UpdateUserArgs) error {
	return nil
}
func (u *UserHandler) OnResponse_MaskEmail_UpdateUser(ctx *NativeHttpContext, info resolver.MethodInfo, input *resolver.UpdateUserArgs, result *resolver.User, err error) (*resolver.User, error) {
	return result, err
}
func (u *UserHandler) GetUsers(ctx context.Context, input *resolver.GetUsersArgs) (*[]*resolver.User, error) {
	return nil, nil
}
func (u *UserHandler) GetUser(ctx context.Context, input *resolver.GetUserArgs) (*resolver.User, error) {
	return &resolver.User{}, nil
}
func (u *UserHandler) CreateUser(ctx context.Context, input *resolver.CreateUserInput) (*resolver.User, error) {
	return &resolver.User{}, nil
}
func (u *UserHandler) UpdateProfile(ctx context.Context, input *resolver.ProfileUpdateInput) (*string, error) {
	s := ""
	return &s, nil
}
func (u *UserHandler) UploadAvatar(ctx context.Context, input *resolver.UploadAvatarInput) (*string, error) {
	s := ""
	return &s, nil
}
func (u *UserHandler) DeleteUser(ctx context.Context, input *resolver.DeleteUserArgs) (*string, error) {
	s := ""
	return &s, nil
}
func (u *UserHandler) ExportUsers(ctx context.Context) (*string, error) {
	s := ""
	return &s, nil
}
func (u *UserHandler) TestValid(ctx context.Context, input *resolver.ValidGET) (any, error) {
	return input, nil
}

// 装饰器与验证器实现 (模拟)
type MyDecorator struct{}

func (d *MyDecorator) Auth(ctx *NativeHttpContext, role string) error          { return nil }
func (d *MyDecorator) Middleware(ctx *NativeHttpContext, names []string) error { return nil }
func (d *MyDecorator) LoginRequired(ctx *NativeHttpContext, info resolver.MethodInfo) error              { return nil }

type MyValidator struct{}

func (v *MyValidator) Required(ctx *NativeHttpContext, fieldName string, value any) error { return nil }
func (v *MyValidator) MinLen(ctx *NativeHttpContext, fieldName string, value any, val int) error {
	return nil
}
func (v *MyValidator) Min(ctx *NativeHttpContext, fieldName string, value any, Len int) error {
	return nil
}
func (v *MyValidator) Max(ctx *NativeHttpContext, fieldName string, value any, Len int) error {
	return nil
}
func (v *MyValidator) Email(ctx *NativeHttpContext, fieldName string, value any) error  { return nil }
func (v *MyValidator) Mobile(ctx *NativeHttpContext, fieldName string, value any) error { return nil }
func (v *MyValidator) Phone(ctx *NativeHttpContext, fieldName string, value any, msg *string) error {
	return nil
}
func (v *MyValidator) FileRule(ctx *NativeHttpContext, fieldName string, value any, maxSize int, ext []string, msg string) error {
	return nil
}
func (v *MyValidator) EqField(ctx *NativeHttpContext, fieldName string, value any, other any) error {
	if value != other {
		return fmt.Errorf("%s 必须等于目标字段", fieldName)
	}
	return nil
}
func (v *MyValidator) GtField(ctx *NativeHttpContext, fieldName string, value any, other any) error {
	t1, ok1 := value.(time.Time)
	t2, ok2 := other.(time.Time)
	if ok1 && ok2 && !t1.After(t2) {
		return fmt.Errorf("%s 必须晚于开始时间", fieldName)
	}
	return nil
}
func (v *MyValidator) GeField(ctx *NativeHttpContext, fieldName string, value any, other any) error {
	return nil
}

func main() {
	// 初始化引擎
	mux := http.NewServeMux()
	en := resolver.NewEngine[*NativeHttpContext]().
		BindResponder(&MyResponder{}).
		BindRegister(func(e *resolver.Engine[*NativeHttpContext], info resolver.MethodInfo, handler resolver.HandlerFunc[*NativeHttpContext]) {
			mux.HandleFunc(fmt.Sprintf("%s %s", info.Method, info.Path), func(w http.ResponseWriter, r *http.Request) {
				ctx := &NativeHttpContext{
					W: &ResponseWriterWrapper{ResponseWriter: w, Status: 200},
					R: r,
				}
				handler(ctx, info)
			})
		}).
		BindDecorator(&MyDecorator{}).
		BindValidator(&MyValidator{})

	resolver.MountUserCenter(en, &UserHandler{})
	fmt.Println("Resgen API Server is running on :8080")
	http.ListenAndServe(":8080", mux)
}
