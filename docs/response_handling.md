# Resgen 响应处理机制

> [!NOTE]
> 本文档已重构并升级为全新的模块化功能专题：
> 👉 请直接阅读：[**08. 统一响应控制与错误处理 (Response Handling & Error Contract)**](./08_response_handling.md)

---

## 核心速览

Resgen 通过强类型的包装器（Wrappers）和响应适配器（Responder）提供统一且可预测的响应结构：

1. **响应包装器 (Wrappers)**：使用 `wrap` 关键字声明 `ResData<T>`、`ListRes<T>`、`TreeRes<T>` 等外壳；
2. **响应流转契约**：严格遵循 `Handler -> ErrorToStatus -> Bind{Wrapper} -> Render` 流程，异常时数据自动置为 `nil`；
3. **关闭外壳 (`wrap=none`)**：适用于防双重套壳、纯原始流/文件下载（`File`）以及 204 No Content 空响应；
4. **嵌套泛型单态化展开 (Monomorphization)**：支持 `ResData<ListRes<User>>` 并在编译期自动生成具体结构体与强类型方法签名；
5. **多语言与结构化错误**：通过在 `Responder` 中捕获 `InvalidEnumError` 和 `EnvelopeError`，结合 `ctx` 轻松实现全局多语言错误映射。
