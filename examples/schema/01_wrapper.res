# 示例 1：响应包装器的使用
# 展示 wrap 泛型包装器的定义，以及在接口中使用 ResData<T> 的各种方式
module WrapperDemo

# 数据模型
type Article {
    id: Int!
    title: String!
    content: String!
}

type PageResult<T> {
    list: T!
    total: Int!
    page: Int!
    size: Int!
}

group /articles [wrap=ResData] {
    # 返回单个对象：ResData<Article>
    GET /:id => GetArticle(id: Int @path): ResData<Article>
    # 返回列表：ResData<[Article]>
    GET /list => ListArticles(page: Int, size: Int): ResData<[Article]>
    # 返回通用列表/分页响应：ResData<ListRes<Article>>
    GET /list/v2 => ListArticlesV2(page: Int, size: Int): ResData<ListRes<Article>>
    # 创建成功返回 201，数据包裹在 ResData 中
    POST /create => CreateArticle(title: String!, content: String!): ResData<Article> [state=201]
    # 不使用 ResData 包装，直接返回原始对象（接口级覆盖组级 wrap）
    GET /raw/:id => GetArticleRaw(id: Int @path): Article [wrap=none]
    # 无返回值接口测试
    POST /logout => Logout()
}
