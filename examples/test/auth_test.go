package test

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xslasd/resgen/examples/resolver"
)

type mockAuthDemoBiz struct {
	checkOwnerCalled bool
	maskEmailCalled  bool
}

func (b *mockAuthDemoBiz) Register(ctx context.Context, input *resolver.RegisterInput) (*resolver.User, error) {
	return &resolver.User{ID: 1, Username: input.Username, Email: input.Email}, nil
}

func (b *mockAuthDemoBiz) SetPeriod(ctx context.Context, input *resolver.TaskPeriodInput) (*string, error) {
	res := "周期设置成功"
	return &res, nil
}

func (b *mockAuthDemoBiz) BindLogin(request resolver.ServerContextBase, input *resolver.LoginArgs) error {
	if val := request.GetQuery("username"); val != "" {
		input.Username = &val
	}
	if val := request.GetQuery("password"); val != "" {
		input.Password = &val
	}
	return nil
}

func (b *mockAuthDemoBiz) ValidateLogin(ctx any, input *resolver.LoginArgs) error {
	if input.Username == nil || *input.Username == "" {
		return fmt.Errorf("自定义校验失败：username 不能为空")
	}
	return nil
}

func (b *mockAuthDemoBiz) Login(ctx context.Context, input *resolver.LoginArgs) (*resolver.Token, error) {
	return &resolver.Token{
		Token:     "mock-jwt-token-123456",
		ExpiresAt: int(time.Now().Add(time.Hour).Unix()),
	}, nil
}

func (b *mockAuthDemoBiz) GetMe(ctx context.Context) (*resolver.User, error) {
	return &resolver.User{ID: 1, Username: "admin_user", Email: "admin@resgen.dev"}, nil
}

func (b *mockAuthDemoBiz) OnInvoke_CheckOwner_UpdateUser(ctx any, info resolver.MethodInfo, input *resolver.UpdateInput) error {
	b.checkOwnerCalled = true
	if input.ID == 999 {
		return fmt.Errorf("权限不足：您不是该资源的所有者")
	}
	return nil
}

func (b *mockAuthDemoBiz) UpdateUser(ctx context.Context, input *resolver.UpdateInput) (*resolver.User, error) {
	email := "new_email@resgen.dev"
	if input.Email != nil {
		email = *input.Email
	}
	return &resolver.User{ID: input.ID, Username: "updated_user", Email: email}, nil
}

func (b *mockAuthDemoBiz) OnResponse_MaskEmail_UpdateUser(ctx any, info resolver.MethodInfo, input *resolver.UpdateInput, result *resolver.User, err error) (*resolver.User, error) {
	b.maskEmailCalled = true
	if result != nil {
		result.Email = "u***@resgen.dev"
	}
	return result, err
}

func (b *mockAuthDemoBiz) DeleteUser(ctx context.Context, id *int) (*string, error) {
	res := fmt.Sprintf("用户 %d 删除成功", *id)
	return &res, nil
}

func setupAuthDemoHandlers(biz *mockAuthDemoBiz, decorator *TestDecorator) map[string]resolver.HandlerFunc[*TestServerContext] {
	handlers := make(map[string]resolver.HandlerFunc[*TestServerContext])
	en := resolver.NewEngine[*TestServerContext]().
		BindResponder(&TestResponder{}).
		BindValidator(&TestValidator{}).
		BindDecorator(decorator).
		BindRegister(func(e *resolver.Engine[*TestServerContext], info resolver.MethodInfo, handler resolver.HandlerFunc[*TestServerContext]) {
			key := fmt.Sprintf("%s %s", info.Method, info.Path)
			handlers[key] = handler
		})

	resolver.MountAuthDemo[any, *TestServerContext](en, biz)
	return handlers
}

func TestAuthDemo_ValidationAndDecorators(t *testing.T) {
	biz := &mockAuthDemoBiz{}
	decorator := &TestDecorator{}
	handlers := setupAuthDemoHandlers(biz, decorator)

	t.Run("1. 注册接口参数校验：正常通过", func(t *testing.T) {
		h := handlers["POST /auth/register"]
		reqBody := `{"username":"alex","email":"alex@example.com","mobile":"13800138000","password":"secure_password"}`
		ctx := NewTestContext(httptest.NewRequest("POST", "/auth/register", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "Register"})

		if ctx.resCode != 201 {
			t.Fatalf("期望状态码 201, 实际为: %d, 返回: %+v", ctx.resCode, ctx.resBody)
		}
	})

	t.Run("2. 注册接口参数校验：用户名过短拦截 (@min=3)", func(t *testing.T) {
		h := handlers["POST /auth/register"]
		reqBody := `{"username":"ab","email":"alex@example.com","mobile":"13800138000","password":"secure_password"}`
		ctx := NewTestContext(httptest.NewRequest("POST", "/auth/register", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "Register"})

		if ctx.resCode != 400 {
			t.Fatalf("期望状态码 400, 实际为: %d", ctx.resCode)
		}
	})

	t.Run("3. 注册接口参数校验：邮箱格式错误拦截 (@email)", func(t *testing.T) {
		h := handlers["POST /auth/register"]
		reqBody := `{"username":"alex","email":"invalid_email","mobile":"13800138000","password":"secure_password"}`
		ctx := NewTestContext(httptest.NewRequest("POST", "/auth/register", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "Register"})

		if ctx.resCode != 400 {
			t.Fatalf("期望状态码 400, 实际为: %d", ctx.resCode)
		}
	})

	t.Run("4. 登录接口：@customBind 与 @customValidate 接管处理", func(t *testing.T) {
		h := handlers["POST /auth/login"]
		req := httptest.NewRequest("POST", "/auth/login?username=myuser&password=123", nil)
		ctx := NewTestContext(req)

		h(ctx, resolver.MethodInfo{Name: "Login"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		res := ctx.resBody.(resolver.ResData)
		token, ok := res.Data.(*resolver.Token)
		if !ok || token.Token != "mock-jwt-token-123456" {
			t.Fatalf("登录返回 Token 不匹配: %+v", res.Data)
		}
	})

	t.Run("5. 登录接口：@customValidate 校验失败拦截", func(t *testing.T) {
		h := handlers["POST /auth/login"]
		req := httptest.NewRequest("POST", "/auth/login", nil)
		ctx := NewTestContext(req)

		h(ctx, resolver.MethodInfo{Name: "Login"})

		if ctx.resCode != 400 {
			t.Fatalf("期望状态码 400, 实际为: %d", ctx.resCode)
		}
	})

	t.Run("6. 三阶段装饰器全链路：loginRequired(request) + checkOwner(invoke) + maskEmail(response)", func(t *testing.T) {
		h := handlers["POST /auth/update"]
		reqBody := `{"id":1,"email":"secret_user@domain.com"}`
		ctx := NewTestContext(httptest.NewRequest("POST", "/auth/update", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "UpdateUser"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		if !biz.checkOwnerCalled {
			t.Fatalf("特化调用前装饰器 checkOwner 未被触发")
		}
		if !biz.maskEmailCalled {
			t.Fatalf("特化响应后装饰器 maskEmail 未被触发")
		}
		res := ctx.resBody.(resolver.ResData)
		user, ok := res.Data.(*resolver.User)
		if !ok || user.Email != "u***@resgen.dev" {
			t.Fatalf("期望邮箱被脱敏为 u***@resgen.dev，实际为: %s", user.Email)
		}
	})

	t.Run("7. 特化装饰器权限拦截：checkOwner 校验失败", func(t *testing.T) {
		h := handlers["POST /auth/update"]
		reqBody := `{"id":999,"email":"other_user@domain.com"}`
		ctx := NewTestContext(httptest.NewRequest("POST", "/auth/update", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "UpdateUser"})

		if ctx.resCode != 400 {
			t.Fatalf("期望状态码 400 (checkOwner 拦截), 实际为: %d", ctx.resCode)
		}
	})

	t.Run("9. 跨字段关联校验：@timeBefore 成功 (startTime < endTime)", func(t *testing.T) {
		h := handlers["POST /auth/period"]
		now := time.Now()
		st := now.Unix()
		et := now.Add(time.Hour).Unix()
		reqBody := fmt.Sprintf(`{"start_time":%d,"end_time":%d}`, st, et)
		ctx := NewTestContext(httptest.NewRequest("POST", "/auth/period", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "SetPeriod"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
	})

	t.Run("10. 跨字段关联校验：@timeBefore 拦截 (startTime > endTime)", func(t *testing.T) {
		h := handlers["POST /auth/period"]
		now := time.Now()
		st := now.Add(time.Hour).Unix()
		et := now.Unix()
		reqBody := fmt.Sprintf(`{"start_time":%d,"end_time":%d}`, st, et)
		ctx := NewTestContext(httptest.NewRequest("POST", "/auth/period", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "SetPeriod"})

		if ctx.resCode != 400 {
			t.Fatalf("期望状态码 400 (timeBefore 跨字段校验失败), 实际为: %d", ctx.resCode)
		}
		res, ok := ctx.resBody.(resolver.ResData)
		if !ok || res.Msg == "" {
			t.Fatalf("期望返回错误信息，实际为: %v", ctx.resBody)
		}
	})
}
