package main

import (
	"context"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/xslasd/resgen/examples/single_payload/resolver"
)

// --- 1. 实现 ServerContext 适配器 ---
type GinContext struct {
	GC *gin.Context
}

func (c *GinContext) Bind(gc *gin.Context) resolver.ServerContext[*gin.Context] {
	return &GinContext{GC: gc}
}
func (c *GinContext) Native() *gin.Context                       { return c.GC }
func (c *GinContext) GetPath(name string) string                 { return c.GC.Param(name) }
func (c *GinContext) GetQuery(name string) string                { return c.GC.Query(name) }
func (c *GinContext) GetHeader(name string) string               { return c.GC.GetHeader(name) }
func (c *GinContext) RenderJson(code int, obj any)               { c.GC.JSON(code, obj) }
func (c *GinContext) RenderText(code int, obj any)               { c.GC.String(code, "%v", obj) }
func (c *GinContext) RenderRaw(code int, ctype string, b []byte) { c.GC.Data(code, ctype, b) }
func (c *GinContext) RenderXml(code int, obj any)                { c.GC.XML(code, obj) }
func (c *GinContext) SetHeader(name, value string)               { c.GC.Header(name, value) }
func (c *GinContext) Context() context.Context                   { return c.GC.Request.Context() }

func (c *GinContext) Payload(source resolver.BodySource, dest any) error {
	switch string(source) {
	case "json", "application/json":
		return c.GC.ShouldBindJSON(dest)
	default:
		return c.GC.ShouldBind(dest)
	}
}

func (c *GinContext) Field(source resolver.BodySource, name string, dest any) error {
	if d, ok := dest.(*string); ok {
		*d = c.GC.PostForm(name)
	}
	return nil
}

// --- 2. 实现 Responder 契约 ---
type MyResponder struct{}

func (r *MyResponder) ErrorToStatus(ctx *gin.Context, err error) int {
	if err == nil {
		return 200
	}
	return 500
}

func (r *MyResponder) BindRes(ctx *gin.Context, data any, err error) resolver.Res {
	res := resolver.Res{
		Code: 200,
		Msg:  "success",
	}
	if err != nil {
		res.Code = 500
		res.Msg = err.Error()
	} else {
		res.Data = data
	}
	return res
}

// --- 3. 实现业务逻辑 Resolver ---
type MyResolver struct{}

func (r *MyResolver) SaveItems(ctx context.Context, input *[]resolver.Item) (int, error) {
	// 单参数 Payload 直接作为顶层结构传入
	fmt.Printf("Received items: %+v\n", *input)
	return len(*input), nil
}

func (r *MyResolver) ProcessRaw(ctx context.Context, input *any) (any, error) {
	// 单个 Any 类型的 Payload 直接作为顶层结构传入
	fmt.Printf("Received raw data: %+v\n", *input)
	return map[string]any{"processed": true, "original": *input}, nil
}

func main() {
	r := gin.Default()

	// 初始化 Resgen Engine
	en := resolver.NewEngine[*GinContext]()
	en.BindResponder(&MyResponder{})

	// 注册 Gin 路由挂载钩子
	en.BindRegister(func(e *resolver.Engine[*GinContext], info resolver.MethodInfo, handler resolver.HandlerFunc[*GinContext]) {
		r.Handle(info.Method, info.Path, func(c *gin.Context) {
			adapter := &GinContext{GC: c}
			handler(adapter, info)
		})
	})

	// 注入业务实现并挂载
	resolver.MountSinglePayloadExample(en, &MyResolver{})

	fmt.Println("Server is running on http://localhost:8080")
	fmt.Println("Test SaveItems with: curl -X POST http://localhost:8080/api/items -d '{\"items\":[{\"id\":1,\"name\":\"Apple\"}]}' -H \"Content-Type: application/json\"")
	
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
