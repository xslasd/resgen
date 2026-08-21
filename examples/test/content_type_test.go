package test

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xslasd/resgen/examples/resolver"
)

type mockContentTypeDemoBiz struct{}

func (b *mockContentTypeDemoBiz) SubmitJson(ctx context.Context, input *resolver.JsonInput) (*string, error) {
	res := fmt.Sprintf("收到 JSON: %s", input.Title)
	return &res, nil
}

func (b *mockContentTypeDemoBiz) SubmitForm(ctx context.Context, input *resolver.FormInput) (*string, error) {
	res := fmt.Sprintf("收到 Form: %s <%s>", input.Name, input.Email)
	return &res, nil
}

func (b *mockContentTypeDemoBiz) SubmitNestedForm(ctx context.Context, input *resolver.NestedFormInput) (*string, error) {
	zip := ""
	if input.Address.ZipCode != nil {
		zip = *input.Address.ZipCode
	}
	res := fmt.Sprintf("收到嵌套 Form: name=%s, age=%d, city=%s, street=%s, zip=%s", input.Name, input.Age, input.Address.City, input.Address.Street, zip)
	return &res, nil
}

func (b *mockContentTypeDemoBiz) SubmitMultipart(ctx context.Context, title string) (*string, error) {
	res := fmt.Sprintf("收到 Multipart: %s", title)
	return &res, nil
}

func (b *mockContentTypeDemoBiz) ExportText(ctx context.Context) (*string, error) {
	text := "这是导出的纯文本内容报表"
	return &text, nil
}

func (b *mockContentTypeDemoBiz) ExportJson(ctx context.Context) (*resolver.Report, error) {
	return &resolver.Report{Title: "JSON 报表", Summary: "报表概要内容"}, nil
}

func (b *mockContentTypeDemoBiz) ExportXml(ctx context.Context) (*resolver.Report, error) {
	return &resolver.Report{Title: "XML 报表", Summary: "XML 格式数据"}, nil
}

func setupContentTypeDemoHandlers() map[string]resolver.HandlerFunc[*TestServerContext] {
	handlers := make(map[string]resolver.HandlerFunc[*TestServerContext])
	en := resolver.NewEngine[*TestServerContext]().
		BindResponder(&TestResponder{}).
		BindValidator(&TestValidator{}).
		BindDecorator(&TestDecorator{}).
		BindRegister(func(e *resolver.Engine[*TestServerContext], info resolver.MethodInfo, handler resolver.HandlerFunc[*TestServerContext]) {
			key := fmt.Sprintf("%s %s", info.Method, info.Path)
			handlers[key] = handler
		})

	resolver.MountContentTypeDemo[any, *TestServerContext](en, &mockContentTypeDemoBiz{})
	return handlers
}

func TestContentTypeDemo_Endpoints(t *testing.T) {
	handlers := setupContentTypeDemoHandlers()

	t.Run("1. POST /content/json 默认 JSON 协议请求", func(t *testing.T) {
		h := handlers["POST /content/json"]
		reqBody := `{"title":"Go 架构指南","content":"架构演进"}`
		ctx := NewTestContext(httptest.NewRequest("POST", "/content/json", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "SubmitJson"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
	})

	t.Run("2. POST /content/form [ctype=form] 表单提交", func(t *testing.T) {
		h := handlers["POST /content/form"]
		reqBody := `{"name":"张三","email":"zhangsan@example.com"}`
		ctx := NewTestContext(httptest.NewRequest("POST", "/content/form", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "SubmitForm"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
	})

	t.Run("2.2. POST /content/form/nested [ctype=form] 多层结构嵌套表单提交", func(t *testing.T) {
		h := handlers["POST /content/form/nested"]
		ctx := NewTestContext(httptest.NewRequest("POST", "/content/form/nested", nil))
		ctx.SetFormData("name", "李四")
		ctx.SetFormData("age", "28")
		ctx.SetFormData("address.city", "深圳市")
		ctx.SetFormData("address.street", "南山区科技园南路")
		ctx.SetFormData("address.zip_code", "518000")

		h(ctx, resolver.MethodInfo{Name: "SubmitNestedForm"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		res, ok := ctx.resBody.(resolver.ResData)
		if !ok || res.Code != 200 {
			t.Fatalf("响应内容不是 ResData: %+v", ctx.resBody)
		}
		strPtr, ok := res.Data.(*string)
		if !ok || !strings.Contains(*strPtr, "深圳市") || !strings.Contains(*strPtr, "南山区科技园南路") {
			t.Fatalf("多层嵌套表单绑定结果异常: %v", res.Data)
		}
	})

	t.Run("3. GET /export/text [ctype=text, etype=json, wrap=ResData] 文本响应", func(t *testing.T) {
		h := handlers["GET /export/text"]
		ctx := NewTestContext(httptest.NewRequest("GET", "/export/text", nil))

		h(ctx, resolver.MethodInfo{Name: "ExportText"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		res, ok := ctx.resBody.(resolver.ResData)
		if !ok || res.Code != 200 {
			t.Fatalf("响应内容不是 ResData 包装: %+v", ctx.resBody)
		}
	})

	t.Run("4. GET /export/xml [ctype=xml, etype=json, wrap=ResData] XML 包装响应", func(t *testing.T) {
		h := handlers["GET /export/xml"]
		ctx := NewTestContext(httptest.NewRequest("GET", "/export/xml", nil))

		h(ctx, resolver.MethodInfo{Name: "ExportXml"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		res, ok := ctx.resBody.(resolver.ResData)
		if !ok || res.Code != 200 {
			t.Fatalf("XML 响应内容不是 ResData 包装: %+v", ctx.resBody)
		}
		// 验证 XML 序列化完全正常且不报错
		xmlBytes, err := xml.Marshal(res)
		if err != nil {
			t.Fatalf("XML 序列化 ResData 失败: %v", err)
		}
		if !strings.Contains(string(xmlBytes), "<ResData>") || !strings.Contains(string(xmlBytes), "XML 报表") {
			t.Fatalf("生成的 XML 内容不符合规范: %s", string(xmlBytes))
		}
	})
}
