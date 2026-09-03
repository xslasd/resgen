package test

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/xslasd/resgen/examples/resolver"
)

// ==========================================
// 1. Mock 业务 Resolver 实现
// ==========================================

type MockUnionDemoBiz struct {
	LastInput      *resolver.CreatePostInput
	LastBatchInput *resolver.BatchCreatePostInput
}

func (b *MockUnionDemoBiz) GetPost(ctx context.Context, id *int) (*resolver.ContentPostItem, error) {
	if id == nil {
		return nil, fmt.Errorf("id 不能为空")
	}
	return &resolver.ContentPostItem{
		ID:   *id,
		Type: resolver.UnionKind_article,
		Payload: &resolver.UnionArticle{
			Title:   "Go 联合类型指南",
			Content: "Resgen 自动化多态联合类型绑定",
		},
	}, nil
}

func (b *MockUnionDemoBiz) CreatePost(ctx context.Context, input *resolver.CreatePostInput) (*resolver.ContentPostItem, error) {
	b.LastInput = input
	return &resolver.ContentPostItem{
		ID:      1001,
		Type:    input.Type,
		Payload: input.Payload,
	}, nil
}

func (b *MockUnionDemoBiz) BatchCreatePost(ctx context.Context, input *resolver.BatchCreatePostInput) (*resolver.BatchCreatePostResult, error) {
	b.LastBatchInput = input
	var items []resolver.ContentPostItem
	for i, it := range input.Items {
		items = append(items, resolver.ContentPostItem{
			ID:      i + 1,
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

// ==========================================
// 2. 测试集
// ==========================================

func setupUnionDemoEngine(biz *MockUnionDemoBiz) (*resolver.Engine[*TestServerContext], map[string]resolver.HandlerFunc[*TestServerContext]) {
	handlers := make(map[string]resolver.HandlerFunc[*TestServerContext])
	en := resolver.NewEngine[*TestServerContext]().
		BindResponder(&TestResponder{}).
		BindValidator(&TestValidator{}).
		BindDecorator(&TestDecorator{}).
		BindRegister(func(e *resolver.Engine[*TestServerContext], info resolver.MethodInfo, handler resolver.HandlerFunc[*TestServerContext]) {
			key := fmt.Sprintf("%s %s", info.Method, info.Path)
			handlers[key] = handler
		})

	resolver.MountUnionDemo[any, *TestServerContext](en, biz)
	return en, handlers
}

// 2.1 端到端请求流与参数校验测试
func TestUnionDemo_ParametersAndValidation(t *testing.T) {
	biz := &MockUnionDemoBiz{}
	_, handlers := setupUnionDemoEngine(biz)

	createPostHandler, ok := handlers["POST /posts/"]
	if !ok {
		t.Fatalf("未找到 POST /posts/ 的路由处理器")
	}

	getPostHandler, ok := handlers["GET /posts/:id"]
	if !ok {
		t.Fatalf("未找到 GET /posts/:id 的路由处理器")
	}

	batchCreatePostHandler, ok := handlers["POST /posts/batch"]
	if !ok {
		t.Fatalf("未找到 POST /posts/batch 的路由处理器")
	}

	t.Run("1. 正常场景：单帖创建 Article 联合类型 (强类型断言验证)", func(t *testing.T) {
		reqBody := `{"type":"article","payload":{"title":"深入理解Resgen","content":"多态绑定与性能优化"}}`
		req := httptest.NewRequest("POST", "/posts/", bytes.NewBufferString(reqBody))
		ctx := NewTestContext(req)

		createPostHandler(ctx, resolver.MethodInfo{Name: "CreatePost"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d, 返回内容: %+v", ctx.resCode, ctx.resBody)
		}

		if biz.LastInput == nil {
			t.Fatalf("业务层未收到 input")
		}
		if biz.LastInput.Type != resolver.UnionKind_article {
			t.Fatalf("期望 Type 为 article, 实际为: %v", biz.LastInput.Type)
		}

		article, ok := biz.LastInput.Payload.(*resolver.UnionArticle)
		if !ok {
			t.Fatalf("💥 失败：input.Payload 实际类型为 %T，未能断言为 *resolver.UnionArticle 单级指针！", biz.LastInput.Payload)
		}
		if article.Title != "深入理解Resgen" || article.Content != "多态绑定与性能优化" {
			t.Fatalf("文章字段解析不匹配: %+v", article)
		}
	})

	t.Run("2. 正常场景：单帖创建 Video 联合类型 (强类型断言验证)", func(t *testing.T) {
		reqBody := `{"type":"video","payload":{"title":"Resgen 演示视频","url":"https://video.com/demo.mp4","duration":360}}`
		req := httptest.NewRequest("POST", "/posts/", bytes.NewBufferString(reqBody))
		ctx := NewTestContext(req)

		createPostHandler(ctx, resolver.MethodInfo{Name: "CreatePost"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d, 返回内容: %+v", ctx.resCode, ctx.resBody)
		}

		video, ok := biz.LastInput.Payload.(*resolver.UnionVideo)
		if !ok {
			t.Fatalf("💥 失败：input.Payload 实际类型为 %T，未能断言为 *resolver.UnionVideo 单级指针！", biz.LastInput.Payload)
		}
		if video.Title != "Resgen 演示视频" || video.URL != "https://video.com/demo.mp4" || video.Duration != 360 {
			t.Fatalf("视频字段解析不匹配: %+v", video)
		}
	})

	t.Run("3. 🌟 多级结构场景：批量发布多级嵌套对象 + 切片数组中的联合类型", func(t *testing.T) {
		reqBody := `{
			"batch_name": "Go语言技术专栏",
			"pinned_post": {
				"author": "张三",
				"type": "article",
				"payload": {
					"title": "头条推荐文章",
					"content": "深度剖析多级嵌套多态架构"
				}
			},
			"items": [
				{
					"author": "李四",
					"type": "video",
					"payload": {
						"title": "Go 并发精讲视频",
						"url": "https://video.com/concurrency.mp4",
						"duration": 540
					}
				},
				{
					"author": "王五",
					"type": "article",
					"payload": {
						"title": "微服务实战经验",
						"content": "高可用服务治理设计"
					}
				}
			]
		}`
		req := httptest.NewRequest("POST", "/posts/batch", bytes.NewBufferString(reqBody))
		ctx := NewTestContext(req)

		batchCreatePostHandler(ctx, resolver.MethodInfo{Name: "BatchCreatePost"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d, 返回内容: %+v", ctx.resCode, ctx.resBody)
		}

		if biz.LastBatchInput == nil {
			t.Fatalf("业务层未收到批量 input")
		}
		if biz.LastBatchInput.BatchName != "Go语言技术专栏" {
			t.Fatalf("BatchName 不匹配: %s", biz.LastBatchInput.BatchName)
		}

		// 🌟 1. 检验单对象嵌套中的多态字段是否被成功递归转换为 *UnionArticle！
		if biz.LastBatchInput.PinnedPost == nil {
			t.Fatalf("PinnedPost 不能为空")
		}
		pinnedArticle, ok := biz.LastBatchInput.PinnedPost.Payload.(*resolver.UnionArticle)
		if !ok {
			t.Fatalf("💥 嵌套单对象 PinnedPost.Payload 未能断言为 *UnionArticle，实际类型: %T", biz.LastBatchInput.PinnedPost.Payload)
		}
		if pinnedArticle.Title != "头条推荐文章" {
			t.Fatalf("PinnedPost.Title 不匹配: %s", pinnedArticle.Title)
		}

		// 🌟 2. 检验数组切片 items[0] 中的多态字段是否被成功递归转换为 *UnionVideo！
		if len(biz.LastBatchInput.Items) != 2 {
			t.Fatalf("期望 Items 长度为 2, 实际为: %d", len(biz.LastBatchInput.Items))
		}
		item0Video, ok := biz.LastBatchInput.Items[0].Payload.(*resolver.UnionVideo)
		if !ok {
			t.Fatalf("💥 切片 items[0].Payload 未能断言为 *UnionVideo，实际类型: %T", biz.LastBatchInput.Items[0].Payload)
		}
		if item0Video.Duration != 540 || item0Video.URL != "https://video.com/concurrency.mp4" {
			t.Fatalf("items[0] 视频字段不匹配: %+v", item0Video)
		}

		// 🌟 3. 检验数组切片 items[1] 中的多态字段是否被成功递归转换为 *UnionArticle！
		item1Article, ok := biz.LastBatchInput.Items[1].Payload.(*resolver.UnionArticle)
		if !ok {
			t.Fatalf("💥 切片 items[1].Payload 未能断言为 *UnionArticle，实际类型: %T", biz.LastBatchInput.Items[1].Payload)
		}
		if item1Article.Title != "微服务实战经验" {
			t.Fatalf("items[1] 文章字段不匹配: %+v", item1Article)
		}
	})

	t.Run("4. 多级结构参数校验：嵌套切片中缺失必填项 items[0].type", func(t *testing.T) {
		reqBody := `{
			"batch_name": "测试批次",
			"items": [
				{
					"author": "李四",
					"payload": {"title": "无类型视频"}
				}
			]
		}`
		req := httptest.NewRequest("POST", "/posts/batch", bytes.NewBufferString(reqBody))
		ctx := NewTestContext(req)

		batchCreatePostHandler(ctx, resolver.MethodInfo{Name: "BatchCreatePost"})

		if ctx.resCode != 400 {
			t.Fatalf("期望状态码 400 (切片内项校验失败), 实际为: %d", ctx.resCode)
		}
	})

	t.Run("5. 多级结构参数校验：嵌套单对象中传入非法枚举 pinned_post.type='invalid'", func(t *testing.T) {
		reqBody := `{
			"batch_name": "测试批次",
			"pinned_post": {
				"author": "张三",
				"type": "invalid_type",
				"payload": {"title": "非法类型"}
			},
			"items": []
		}`
		req := httptest.NewRequest("POST", "/posts/batch", bytes.NewBufferString(reqBody))
		ctx := NewTestContext(req)

		batchCreatePostHandler(ctx, resolver.MethodInfo{Name: "BatchCreatePost"})

		if ctx.resCode != 400 {
			t.Fatalf("期望状态码 400 (嵌套对象枚举拦截), 实际为: %d", ctx.resCode)
		}
	})

	t.Run("6. 正常场景：查询帖子 GET /posts/:id", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/posts/42", nil)
		ctx := NewTestContext(req)
		ctx.pathParams["id"] = "42"

		getPostHandler(ctx, resolver.MethodInfo{Name: "GetPost"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		res, ok := ctx.resBody.(resolver.ResData)
		if !ok || res.Code != 200 {
			t.Fatalf("响应数据格式不正确: %+v", ctx.resBody)
		}
	})

	t.Run("7. 参数验证场景：缺失必填判别器 type", func(t *testing.T) {
		reqBody := `{"payload":{"title":"缺失类型"}}`
		req := httptest.NewRequest("POST", "/posts/", bytes.NewBufferString(reqBody))
		ctx := NewTestContext(req)

		createPostHandler(ctx, resolver.MethodInfo{Name: "CreatePost"})

		if ctx.resCode != 400 {
			t.Fatalf("期望状态码 400 (参数校验失败), 实际为: %d", ctx.resCode)
		}
		res, ok := ctx.resBody.(resolver.ResData)
		if !ok || res.Code != 400 {
			t.Fatalf("期望返回错误响应: %+v", ctx.resBody)
		}
	})

	t.Run("8. 参数验证场景：传入非法判别器枚举值 (type='audio')", func(t *testing.T) {
		reqBody := `{"type":"audio","payload":{"title":"音频广播"}}`
		req := httptest.NewRequest("POST", "/posts/", bytes.NewBufferString(reqBody))
		ctx := NewTestContext(req)

		createPostHandler(ctx, resolver.MethodInfo{Name: "CreatePost"})

		if ctx.resCode != 400 {
			t.Fatalf("期望状态码 400 (非法枚举校验拦截), 实际为: %d", ctx.resCode)
		}
		res, ok := ctx.resBody.(resolver.ResData)
		if !ok || res.Code != 400 {
			t.Fatalf("未返回错误消息: %+v", ctx.resBody)
		}
	})

	t.Run("9. 参数验证场景：GET 路径参数非法类型 (id='abc')", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/posts/abc", nil)
		ctx := NewTestContext(req)
		ctx.pathParams["id"] = "abc"

		getPostHandler(ctx, resolver.MethodInfo{Name: "GetPost"})

		if ctx.resCode != 400 {
			t.Fatalf("期望路径参数转换失败返回 400, 实际为: %d", ctx.resCode)
		}
	})
}

// 2.2 底层自适应解码器直接解析测试
func TestResolveContentPayload_Adaptive(t *testing.T) {
	t.Run("自适应从 Map 解析 Article", func(t *testing.T) {
		raw := map[string]any{
			"title":   "测试文章标题",
			"content": "测试文章内容详情",
		}
		res, err := resolver.ResolveContentPayload(resolver.UnionKind_article, raw)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		article, ok := res.(*resolver.UnionArticle)
		if !ok {
			t.Fatalf("期望得到 *resolver.UnionArticle 单级指针，实际得到类型: %T", res)
		}
		if article.Title != "测试文章标题" || article.Content != "测试文章内容详情" {
			t.Fatalf("字段解析不匹配: %+v", article)
		}
	})

	t.Run("自适应从 JSON 字符串解析 Video", func(t *testing.T) {
		jsonStr := `{"title":"Go进阶视频","url":"https://example.com/video.mp4","duration":120}`
		res, err := resolver.ResolveContentPayload(resolver.UnionKind_video, jsonStr)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		video, ok := res.(*resolver.UnionVideo)
		if !ok {
			t.Fatalf("期望得到 *resolver.UnionVideo 单级指针，实际得到类型: %T", res)
		}
		if video.Title != "Go进阶视频" || video.URL != "https://example.com/video.mp4" || video.Duration != 120 {
			t.Fatalf("字段解析不匹配: %+v", video)
		}
	})

	t.Run("自适应从 JSON 字节切片解析", func(t *testing.T) {
		jsonBytes := []byte(`{"title":"字节测试","content":"字节内容"}`)
		res, err := resolver.ResolveContentPayload("article", jsonBytes)
		if err != nil {
			t.Fatalf("解析失败: %v", err)
		}
		article, ok := res.(*resolver.UnionArticle)
		if !ok || article.Title != "字节测试" {
			t.Fatalf("解析失败: %+v", res)
		}
	})

	t.Run("非多态或未知类型安全回退", func(t *testing.T) {
		raw := "hello"
		res, err := resolver.ResolveContentPayload("unknown_kind", raw)
		if err != nil {
			t.Fatalf("未知类型不应报错: %v", err)
		}
		if res != "hello" {
			t.Fatalf("期望原样返回，实际为: %v", res)
		}
	})
}
