package generator

import (
	"fmt"
	"strings"
)

// parseGoType extracts array and pointer modifiers from a Go type string
func parseGoType(goType string) (isArray, isPointer, isElementPointer bool, baseType string) {
	if strings.HasPrefix(goType, "*") {
		isPointer = true
		goType = goType[1:]
	}
	if strings.HasPrefix(goType, "[]") {
		isArray = true
		goType = goType[2:]
	}
	if strings.HasPrefix(goType, "*") {
		if isArray {
			isElementPointer = true
		} else {
			isPointer = true
		}
		goType = goType[1:]
	}
	baseType = goType
	return
}

func hasAnyValidation(validators, itemValidators []MetaInfo, isEnum bool, refModel *ModelInfo) bool {
	if len(validators) > 0 || len(itemValidators) > 0 || isEnum {
		return true
	}
	if refModel != nil {
		for _, f := range refModel.Fields {
			if hasAnyValidation(f.Validators, f.ItemValidators, f.IsEnum, f.RefModel) {
				return true
			}
		}
	}
	return false
}

func generateValidationCode(method *MethodInfo) string {
	var sb strings.Builder
	
	var walk func(validators, itemValidators []MetaInfo, accessor, jsonPath, goType string, isEnum bool, refModel *ModelInfo, indent string)
	walk = func(validators, itemValidators []MetaInfo, accessor, jsonPath, goType string, isEnum bool, refModel *ModelInfo, indent string) {
		if !hasAnyValidation(validators, itemValidators, isEnum, refModel) {
			return
		}

		isArray, isPointer, isElementPointer, baseType := parseGoType(goType)

		// 1. Process array
		if isArray {
			callAccessor := accessor
			innerIndent := indent
			if isPointer {
				sb.WriteString(fmt.Sprintf("%sif %s != nil {\n", indent, accessor))
				innerIndent += "\t"
				callAccessor = "*" + accessor
			}

			// 数组本身的校验器（如 ArrNotNull 带来的 Required、切片长度 MinLen/MaxLen 等）
			for _, v := range validators {
				var vArgs []string
				for _, a := range v.Args {
					vArgs = append(vArgs, a.GoValue)
				}
				argsStr := strings.Join(vArgs, ", ")
				if argsStr != "" {
					argsStr = ", " + argsStr
				}
				sb.WriteString(fmt.Sprintf("%sif err := e.v.%s(ctx, %s, %s%s); err != nil { return err }\n", innerIndent, v.Name, jsonPath, callAccessor, argsStr))
			}

			// 数组元素的校验（遍历切片）
			if hasAnyValidation(itemValidators, nil, isEnum, refModel) {
				rangeTarget := accessor
				if isPointer {
					rangeTarget = "*" + accessor
				}
				sb.WriteString(fmt.Sprintf("%sfor i, item := range %s {\n", innerIndent, rangeTarget))
				
				itemAccessor := "item"
				itemJSONPath := fmt.Sprintf("%s + \"[\" + strconv.Itoa(i) + \"]\"", jsonPath)
				if isElementPointer {
					walk(itemValidators, nil, itemAccessor, itemJSONPath, "*"+baseType, isEnum, refModel, innerIndent+"\t")
				} else {
					walk(itemValidators, nil, itemAccessor, itemJSONPath, baseType, isEnum, refModel, innerIndent+"\t")
				}
				
				sb.WriteString(fmt.Sprintf("%s}\n", innerIndent))
			}
			
			if isPointer {
				sb.WriteString(fmt.Sprintf("%s}\n", indent))
			}
			return
		}

		// 2. Process single field (pointer or value)
		callAccessor := accessor
		innerIndent := indent
		if isPointer {
			sb.WriteString(fmt.Sprintf("%sif %s != nil {\n", indent, accessor))
			innerIndent += "\t"
			callAccessor = "*" + accessor
		}

		for _, v := range validators {
			var vArgs []string
			for _, a := range v.Args {
				// Replace "input." with the actual parent accessor if necessary, 
				// but currently args.GoValue might just be static or simple.
				// We keep it as is.
				vArgs = append(vArgs, a.GoValue)
			}
			argsStr := strings.Join(vArgs, ", ")
			if argsStr != "" {
				argsStr = ", " + argsStr
			}
			
			// OmitEmpty logic for string
			if baseType == "string" && v.Name != "Required" && isPointer {
				sb.WriteString(fmt.Sprintf("%sif %s != \"\" {\n", innerIndent, callAccessor))
				sb.WriteString(fmt.Sprintf("%s\tif err := e.v.%s(ctx, %s, %s%s); err != nil { return err }\n", innerIndent, v.Name, jsonPath, callAccessor, argsStr))
				sb.WriteString(fmt.Sprintf("%s}\n", innerIndent))
			} else {
				sb.WriteString(fmt.Sprintf("%sif err := e.v.%s(ctx, %s, %s%s); err != nil { return err }\n", innerIndent, v.Name, jsonPath, callAccessor, argsStr))
			}
		}

		if isEnum {
			// For IsValid(), we can just use the accessor (which might be a pointer) because Go automatically dereferences it.
			// Or we can safely wrap it in parentheses if we use callAccessor: (%s).IsValid()
			sb.WriteString(fmt.Sprintf("%sif !(%s).IsValid() { return e.v.EnumError(ctx, %s, \"%s\", %s) }\n", innerIndent, callAccessor, jsonPath, baseType, callAccessor))
		}

		if refModel != nil {
			for _, field := range refModel.Fields {
				fieldAccessor := fmt.Sprintf("%s.%s", accessor, field.Name)
				fieldJSONPath := fmt.Sprintf("%s + \".%s\"", jsonPath, field.JSONName)
				walk(field.Validators, field.ItemValidators, fieldAccessor, fieldJSONPath, field.GoType, field.IsEnum, field.RefModel, innerIndent)
			}
		}

		if isPointer {
			sb.WriteString(fmt.Sprintf("%s}\n", indent))
		}
	}

	if method.IsArgsWrapped {
		for _, arg := range method.Args {
			walk(arg.Validators, arg.ItemValidators, "input."+arg.GoName, `"`+arg.Name+`"`, arg.GoType, arg.IsEnum, arg.RefModel, "\t")
		}
	} else if len(method.Args) > 0 {
		inputModel := method.Args[0].RefModel
		if inputModel != nil {
			for _, field := range inputModel.Fields {
				walk(field.Validators, field.ItemValidators, "input."+field.Name, `"`+field.JSONName+`"`, field.GoType, field.IsEnum, field.RefModel, "\t")
			}
		}
	}

	return sb.String()
}
