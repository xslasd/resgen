package test

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/xslasd/resgen/examples/resolver"
)

type mockTaskDemoBiz struct {
	lastCancelTaskID  int
	lastUpdateName    string
	lastPatchStatus   int
	lastDeleteTaskID  int
	lastTriggerTaskID int
	lastDeleteItemID  string
	lastResetItemID   int
}

func (b *mockTaskDemoBiz) CancelTask(ctx context.Context, input *resolver.CancelTaskArgs) (*bool, error) {
	b.lastCancelTaskID = input.TaskID
	res := true
	return &res, nil
}

func (b *mockTaskDemoBiz) UpdateName(ctx context.Context, input *resolver.UpdateNameArgs) (*bool, error) {
	b.lastUpdateName = input.NewName
	res := true
	return &res, nil
}

func (b *mockTaskDemoBiz) PatchStatus(ctx context.Context, input *resolver.PatchStatusArgs) (*bool, error) {
	b.lastPatchStatus = input.Status
	res := true
	return &res, nil
}

func (b *mockTaskDemoBiz) DeleteTask(ctx context.Context, input *resolver.DeleteTaskArgs) (*bool, error) {
	b.lastDeleteTaskID = input.TaskID
	res := true
	return &res, nil
}

func (b *mockTaskDemoBiz) TriggerTask(ctx context.Context, task_id int) (*bool, error) {
	b.lastTriggerTaskID = task_id
	res := true
	return &res, nil
}

func (b *mockTaskDemoBiz) DeleteItem(ctx context.Context, id string) (*bool, error) {
	b.lastDeleteItemID = id
	res := true
	return &res, nil
}

func (b *mockTaskDemoBiz) ResetItem(ctx context.Context, id int) (*bool, error) {
	b.lastResetItemID = id
	res := true
	return &res, nil
}

func setupTaskDemoHandlers(biz *mockTaskDemoBiz) map[string]resolver.HandlerFunc[*TestServerContext] {
	handlers := make(map[string]resolver.HandlerFunc[*TestServerContext])
	en := resolver.NewEngine[*TestServerContext]().
		BindResponder(&TestResponder{}).
		BindValidator(&TestValidator{}).
		BindDecorator(&TestDecorator{}).
		BindRegister(func(e *resolver.Engine[*TestServerContext], info resolver.MethodInfo, handler resolver.HandlerFunc[*TestServerContext]) {
			key := fmt.Sprintf("%s %s", info.Method, info.Path)
			handlers[key] = handler
		})

	resolver.MountTaskDemo[any, *TestServerContext](en, biz)
	return handlers
}

func TestTaskDemo_SingleScalarBodyAndQueryHandling(t *testing.T) {
	biz := &mockTaskDemoBiz{}
	handlers := setupTaskDemoHandlers(biz)

	t.Run("1. POST /tasks/cancel: 单标量 Body 自动反序列化成功", func(t *testing.T) {
		h, ok := handlers["POST /tasks/cancel"]
		if !ok {
			t.Fatalf("未找到路由处理函数 POST /tasks/cancel")
		}
		reqBody := `{"task_id": 10086}`
		ctx := NewTestContext(httptest.NewRequest("POST", "/tasks/cancel", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "CancelTask"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		if biz.lastCancelTaskID != 10086 {
			t.Fatalf("期望 task_id 为 10086, 实际业务接收到: %d", biz.lastCancelTaskID)
		}
	})

	t.Run("2. PUT /tasks/name: 单标量 Body 自动反序列化成功", func(t *testing.T) {
		h, ok := handlers["PUT /tasks/name"]
		if !ok {
			t.Fatalf("未找到路由处理函数 PUT /tasks/name")
		}
		reqBody := `{"new_name": "核心自动化任务"}`
		ctx := NewTestContext(httptest.NewRequest("PUT", "/tasks/name", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "UpdateName"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		if biz.lastUpdateName != "核心自动化任务" {
			t.Fatalf("期望 new_name 为 '核心自动化任务', 实际为: %s", biz.lastUpdateName)
		}
	})

	t.Run("3. PATCH /tasks/status: 单标量 Body 自动反序列化成功", func(t *testing.T) {
		h, ok := handlers["PATCH /tasks/status"]
		if !ok {
			t.Fatalf("未找到路由处理函数 PATCH /tasks/status")
		}
		reqBody := `{"status": 3}`
		ctx := NewTestContext(httptest.NewRequest("PATCH", "/tasks/status", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "PatchStatus"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		if biz.lastPatchStatus != 3 {
			t.Fatalf("期望 status 为 3, 实际为: %d", biz.lastPatchStatus)
		}
	})

	t.Run("4. DELETE /tasks/delete: 单标量 Body 自动反序列化成功 (与 POST/PUT/PATCH 一致)", func(t *testing.T) {
		h, ok := handlers["DELETE /tasks/delete"]
		if !ok {
			t.Fatalf("未找到路由处理函数 DELETE /tasks/delete")
		}
		reqBody := `{"task_id": 9999}`
		ctx := NewTestContext(httptest.NewRequest("DELETE", "/tasks/delete", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "DeleteTask"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		if biz.lastDeleteTaskID != 9999 {
			t.Fatalf("期望 task_id 为 9999, 实际为: %d", biz.lastDeleteTaskID)
		}
	})

	t.Run("5. POST /tasks/trigger: 显式 @query 参数从 URL 查询字符串提取", func(t *testing.T) {
		h, ok := handlers["POST /tasks/trigger"]
		if !ok {
			t.Fatalf("未找到路由处理函数 POST /tasks/trigger")
		}
		ctx := NewTestContext(httptest.NewRequest("POST", "/tasks/trigger?task_id=2026", nil))
		ctx.SetQueryParam("task_id", "2026")

		h(ctx, resolver.MethodInfo{Name: "TriggerTask"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		if biz.lastTriggerTaskID != 2026 {
			t.Fatalf("期望 task_id 为 2026, 实际为: %d", biz.lastTriggerTaskID)
		}
	})

	t.Run("6. DELETE /tasks/item: 显式 @query 参数从 URL 查询字符串提取", func(t *testing.T) {
		h, ok := handlers["DELETE /tasks/item"]
		if !ok {
			t.Fatalf("未找到路由处理函数 DELETE /tasks/item")
		}
		ctx := NewTestContext(httptest.NewRequest("DELETE", "/tasks/item?id=item-delete-001", nil))
		ctx.SetQueryParam("id", "item-delete-001")

		h(ctx, resolver.MethodInfo{Name: "DeleteItem"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		if biz.lastDeleteItemID != "item-delete-001" {
			t.Fatalf("期望 id 为 'item-delete-001', 实际为: %s", biz.lastDeleteItemID)
		}
	})

	t.Run("7. POST /tasks/reset/:id: 显式 @path 参数从 URL 路径提取", func(t *testing.T) {
		h, ok := handlers["POST /tasks/reset/:id"]
		if !ok {
			t.Fatalf("未找到路由处理函数 POST /tasks/reset/:id")
		}
		ctx := NewTestContext(httptest.NewRequest("POST", "/tasks/reset/888", nil))
		ctx.pathParams["id"] = "888"

		h(ctx, resolver.MethodInfo{Name: "ResetItem"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		if biz.lastResetItemID != 888 {
			t.Fatalf("期望 id 为 888, 实际为: %d", biz.lastResetItemID)
		}
	})
}
