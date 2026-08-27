package test

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/xslasd/resgen/examples/resolver"
	"github.com/xslasd/resgen/examples/scalars"
)

// ==========================================
// 1. Mock ServerContext
// ==========================================

type TestServerContext struct {
	req         *http.Request
	resCode     int
	resBody     any
	resRawType  string
	pathParams  map[string]string
	queryParams map[string]string
	headers     map[string]string
	formData    map[string]string
	formFiles   map[string]*multipart.FileHeader
}

func NewTestContext(req *http.Request) *TestServerContext {
	if req == nil {
		req, _ = http.NewRequest("GET", "/", nil)
	}
	return &TestServerContext{
		req:         req,
		pathParams:  make(map[string]string),
		queryParams: make(map[string]string),
		headers:     make(map[string]string),
		formData:    make(map[string]string),
		formFiles:   make(map[string]*multipart.FileHeader),
	}
}

func (c *TestServerContext) Native() any {
	return c.req
}

func (c *TestServerContext) Context() context.Context {
	return c.req.Context()
}

func (c *TestServerContext) SetFormData(key, value string) *TestServerContext {
	c.formData[key] = value
	return c
}

func (c *TestServerContext) SetFormFile(key string, file *multipart.FileHeader) *TestServerContext {
	c.formFiles[key] = file
	return c
}

func (c *TestServerContext) SetPathParam(key, value string) *TestServerContext {
	c.pathParams[key] = value
	return c
}

func (c *TestServerContext) SetQueryParam(key, value string) *TestServerContext {
	c.queryParams[key] = value
	return c
}

func (c *TestServerContext) GetPath(name string) string {
	if v, ok := c.pathParams[name]; ok {
		return v
	}
	return ""
}

func (c *TestServerContext) GetQuery(name string) string {
	if v, ok := c.queryParams[name]; ok {
		return v
	}
	return c.req.URL.Query().Get(name)
}

func (c *TestServerContext) GetHeader(name string) string {
	if v, ok := c.headers[name]; ok {
		return v
	}
	return c.req.Header.Get(name)
}

func bindFormFields(val reflect.Value, prefix string, formData map[string]string, formFiles map[string]*multipart.FileHeader) {
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			val.Set(reflect.New(val.Type().Elem()))
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return
	}
	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		f := val.Field(i)
		if !f.CanSet() {
			continue
		}
		sf := typ.Field(i)
		jsonTag := sf.Tag.Get("json")
		formTag := sf.Tag.Get("form")
		name := sf.Name
		if formTag != "" {
			name = formTag
		} else if jsonTag != "" {
			name = jsonTag
		}

		fullKey := name
		if prefix != "" {
			fullKey = prefix + "." + name
		}

		// 嵌套结构体递归绑定（排除文件对象 *multipart.FileHeader / multipart.FileHeader）
		isFile := f.Type() == reflect.TypeOf(&multipart.FileHeader{}) || f.Type() == reflect.TypeOf(multipart.FileHeader{})
		if !isFile && (f.Kind() == reflect.Struct || (f.Kind() == reflect.Ptr && f.Type().Elem().Kind() == reflect.Struct)) {
			bindFormFields(f, fullKey, formData, formFiles)
			continue
		}

		// 匹配 formFiles
		if file, ok := formFiles[fullKey]; ok {
			if f.Type() == reflect.TypeOf(&multipart.FileHeader{}) {
				f.Set(reflect.ValueOf(file))
			} else if f.Type() == reflect.TypeOf(multipart.FileHeader{}) {
				f.Set(reflect.ValueOf(*file))
			}
		} else if file, ok := formFiles[name]; ok && prefix == "" {
			if f.Type() == reflect.TypeOf(&multipart.FileHeader{}) {
				f.Set(reflect.ValueOf(file))
			} else if f.Type() == reflect.TypeOf(multipart.FileHeader{}) {
				f.Set(reflect.ValueOf(*file))
			}
		}

		// 匹配 formData (支持带前缀 fullKey 如 "address.city"，也支持直接 key 如 "city")
		strVal, hasVal := formData[fullKey]
		if !hasVal {
			strVal, hasVal = formData[name]
		}
		if hasVal {
			if f.Kind() == reflect.String {
				f.SetString(strVal)
			} else if f.Kind() == reflect.Ptr && f.Type().Elem().Kind() == reflect.String {
				f.Set(reflect.ValueOf(&strVal))
			} else if f.Kind() == reflect.Int || f.Kind() == reflect.Int64 {
				var intVal int64
				fmt.Sscanf(strVal, "%d", &intVal)
				f.SetInt(intVal)
			} else if f.Kind() == reflect.Ptr && (f.Type().Elem().Kind() == reflect.Int || f.Type().Elem().Kind() == reflect.Int64) {
				var intVal int
				fmt.Sscanf(strVal, "%d", &intVal)
				f.Set(reflect.ValueOf(&intVal))
			}
		}
	}
}

func (c *TestServerContext) Payload(source resolver.BodySource, dest any) error {
	s := string(source)
	if s == "multipart" || s == "form" || s == "application/x-www-form-urlencoded" || s == "multipart/form-data" {
		if len(c.formData) > 0 || len(c.formFiles) > 0 {
			val := reflect.ValueOf(dest)
			bindFormFields(val, "", c.formData, c.formFiles)
			return nil
		}
	}
	if c.req.Body == nil {
		return fmt.Errorf("request body is nil")
	}
	defer c.req.Body.Close()
	return json.NewDecoder(c.req.Body).Decode(dest)
}

func (c *TestServerContext) Field(source resolver.BodySource, name string, dest any) error {
	if file, ok := c.formFiles[name]; ok {
		if d, ok := dest.(**multipart.FileHeader); ok {
			*d = file
			return nil
		}
	}
	if val, ok := c.formData[name]; ok {
		if d, ok := dest.(*string); ok {
			*d = val
			return nil
		}
	}
	return nil
}

func (c *TestServerContext) RenderJson(code int, obj any) {
	c.resCode = code
	c.resBody = obj
}

func (c *TestServerContext) RenderText(code int, obj any) {
	c.resCode = code
	c.resBody = obj
}

func (c *TestServerContext) RenderXml(code int, obj any) {
	c.resCode = code
	c.resBody = obj
}

func (c *TestServerContext) RenderStream(code int, localFileDownload resolver.LocalFileDownload) {
	c.resCode = code
	c.resBody = localFileDownload
	if localFileDownload.ContentType != "" {
		c.resRawType = localFileDownload.ContentType
	}
}

func (c *TestServerContext) RenderRaw(code int, contentType string, body []byte) {
	c.resCode = code
	c.resRawType = contentType
	c.resBody = body
}

// ==========================================
// 2. Mock Validator
// ==========================================

type TestValidator struct{}

func (v *TestValidator) Required(ctx any, fieldName string, value any) error {
	if value == nil || value == "" {
		return fmt.Errorf("字段 [%s] 是必填项", fieldName)
	}
	return nil
}

func (v *TestValidator) Email(ctx any, fieldName string, value any) error {
	if s, ok := value.(string); ok && s != "" {
		if !strings.Contains(s, "@") {
			return fmt.Errorf("字段 [%s] 邮箱格式不正确: %s", fieldName, s)
		}
	}
	return nil
}

func (v *TestValidator) Mobile(ctx any, fieldName string, value any) error {
	if s, ok := value.(string); ok && s != "" {
		if len(s) != 11 {
			return fmt.Errorf("字段 [%s] 手机号必须为11位", fieldName)
		}
	}
	return nil
}

func (v *TestValidator) Min(ctx any, fieldName string, value any, Len int) error {
	if s, ok := value.(string); ok && len(s) < Len {
		return fmt.Errorf("字段 [%s] 长度不能小于 %d", fieldName, Len)
	}
	return nil
}

func (v *TestValidator) Max(ctx any, fieldName string, value any, Len int) error {
	if s, ok := value.(string); ok && len(s) > Len {
		return fmt.Errorf("字段 [%s] 长度不能大于 %d", fieldName, Len)
	}
	return nil
}

func (v *TestValidator) TimeBefore(ctx any, fieldName string, value any, targetField any) error {
	t1, ok1 := value.(scalars.IntTime)
	t2, ok2 := targetField.(scalars.IntTime)
	if ok1 && ok2 {
		if time.Time(t1).After(time.Time(t2)) {
			return fmt.Errorf("字段 [%s] 时间必须早于目标时间", fieldName)
		}
	}
	return nil
}

func (v *TestValidator) FileRule(ctx any, fieldName string, value any, MaxSize int, Types []string, Msg string) error {
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

	if file.Size > int64(MaxSize) {
		if Msg != "" {
			return fmt.Errorf("%s", Msg)
		}
		return fmt.Errorf("文件 [%s] 大小超过限制: %d 字节", fieldName, MaxSize)
	}

	contentType := file.Header.Get("Content-Type")
	matched := false
	for _, t := range Types {
		if contentType == t {
			matched = true
			break
		}
	}
	if !matched {
		if Msg != "" {
			return fmt.Errorf("%s", Msg)
		}
		return fmt.Errorf("文件 [%s] 类型 [%s] 不被允许，支持类型: %v", fieldName, contentType, Types)
	}

	return nil
}

func (v *TestValidator) EnumError(ctx any, fieldName string, enumType string, value any) error {
	return &resolver.InvalidEnumError{
		FieldName: fieldName,
		EnumType:  enumType,
		Value:     value,
	}
}

// ==========================================
// 3. Mock Responder
// ==========================================

type TestResponder struct{}

func (r *TestResponder) ErrorToStatus(ctx any, err error) int {
	return 400
}

func (r *TestResponder) BindResData(ctx any, data any, err error) resolver.ResData {
	if err != nil {
		return resolver.ResData{Code: 400, Msg: err.Error(), Data: nil}
	}
	return resolver.ResData{Code: 200, Msg: "success", Data: data}
}

func (r *TestResponder) BindListRes(ctx any, data any, err error) resolver.ListRes {
	var rows []any
	if data != nil {
		rows = []any{data}
	}
	return resolver.ListRes{Rows: rows, Total: len(rows)}
}

func (r *TestResponder) BindTreeRes(ctx any, data any, err error) resolver.TreeRes {
	var items []any
	if data != nil {
		items = []any{data}
	}
	return resolver.TreeRes{Items: items, Total: len(items)}
}

func (r *TestResponder) BindPageData(ctx any, data any, err error) resolver.PageData {
	return resolver.PageData{List: data, Total: 100, Page: 1}
}

// ==========================================
// 4. Mock Decorator
// ==========================================

type TestDecorator struct {
	AuthFunc          func(ctx any, info resolver.MethodInfo, Role string) error
	LoginRequiredFunc func(ctx any, info resolver.MethodInfo) error
}

func (d *TestDecorator) Auth(ctx any, info resolver.MethodInfo, Role string) error {
	if d.AuthFunc != nil {
		return d.AuthFunc(ctx, info, Role)
	}
	return nil
}

func (d *TestDecorator) LoginRequired(ctx any, info resolver.MethodInfo) error {
	if d.LoginRequiredFunc != nil {
		return d.LoginRequiredFunc(ctx, info)
	}
	return nil
}
