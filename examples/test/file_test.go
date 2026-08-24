package test

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/xslasd/resgen/examples/resolver"
)

type mockFileDemoBiz struct{}

func (b *mockFileDemoBiz) UploadAvatar(ctx context.Context, input *resolver.UploadAvatarInput) (*resolver.UploadResult, error) {
	return &resolver.UploadResult{
		FileUrl:  "https://cdn.example.com/avatars/user_1.png",
		FileSize: 102400,
		MimeType: "image/png",
	}, nil
}

func (b *mockFileDemoBiz) UploadDocument(ctx context.Context, input *resolver.UploadDocumentInput) (*resolver.UploadResult, error) {
	return &resolver.UploadResult{
		FileUrl:  "https://cdn.example.com/docs/annual_report.pdf",
		FileSize: 2048000,
		MimeType: "application/pdf",
	}, nil
}

func (b *mockFileDemoBiz) DownloadFile(ctx context.Context, id *int) (*resolver.LocalFile, error) {
	return &resolver.LocalFile{
		FilePath: "/tmp/storage/sample.pdf",
		Filename: "sample.pdf",
	}, nil
}

func (b *mockFileDemoBiz) ExportCsv(ctx context.Context, ids *string) (*resolver.LocalFile, error) {
	return &resolver.LocalFile{
		FilePath: "/tmp/storage/report.csv",
		Filename: "report.csv",
	}, nil
}

func (b *mockFileDemoBiz) DownloadDynamic(ctx context.Context, id *int) (*resolver.LocalFileDownload, error) {
	return &resolver.LocalFileDownload{
		FilePath:    "/tmp/storage/dynamic.bin",
		Filename:    "dynamic.bin",
		ContentType: "application/octet-stream",
	}, nil
}

func setupFileDemoHandlers() map[string]resolver.HandlerFunc[*TestServerContext] {
	handlers := make(map[string]resolver.HandlerFunc[*TestServerContext])
	en := resolver.NewEngine[*TestServerContext]().
		BindResponder(&TestResponder{}).
		BindValidator(&TestValidator{}).
		BindDecorator(&TestDecorator{}).
		BindRegister(func(e *resolver.Engine[*TestServerContext], info resolver.MethodInfo, handler resolver.HandlerFunc[*TestServerContext]) {
			key := fmt.Sprintf("%s %s", info.Method, info.Path)
			handlers[key] = handler
		})

	resolver.MountFileDemo[any, *TestServerContext](en, &mockFileDemoBiz{})
	return handlers
}

func TestFileDemo_UploadAndRules(t *testing.T) {
	handlers := setupFileDemoHandlers()

	t.Run("1. POST /files/avatar 单文件合法上传成功", func(t *testing.T) {
		h := handlers["POST /files/avatar"]
		ctx := NewTestContext(httptest.NewRequest("POST", "/files/avatar", nil))
		ctx.formData["user_id"] = "1"
		ctx.formFiles["avatar"] = &multipart.FileHeader{
			Filename: "avatar.png",
			Size:     1024,
			Header: textproto.MIMEHeader{
				"Content-Type": []string{"image/png"},
			},
		}

		h(ctx, resolver.MethodInfo{Name: "UploadAvatar"})

		if ctx.resCode != 201 {
			t.Fatalf("期望状态码 201, 实际为: %d", ctx.resCode)
		}
		res := ctx.resBody.(resolver.ResData)
		upRes, ok := res.Data.(*resolver.UploadResult)
		if !ok || upRes.MimeType != "image/png" {
			t.Fatalf("上传返回结果解析失败: %+v", res.Data)
		}
	})

	t.Run("2. POST /files/avatar 文件过大拦截 (@fileRule maxSize=2MB)", func(t *testing.T) {
		h := handlers["POST /files/avatar"]
		ctx := NewTestContext(httptest.NewRequest("POST", "/files/avatar", nil))
		ctx.formData["user_id"] = "1"
		ctx.formFiles["avatar"] = &multipart.FileHeader{
			Filename: "huge.png",
			Size:     5 * 1024 * 1024, // 5MB > 2MB
			Header: textproto.MIMEHeader{
				"Content-Type": []string{"image/png"},
			},
		}

		h(ctx, resolver.MethodInfo{Name: "UploadAvatar"})

		if ctx.resCode != 400 {
			t.Fatalf("期望状态码 400 (超大文件拦截), 实际为: %d", ctx.resCode)
		}
	})

	t.Run("3. POST /files/avatar 文件格式不被允许拦截 (@fileRule types)", func(t *testing.T) {
		h := handlers["POST /files/avatar"]
		ctx := NewTestContext(httptest.NewRequest("POST", "/files/avatar", nil))
		ctx.formData["user_id"] = "1"
		ctx.formFiles["avatar"] = &multipart.FileHeader{
			Filename: "avatar.exe",
			Size:     1024,
			Header: textproto.MIMEHeader{
				"Content-Type": []string{"application/octet-stream"},
			},
		}

		h(ctx, resolver.MethodInfo{Name: "UploadAvatar"})

		if ctx.resCode != 400 {
			t.Fatalf("期望状态码 400 (非法类型拦截), 实际为: %d", ctx.resCode)
		}
	})

	t.Run("4. GET /files/download/:id PDF 静态已知文件下载流式响应", func(t *testing.T) {
		h := handlers["GET /files/download/:id"]
		ctx := NewTestContext(httptest.NewRequest("GET", "/files/download/10", nil))
		ctx.SetPathParam("id", "10")

		h(ctx, resolver.MethodInfo{Name: "DownloadFile"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		downloadInfo, ok := ctx.resBody.(resolver.LocalFileDownload)
		if !ok || downloadInfo.Filename != "sample.pdf" || downloadInfo.ContentType != "application/pdf" {
			t.Fatalf("文件下载载体数据异常: %+v", ctx.resBody)
		}
	})

	t.Run("5. GET /files/export/csv CSV 静态已知文件导出流式响应", func(t *testing.T) {
		h := handlers["GET /files/export/csv"]
		ctx := NewTestContext(httptest.NewRequest("GET", "/files/export/csv?ids=1,2,3", nil))

		h(ctx, resolver.MethodInfo{Name: "ExportCsv"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		downloadInfo, ok := ctx.resBody.(resolver.LocalFileDownload)
		if !ok || downloadInfo.Filename != "report.csv" || downloadInfo.ContentType != "text/csv" {
			t.Fatalf("CSV 导出载体数据异常: %+v", ctx.resBody)
		}
	})

	t.Run("6. GET /files/dynamic/:id 动态通用文件流下载", func(t *testing.T) {
		h := handlers["GET /files/dynamic/:id"]
		ctx := NewTestContext(httptest.NewRequest("GET", "/files/dynamic/10", nil))
		ctx.SetPathParam("id", "10")

		h(ctx, resolver.MethodInfo{Name: "DownloadDynamic"})

		if ctx.resCode != 200 {
			t.Fatalf("期望状态码 200, 实际为: %d", ctx.resCode)
		}
		downloadInfo, ok := ctx.resBody.(resolver.LocalFileDownload)
		if !ok || downloadInfo.Filename != "dynamic.bin" || downloadInfo.ContentType != "application/octet-stream" {
			t.Fatalf("动态文件流下载载体数据异常: %+v", ctx.resBody)
		}
	})
}
