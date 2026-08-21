# 示例 1：响应包装器与数据结构返回
# 展示：
# - 通用泛型包装器（ResData<T>、ListRes<T>、TreeRes<T>）
# - 单对象、数组列表、嵌套分页泛型
# - 自引用树形结构数据与 TreeRes<T> 树列表包装器
# - 接口级 wrap 覆盖组级 wrap，以及 wrap=none 裸响应
module WrapperDemo

# 数据模型：基础文章
type Article {
    id: Int!
    title: String!
    content: String!
}

# 数据模型：树形分类节点（自引用递归结构）
type CategoryTreeNode {
    id: Int!
    parentId: Int!
    name: String!
    sort: Int!
    children: [CategoryTreeNode!]
}

# 数据模型：自定义分页结果
type PageResult<T> {
    list: T!
    total: Int!
    page: Int!
    size: Int!
}

group /articles [wrap=ResData] {
    # 返回单个对象：ResData<Article>
    GET /:id => GetArticle(id: Int @path): ResData<Article>
    # 返回扁平列表：ResData<[Article]>
    GET /list => ListArticles(page: Int, size: Int): ResData<[Article]>
    # 返回通用列表/分页响应：ResData<ListRes<Article>>
    GET /list/v2 => ListArticlesV2(page: Int, size: Int): ResData<ListRes<Article>>
    # 🌟 返回树形结构数据：ResData<TreeRes<CategoryTreeNode>>
    GET /categories/tree => GetCategoryTree(): ResData<TreeRes<CategoryTreeNode>>
    # 🌟 裸返回树形结构数据（接口级 wrap=none 覆盖组级 wrap）
    GET /categories/tree/raw => GetCategoryTreeRaw(): TreeRes<CategoryTreeNode> [wrap=none]
    # 创建成功返回 201，数据包裹在 ResData 中
    POST /create => CreateArticle(title: String!, content: String!): ResData<Article> [state=201]
    # 不使用 ResData 包装，直接返回原始对象（接口级覆盖组级 wrap）
    GET /raw/:id => GetArticleRaw(id: Int @path): Article [wrap=none]
    # 无返回值接口
    POST /logout => Logout()
}
