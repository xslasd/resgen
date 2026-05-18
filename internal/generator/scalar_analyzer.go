package generator

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"golang.org/x/tools/go/packages"
)

// ScalarAnalysisResult 存放标量 AST 静态推导和校验的物理结果
type ScalarAnalysisResult struct {
	TargetType string // 物理目标类型，如 "time.Time" 或 "any"
	ImportPath string // 如果目标类型需要导入，返回对应的包路径，如 "time"
}

// AnalyzeScalar 利用 golang.org/x/tools/go/packages 动态分析并校验自定义标量符号
// 如果加载包失败，为确保离线 CI/CD 编译的鲁棒性，体面退化返回 nil, nil 而不直接崩溃
func AnalyzeScalar(modelPath string, dslBaseType string) (*ScalarAnalysisResult, error) {
	// 1. 解析物理完整路径，提取包路径和具名类型
	// 期望格式: "github.com/xslasd/resgen/examples/scalars.IntTime"
	lastDot := strings.LastIndex(modelPath, ".")
	if lastDot == -1 {
		return nil, fmt.Errorf("标量物理实现路径格式不正确。期望包含类型名称，例如: github.com/xxx/scalars.MyScalar")
	}
	pkgPath := modelPath[:lastDot]
	typeName := modelPath[lastDot+1:]

	// 2. 加载外部 Go 包符号 (包含 Syntax 与 TypesInfo 以便我们精细审查 AST 右侧表达式)
	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedSyntax,
	}
	pkgs, err := packages.Load(cfg, pkgPath)
	if err != nil || len(pkgs) == 0 {
		// 体面退化：如果没有 Go 环境或包暂不存在，跳过校验，退化到让 Go 编译期报错
		return nil, nil
	}
	if len(pkgs[0].Errors) > 0 {
		// 体面退化：如果分析时有包加载错误，跳过校验以保证容错率
		return nil, nil
	}

	pkg := pkgs[0]
	obj := pkg.Types.Scope().Lookup(typeName)
	if obj == nil {
		return nil, fmt.Errorf("在 Go 包 '%s' 中未找到自定义标量类型 '%s'", pkgPath, typeName)
	}

	// 3. 验证标量必须是具名类型 (Named Type)
	named, ok := obj.Type().(*types.Named)
	if !ok {
		return nil, fmt.Errorf("自定义标量类型 '%s' 必须是 Go 具名类型，例如: type %s time.Time", typeName, typeName)
	}

	// 4. 🌟 智能 AST 分析：定位 typeSpec 声明，精准获取定义右侧的具名类型别名本身 (解包一层)
	// 这避免了直接调用 named.Underlying() 导致穿透到底层的匿名 struct { wall uint64... } 结构体中！
	var typeSpec *ast.TypeSpec
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				tSpec, ok := spec.(*ast.TypeSpec)
				if ok && tSpec.Name.Name == typeName {
					typeSpec = tSpec
					break
				}
			}
			if typeSpec != nil {
				break
			}
		}
	}

	targetTypeStr := ""
	targetImportPath := ""

	if typeSpec != nil {
		if tInfo, ok := pkg.TypesInfo.Types[typeSpec.Type]; ok {
			directType := tInfo.Type
			switch dt := directType.(type) {
			case *types.Named:
				// 如果右侧是一个具名外部类型 (例如 time.Time)
				targetObj := dt.Obj()
				if targetObj.Pkg() != nil {
					targetImportPath = targetObj.Pkg().Path()
					targetTypeStr = targetObj.Pkg().Name() + "." + targetObj.Name()
				} else {
					targetTypeStr = targetObj.Name()
				}
			default:
				// 如果是基础类型 (如 string, int64 等)
				rawStr := directType.String()
				if rawStr == "interface{}" {
					rawStr = "any"
				}
				targetTypeStr = rawStr
			}
		}
	}

	// 如果 AST 解析由于极端宏定义失败，兜底退化至 Underlying() 提取
	if targetTypeStr == "" {
		underlyingType := named.Underlying()
		switch t := underlyingType.(type) {
		case *types.Named:
			targetObj := t.Obj()
			if targetObj.Pkg() != nil {
				targetImportPath = targetObj.Pkg().Path()
				targetTypeStr = targetObj.Pkg().Name() + "." + targetObj.Name()
			} else {
				targetTypeStr = targetObj.Name()
			}
		default:
			rawStr := underlyingType.String()
			if rawStr == "interface{}" {
				rawStr = "any"
			}
			targetTypeStr = rawStr
		}
	}

	// 5. 极致强校验：三关接口契约校验
	ptrType := types.NewPointer(named)
	goBaseType := toGoPhysicalBaseType(dslBaseType)

	// 第一关：验证 FromParam 契约 (挂在指针接收者上)
	// 期望签名: func (it *IntTime) FromParam(ctx any, s string) error
	fromParamObj, _, _ := types.LookupFieldOrMethod(ptrType, true, pkg.Types, "FromParam")
	if fromParamObj == nil {
		return nil, fmt.Errorf("标量类型 '%s' 未实现 FromParam 契约方法！\n  期望的签名: func (it *%s) FromParam(ctx any, s string) error", typeName, typeName)
	}
	fromParamFunc, ok := fromParamObj.(*types.Func)
	if !ok {
		return nil, fmt.Errorf("标量 '%s' 上的 FromParam 必须是一个契约方法", typeName)
	}
	fromParamSig := fromParamFunc.Type().(*types.Signature)
	if fromParamSig.Params().Len() != 2 || fromParamSig.Results().Len() != 1 {
		return nil, fmt.Errorf("标量 '%s' 的 FromParam 方法参数或返回值个数不符合契约！\n  期望的签名: func (it *%s) FromParam(ctx any, s string) error", typeName, typeName)
	}
	if !isErrorType(fromParamSig.Results().At(0).Type()) {
		return nil, fmt.Errorf("标量 '%s' 的 FromParam 方法的返回值必须是 error 类型！", typeName)
	}

	// 第二关：验证 FromValue 契约 (挂在指针接收者上)
	// 期望签名: func (it *IntTime) FromValue(ctx any, v BaseType) error
	fromValueObj, _, _ := types.LookupFieldOrMethod(ptrType, true, pkg.Types, "FromValue")
	if fromValueObj == nil {
		return nil, fmt.Errorf("标量类型 '%s' 未实现 FromValue 契约方法！\n  期望的签名: func (it *%s) FromValue(ctx any, v %s) error", typeName, typeName, goBaseType)
	}
	fromValueFunc := fromValueObj.(*types.Func)
	fromValueSig := fromValueFunc.Type().(*types.Signature)
	if fromValueSig.Params().Len() != 2 || fromValueSig.Results().Len() != 1 {
		return nil, fmt.Errorf("标量 '%s' 的 FromValue 方法参数或返回值个数不符合契约！\n  期望的签名: func (it *%s) FromValue(ctx any, v %s) error", typeName, typeName, goBaseType)
	}
	// 校验反序列化值类型匹配度
	valTypeStr := fromValueSig.Params().At(1).Type().String()
	if valTypeStr == "interface{}" {
		valTypeStr = "any"
	}
	if !strings.HasSuffix(valTypeStr, goBaseType) {
		return nil, fmt.Errorf("标量 '%s' 的 FromValue 方法的第二个参数类型为 '%s'，与 DSL 中定义的基类物理类型 '%s' 不匹配！", typeName, valTypeStr, goBaseType)
	}
	if !isErrorType(fromValueSig.Results().At(0).Type()) {
		return nil, fmt.Errorf("标量 '%s' 的 FromValue 方法的返回值必须是 error 类型！", typeName)
	}

	// 第三关：验证 ToValue 契约 (值接收者或指针接收者均可)
	// 期望签名: func (it IntTime) ToValue(ctx any) (BaseType, error)
	toValueObj, _, _ := types.LookupFieldOrMethod(ptrType, true, pkg.Types, "ToValue")
	if toValueObj == nil {
		return nil, fmt.Errorf("标量类型 '%s' 未实现 ToValue 契约方法！\n  期望的签名: func (it %s) ToValue(ctx any) (%s, error)", typeName, typeName, goBaseType)
	}
	toValueFunc := toValueObj.(*types.Func)
	toValueSig := toValueFunc.Type().(*types.Signature)
	if toValueSig.Params().Len() != 1 || toValueSig.Results().Len() != 2 {
		return nil, fmt.Errorf("标量 '%s' 的 ToValue 方法参数或返回值个数不符合契约！\n  期望的签名: func (it %s) ToValue(ctx any) (%s, error)", typeName, typeName, goBaseType)
	}
	retTypeStr := toValueSig.Results().At(0).Type().String()
	if retTypeStr == "interface{}" {
		retTypeStr = "any"
	}
	if !strings.HasSuffix(retTypeStr, goBaseType) {
		return nil, fmt.Errorf("标量 '%s' 的 ToValue 方法的第一个返回值类型为 '%s'，与 DSL 中定义的基类物理类型 '%s' 不匹配！", typeName, retTypeStr, goBaseType)
	}
	if !isErrorType(toValueSig.Results().At(1).Type()) {
		return nil, fmt.Errorf("标量 '%s' 的 ToValue 方法的第二个返回值必须是 error 类型！", typeName)
	}

	return &ScalarAnalysisResult{
		TargetType: targetTypeStr,
		ImportPath: targetImportPath,
	}, nil
}

// isErrorType 判断类型是否为 error 接口
func isErrorType(t types.Type) bool {
	return t.String() == "error"
}

// toGoPhysicalBaseType 将 DSL 中的标量 BaseType 翻译为 Go 物理基础类型
func toGoPhysicalBaseType(dslType string) string {
	switch strings.ToLower(dslType) {
	case "int", "int64":
		return "int64"
	case "float", "float64":
		return "float64"
	case "string":
		return "string"
	case "bool", "boolean":
		return "bool"
	case "any":
		return "any"
	default:
		return dslType
	}
}
