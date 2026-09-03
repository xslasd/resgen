package test

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/xslasd/resgen/examples/resolver"
)

type mockEnumDemoBiz struct {
	lastCreateIn *resolver.CreateUserInput
	lastQueryArgs *resolver.QueryByRoleArgs
}

func (b *mockEnumDemoBiz) CreateUser(ctx context.Context, input *resolver.CreateUserInput) (*resolver.UserWithRole, error) {
	b.lastCreateIn = input
	return &resolver.UserWithRole{
		ID:        nil,
		Role:      &input.Role,
		Status:    &input.Status,
		CreatedAt: input.CreatedAt,
	}, nil
}

func (b *mockEnumDemoBiz) QueryByRole(ctx context.Context, input *resolver.QueryByRoleArgs) (*resolver.UserWithRole, error) {
	b.lastQueryArgs = input
	return &resolver.UserWithRole{
		Role:   input.Role,
		Status: input.Status,
	}, nil
}

func setupEnumDemoHandlers(biz *mockEnumDemoBiz) map[string]resolver.HandlerFunc[*TestServerContext] {
	handlers := make(map[string]resolver.HandlerFunc[*TestServerContext])
	en := resolver.NewEngine[*TestServerContext]().
		BindResponder(&TestResponder{}).
		BindValidator(&TestValidator{}).
		BindDecorator(&TestDecorator{}).
		BindRegister(func(e *resolver.Engine[*TestServerContext], info resolver.MethodInfo, handler resolver.HandlerFunc[*TestServerContext]) {
			key := fmt.Sprintf("%s %s", info.Method, info.Path)
			handlers[key] = handler
		})

	resolver.MountEnumDemo[any, *TestServerContext](en, biz)
	return handlers
}

func TestEnumDemo_ParsingAndValidation(t *testing.T) {
	biz := &mockEnumDemoBiz{}
	handlers := setupEnumDemoHandlers(biz)

	t.Run("1. POST /enum/create 正常提交枚举参数", func(t *testing.T) {
		h := handlers["POST /enum/create"]
		reqBody := `{"role":"admin","status":1,"created_at":946684800}`
		ctx := NewTestContext(httptest.NewRequest("POST", "/enum/create", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "CreateUser"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		if biz.lastCreateIn == nil || biz.lastCreateIn.Role != resolver.UserRole_ADMIN || biz.lastCreateIn.Status != resolver.RecordStatus_ENABLE {
			t.Fatalf("枚举值解析不匹配: %+v", biz.lastCreateIn)
		}
	})

	t.Run("2. POST /enum/create 提交非法 String 枚举值拦截", func(t *testing.T) {
		h := handlers["POST /enum/create"]
		reqBody := `{"role":"superman","status":1}`
		ctx := NewTestContext(httptest.NewRequest("POST", "/enum/create", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "CreateUser"})

		if ctx.resCode != 400 {
			t.Fatalf("期望状态码 400 (非法枚举校验拦截), 实际为: %d", ctx.resCode)
		}
	})

	t.Run("3. GET /enum/query Query 参数中的枚举提取与解析 (FromParam)", func(t *testing.T) {
		h := handlers["GET /enum/query"]
		req := httptest.NewRequest("GET", "/enum/query?role=guest&status=0", nil)
		ctx := NewTestContext(req)

		h(ctx, resolver.MethodInfo{Name: "QueryByRole"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		if biz.lastQueryArgs == nil || *biz.lastQueryArgs.Role != resolver.UserRole_GUEST || *biz.lastQueryArgs.Status != resolver.RecordStatus_DISABLE {
			t.Fatalf("Query 枚举解析不匹配: %+v", biz.lastQueryArgs)
		}
	})

	t.Run("4. GET /enum/query Query 参数中的非法枚举值拦截", func(t *testing.T) {
		h := handlers["GET /enum/query"]
		req := httptest.NewRequest("GET", "/enum/query?role=hacker", nil)
		ctx := NewTestContext(req)

		h(ctx, resolver.MethodInfo{Name: "QueryByRole"})

		if ctx.resCode != 400 {
			t.Fatalf("期望状态码 400 (Query 非法枚举拦截), 实际为: %d", ctx.resCode)
		}
	})
}
