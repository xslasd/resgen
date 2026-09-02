package parser

import (
	"testing"
)

func TestParseComments(t *testing.T) {
	content := `
# 通用列表包装器
wrap ListRes<T> {
    rows: [T!]! # 列表数据
    total: Int! # 总条数
}

# 文章模型
type Article {
    # 文章ID
    # 全局唯一
    id: Int!
    title: String! # 文章标题
}

group /articles {
    # 获取文章详情
    # id: 文章主键ID
    GET /:id => GetArticle(id: Int @path): Article

    # 简单列表
    GET /list => ListArticles(): [Article] # 行尾接口注释

    # 用户登录接口
    # username: 用户名
    @loginRequired
    POST /login => Login(username: String): Token
}
`
	schema, err := ParseFileContent("test.res", content)
	if err != nil {
		t.Fatalf("ParseFileContent error: %v", err)
	}

	var foundListRes, foundArticle, foundGetArticle, foundLogin bool
	for _, decl := range schema.Declarations {
		if decl.Model != nil {
			if decl.Model.Name == "ListRes" {
				foundListRes = true
				if decl.Model.Doc != "通用列表包装器" {
					t.Errorf("ListRes.Doc = %q, want '通用列表包装器'", decl.Model.Doc)
				}
				if len(decl.Model.Properties) < 2 {
					t.Fatalf("ListRes should have 2 properties")
				}
				if decl.Model.Properties[0].TrailingDoc != "列表数据" {
					t.Errorf("ListRes.rows.TrailingDoc = %q, want '列表数据'", decl.Model.Properties[0].TrailingDoc)
				}
				if decl.Model.Properties[1].TrailingDoc != "总条数" {
					t.Errorf("ListRes.total.TrailingDoc = %q, want '总条数'", decl.Model.Properties[1].TrailingDoc)
				}
			}
			if decl.Model.Name == "Article" {
				foundArticle = true
				if decl.Model.Doc != "文章模型" {
					t.Errorf("Article.Doc = %q, want '文章模型'", decl.Model.Doc)
				}
				if decl.Model.Properties[0].Doc != "文章ID\n全局唯一" {
					t.Errorf("Article.id.Doc = %q, want '文章ID\\n全局唯一'", decl.Model.Properties[0].Doc)
				}
				if decl.Model.Properties[1].TrailingDoc != "文章标题" {
					t.Errorf("Article.title.TrailingDoc = %q, want '文章标题'", decl.Model.Properties[1].TrailingDoc)
				}
			}
		}
		if decl.Group != nil {
			for _, ep := range decl.Group.Endpoints {
				if ep.Name == "GetArticle" {
					foundGetArticle = true
					if ep.Doc != "获取文章详情" {
						t.Errorf("GetArticle.Doc = %q, want '获取文章详情'", ep.Doc)
					}
					if len(ep.Args) > 0 && ep.Args[0].Doc != "文章主键ID" {
						t.Errorf("GetArticle.id.Doc = %q, want '文章主键ID'", ep.Args[0].Doc)
					}
				}
				if ep.Name == "Login" {
					foundLogin = true
					if ep.Doc != "用户登录接口" {
						t.Errorf("Login.Doc = %q, want '用户登录接口'", ep.Doc)
					}
					if len(ep.Args) > 0 && ep.Args[0].Doc != "用户名" {
						t.Errorf("Login.username.Doc = %q, want '用户名'", ep.Args[0].Doc)
					}
				}
			}
		}
	}
	if !foundListRes || !foundArticle || !foundGetArticle || !foundLogin {
		t.Errorf("Missing expected declarations in parse result")
	}
}
