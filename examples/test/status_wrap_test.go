package test

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/xslasd/resgen/examples/resolver"
)

type mockStatusDemoBiz struct{}

func (b *mockStatusDemoBiz) GetProduct(ctx context.Context, id *int) (*resolver.Product, error) {
	return &resolver.Product{Id: *id, Name: "高性能键盘", Price: 599.0}, nil
}

func (b *mockStatusDemoBiz) CreateProduct(ctx context.Context, input *resolver.CreateProductInput) (*resolver.Product, error) {
	return &resolver.Product{Id: 101, Name: input.Name, Price: input.Price}, nil
}

func (b *mockStatusDemoBiz) BatchUpdate(ctx context.Context, ids []int) (*string, error) {
	msg := fmt.Sprintf("已成功提交 %d 个商品的异步更新任务", len(ids))
	return &msg, nil
}

func (b *mockStatusDemoBiz) DeleteProduct(ctx context.Context, id *int) (*string, error) {
	msg := "deleted"
	return &msg, nil
}

func (b *mockStatusDemoBiz) ListProducts(ctx context.Context, input *resolver.ListProductsArgs) ([]resolver.Product, error) {
	list := []resolver.Product{
		{Id: 1, Name: "商品1", Price: 99.0},
		{Id: 2, Name: "商品2", Price: 199.0},
	}
	return list, nil
}

func (b *mockStatusDemoBiz) GetRawProduct(ctx context.Context, id *int) (*resolver.Product, error) {
	return &resolver.Product{Id: *id, Name: "原始裸商品", Price: 88.0}, nil
}

func (b *mockStatusDemoBiz) GetRawProducts(ctx context.Context, page *int) (*[]*resolver.Product, error) {
	list := []*resolver.Product{
		{Id: 1, Name: "商品A", Price: 10.0},
	}
	return &list, nil
}

func setupStatusDemoHandlers() map[string]resolver.HandlerFunc[*TestServerContext] {
	handlers := make(map[string]resolver.HandlerFunc[*TestServerContext])
	en := resolver.NewEngine[*TestServerContext]().
		BindResponder(&TestResponder{}).
		BindValidator(&TestValidator{}).
		BindDecorator(&TestDecorator{}).
		BindRegister(func(e *resolver.Engine[*TestServerContext], info resolver.MethodInfo, handler resolver.HandlerFunc[*TestServerContext]) {
			key := fmt.Sprintf("%s %s", info.Method, info.Path)
			handlers[key] = handler
		})

	resolver.MountStatusDemo[any, *TestServerContext](en, &mockStatusDemoBiz{})
	return handlers
}

func TestStatusDemo_StatusCodesAndWrapOverrides(t *testing.T) {
	handlers := setupStatusDemoHandlers()

	t.Run("1. POST /products/create 返回 HTTP 201 Created", func(t *testing.T) {
		h := handlers["POST /products/create"]
		reqBody := `{"name":"机械键盘","price":499.0}`
		ctx := NewTestContext(httptest.NewRequest("POST", "/products/create", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "CreateProduct"})

		if ctx.resCode != 201 {
			t.Fatalf("期望状态码 201, 实际为: %d", ctx.resCode)
		}
	})

	t.Run("2. POST /products/batch-update 返回 HTTP 202 Accepted", func(t *testing.T) {
		h := handlers["POST /products/batch-update"]
		reqBody := `[1, 2, 3]`
		ctx := NewTestContext(httptest.NewRequest("POST", "/products/batch-update", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "BatchUpdate"})

		if ctx.resCode != 202 {
			t.Fatalf("期望状态码 202, 实际为: %d", ctx.resCode)
		}
	})

	t.Run("3. DELETE /products/:id 返回 HTTP 204 No Content (wrap=none)", func(t *testing.T) {
		h := handlers["DELETE /products/:id"]
		ctx := NewTestContext(httptest.NewRequest("DELETE", "/products/88", nil))
		ctx.pathParams["id"] = "88"

		h(ctx, resolver.MethodInfo{Name: "DeleteProduct"})

		if ctx.resCode != 204 {
			t.Fatalf("期望状态码 204, 实际为: %d", ctx.resCode)
		}
	})

	t.Run("4. GET /products/list 接口级覆盖为 PageData 包装器", func(t *testing.T) {
		h := handlers["GET /products/list"]
		ctx := NewTestContext(httptest.NewRequest("GET", "/products/list", nil))

		h(ctx, resolver.MethodInfo{Name: "ListProducts"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		res, ok := ctx.resBody.(resolver.PageData)
		if !ok || res.Total != 100 {
			t.Fatalf("PageData 包装器解析失败: %+v", ctx.resBody)
		}
	})

	t.Run("5. GET /raw/product/:id 裸响应 Product (wrap=none)", func(t *testing.T) {
		h := handlers["GET /raw/product/:id"]
		ctx := NewTestContext(httptest.NewRequest("GET", "/raw/product/77", nil))
		ctx.pathParams["id"] = "77"

		h(ctx, resolver.MethodInfo{Name: "GetRawProduct"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		p, ok := ctx.resBody.(*resolver.Product)
		if !ok || p.Id != 77 || p.Name != "原始裸商品" {
			t.Fatalf("裸响应对象不匹配: %+v", ctx.resBody)
		}
	})
}
