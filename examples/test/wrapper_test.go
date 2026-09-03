package test

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/xslasd/resgen/examples/resolver"
)

type mockWrapperDemoBiz struct{}

func (b *mockWrapperDemoBiz) GetArticle(ctx context.Context, id *int) (*resolver.Article, error) {
	return &resolver.Article{ID: *id, Title: "Resgen 设计指南", Content: "DSL 规范与高效生成"}, nil
}

func (b *mockWrapperDemoBiz) ListArticles(ctx context.Context, input *resolver.ListArticlesArgs) (*[]*resolver.Article, error) {
	list := []*resolver.Article{
		{ID: 1, Title: "文章1", Content: "内容1"},
		{ID: 2, Title: "文章2", Content: "内容2"},
	}
	return &list, nil
}

func (b *mockWrapperDemoBiz) ListArticlesV2(ctx context.Context, input *resolver.ListArticlesV2Args) (*resolver.ListResArticle, error) {
	list := []resolver.Article{
		{ID: 10, Title: "分页文章1", Content: "分页内容1"},
		{ID: 20, Title: "分页文章2", Content: "分页内容2"},
	}
	return &resolver.ListResArticle{
		Rows:  list,
		Total: 2,
	}, nil
}

func (b *mockWrapperDemoBiz) GetCategoryTree(ctx context.Context) (*resolver.TreeResCategoryTreeNode, error) {
	child := resolver.CategoryTreeNode{ID: 101, ParentId: 1, Name: "Go微服务", Sort: 1}
	root := resolver.CategoryTreeNode{
		ID:       1,
		ParentId: 0,
		Name:     "技术架构",
		Sort:     1,
		Children: &[]resolver.CategoryTreeNode{child},
	}
	return &resolver.TreeResCategoryTreeNode{
		Items: []resolver.CategoryTreeNode{root},
		Total: 2,
	}, nil
}

func (b *mockWrapperDemoBiz) GetCategoryTreeRaw(ctx context.Context) (*resolver.CategoryTreeNode, error) {
	return &resolver.CategoryTreeNode{
		ID:       1,
		ParentId: 0,
		Name:     "裸树节点",
		Sort:     1,
	}, nil
}

func (b *mockWrapperDemoBiz) CreateArticle(ctx context.Context, input *resolver.CreateArticleArgs) (*resolver.Article, error) {
	return &resolver.Article{ID: 100, Title: input.Title, Content: input.Content}, nil
}

func (b *mockWrapperDemoBiz) GetArticleRaw(ctx context.Context, id *int) (*resolver.Article, error) {
	return &resolver.Article{ID: *id, Title: "裸文章", Content: "不带包装器"}, nil
}

func (b *mockWrapperDemoBiz) Logout(ctx context.Context) error {
	return nil
}

func setupWrapperDemoHandlers() map[string]resolver.HandlerFunc[*TestServerContext] {
	handlers := make(map[string]resolver.HandlerFunc[*TestServerContext])
	en := resolver.NewEngine[*TestServerContext]().
		BindResponder(&TestResponder{}).
		BindValidator(&TestValidator{}).
		BindDecorator(&TestDecorator{}).
		BindRegister(func(e *resolver.Engine[*TestServerContext], info resolver.MethodInfo, handler resolver.HandlerFunc[*TestServerContext]) {
			key := fmt.Sprintf("%s %s", info.Method, info.Path)
			handlers[key] = handler
		})

	resolver.MountWrapperDemo[any, *TestServerContext](en, &mockWrapperDemoBiz{})
	return handlers
}

func TestWrapperDemo_Endpoints(t *testing.T) {
	handlers := setupWrapperDemoHandlers()

	t.Run("1. GET /articles/:id 返回 ResData<Article>", func(t *testing.T) {
		h := handlers["GET /articles/:id"]
		ctx := NewTestContext(httptest.NewRequest("GET", "/articles/42", nil))
		ctx.pathParams["id"] = "42"

		h(ctx, resolver.MethodInfo{Name: "GetArticle"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		res, ok := ctx.resBody.(resolver.ResData)
		if !ok || res.Code != 200 {
			t.Fatalf("响应数据不匹配: %+v", ctx.resBody)
		}
		art, ok := res.Data.(*resolver.Article)
		if !ok || art.ID != 42 || art.Title != "Resgen 设计指南" {
			t.Fatalf("文章数据解析不匹配: %+v", art)
		}
	})

	t.Run("2. GET /articles/list 返回 ResData<[Article]>", func(t *testing.T) {
		h := handlers["GET /articles/list"]
		ctx := NewTestContext(httptest.NewRequest("GET", "/articles/list", nil))

		h(ctx, resolver.MethodInfo{Name: "ListArticles"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		res := ctx.resBody.(resolver.ResData)
		list, ok := res.Data.(*[]*resolver.Article)
		if !ok || len(*list) != 2 {
			t.Fatalf("列表数据不匹配: %+v", res.Data)
		}
	})

	t.Run("3. GET /articles/list/v2 返回 ResData<ListRes<Article>>", func(t *testing.T) {
		h := handlers["GET /articles/list/v2"]
		ctx := NewTestContext(httptest.NewRequest("GET", "/articles/list/v2", nil))

		h(ctx, resolver.MethodInfo{Name: "ListArticlesV2"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		res := ctx.resBody.(resolver.ResData)
		listRes, ok := res.Data.(*resolver.ListResArticle)
		if !ok || listRes.Total != 2 || len(listRes.Rows) != 2 {
			t.Fatalf("分页列表数据不匹配: %+v", res.Data)
		}
	})

	t.Run("4. GET /articles/categories/tree 返回 ResData<TreeRes<CategoryTreeNode>>", func(t *testing.T) {
		h := handlers["GET /articles/categories/tree"]
		ctx := NewTestContext(httptest.NewRequest("GET", "/articles/categories/tree", nil))

		h(ctx, resolver.MethodInfo{Name: "GetCategoryTree"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		res := ctx.resBody.(resolver.ResData)
		treeRes, ok := res.Data.(*resolver.TreeResCategoryTreeNode)
		if !ok || treeRes.Total != 2 {
			t.Fatalf("树结构数据不匹配: %+v", res.Data)
		}
		if (*treeRes.Items[0].Children)[0].Name != "Go微服务" {
			t.Fatalf("子树节点名称不匹配")
		}
	})

	t.Run("5. GET /articles/categories/tree/raw 裸返回树节点 (wrap=none)", func(t *testing.T) {
		h := handlers["GET /articles/categories/tree/raw"]
		ctx := NewTestContext(httptest.NewRequest("GET", "/articles/categories/tree/raw", nil))

		h(ctx, resolver.MethodInfo{Name: "GetCategoryTreeRaw"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		treeRes, ok := ctx.resBody.(resolver.TreeRes)
		if !ok || len(treeRes.Items) != 1 {
			t.Fatalf("TreeRes 包装器返回不匹配: %+v", ctx.resBody)
		}
	})

	t.Run("6. POST /articles/create 创建成功返回 state=201", func(t *testing.T) {
		h := handlers["POST /articles/create"]
		reqBody := `{"title":"新文章","content":"新内容"}`
		ctx := NewTestContext(httptest.NewRequest("POST", "/articles/create", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "CreateArticle"})

		if ctx.resCode != 201 {
			t.Fatalf("期望状态码 201, 实际为: %d", ctx.resCode)
		}
	})

	t.Run("7. GET /articles/raw/:id 返回原始对象 Article (wrap=none)", func(t *testing.T) {
		h := handlers["GET /articles/raw/:id"]
		ctx := NewTestContext(httptest.NewRequest("GET", "/articles/raw/99", nil))
		ctx.pathParams["id"] = "99"

		h(ctx, resolver.MethodInfo{Name: "GetArticleRaw"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		art, ok := ctx.resBody.(*resolver.Article)
		if !ok || art.ID != 99 || art.Title != "裸文章" {
			t.Fatalf("原始对象返回不匹配: %+v", ctx.resBody)
		}
	})

	t.Run("8. POST /articles/logout 无返回值接口", func(t *testing.T) {
		h := handlers["POST /articles/logout"]
		ctx := NewTestContext(httptest.NewRequest("POST", "/articles/logout", nil))

		h(ctx, resolver.MethodInfo{Name: "Logout"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
	})
}
