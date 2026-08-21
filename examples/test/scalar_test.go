package test

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xslasd/resgen/examples/resolver"
	"github.com/xslasd/resgen/examples/scalars"
)

type mockScalarDemoBiz struct {
	lastStartTime scalars.IntTime
	lastCreateIn  *resolver.CreateEventInput
}

func (b *mockScalarDemoBiz) GetEventByTime(ctx context.Context, startTime *scalars.IntTime) (*resolver.Event, error) {
	b.lastStartTime = *startTime
	now := scalars.IntTime(time.Unix(1700000000, 0))
	return &resolver.Event{
		Id:        1,
		Name:      "时间标量测试事件",
		StartTime: *startTime,
		EndTime:   &now,
		CreatedAt: &now,
	}, nil
}

func (b *mockScalarDemoBiz) ListEvents(ctx context.Context, input *resolver.QueryEventsInput) (*[]*resolver.Event, error) {
	now := scalars.IntTime(time.Unix(1700000000, 0))
	list := []*resolver.Event{
		{Id: 1, Name: "事件1", StartTime: now, EndTime: &now, CreatedAt: &now},
	}
	return &list, nil
}

func (b *mockScalarDemoBiz) CreateEvent(ctx context.Context, input *resolver.CreateEventInput) (*resolver.Event, error) {
	b.lastCreateIn = input
	return &resolver.Event{
		Id:        100,
		Name:      input.Name,
		StartTime: input.StartTime,
		EndTime:   &input.EndTime,
		CreatedAt: &input.StartTime,
	}, nil
}

func setupScalarDemoHandlers(biz *mockScalarDemoBiz) map[string]resolver.HandlerFunc[*TestServerContext] {
	handlers := make(map[string]resolver.HandlerFunc[*TestServerContext])
	en := resolver.NewEngine[*TestServerContext]().
		BindResponder(&TestResponder{}).
		BindValidator(&TestValidator{}).
		BindDecorator(&TestDecorator{}).
		BindRegister(func(e *resolver.Engine[*TestServerContext], info resolver.MethodInfo, handler resolver.HandlerFunc[*TestServerContext]) {
			key := fmt.Sprintf("%s %s", info.Method, info.Path)
			handlers[key] = handler
		})

	resolver.MountScalarDemo[any, *TestServerContext](en, biz)
	return handlers
}

func TestScalarDemo_Endpoints(t *testing.T) {
	biz := &mockScalarDemoBiz{}
	handlers := setupScalarDemoHandlers(biz)

	t.Run("1. GET /events/:startTime 路径参数中的标量解析 (FromParam)", func(t *testing.T) {
		h := handlers["GET /events/:startTime"]
		ctx := NewTestContext(httptest.NewRequest("GET", "/events/1700000000", nil))
		ctx.pathParams["startTime"] = "1700000000"

		h(ctx, resolver.MethodInfo{Name: "GetEventByTime"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		expectedTime := scalars.IntTime(time.Unix(1700000000, 0))
		if time.Time(biz.lastStartTime).Unix() != time.Time(expectedTime).Unix() {
			t.Fatalf("路径参数标量解析不匹配: 期望 %v, 实际 %v", expectedTime, biz.lastStartTime)
		}
	})

	t.Run("2. GET /events/list Query 参数中的标量解析", func(t *testing.T) {
		h := handlers["GET /events/list"]
		req := httptest.NewRequest("GET", "/events/list?after=1700000000&before=1700050000", nil)
		ctx := NewTestContext(req)

		h(ctx, resolver.MethodInfo{Name: "ListEvents"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
	})

	t.Run("3. POST /events/create Body 中的标量自动双向转换 (FromValue/ToDTO)", func(t *testing.T) {
		h := handlers["POST /events/create"]
		reqBody := `{"name":"技术大会","start_time":1700000000,"end_time":1700036000}`
		ctx := NewTestContext(httptest.NewRequest("POST", "/events/create", bytes.NewBufferString(reqBody)))

		h(ctx, resolver.MethodInfo{Name: "CreateEvent"})

		if ctx.resCode != 201 {
			t.Fatalf("期望状态码 201, 实际为: %d", ctx.resCode)
		}
		if biz.lastCreateIn == nil || time.Time(biz.lastCreateIn.StartTime).Unix() != 1700000000 {
			t.Fatalf("Body 中标量转换失败: %+v", biz.lastCreateIn)
		}
	})
}
