package generator

import (
	"fmt"
	"strings"
)

// parseGoType extracts array and pointer modifiers from a Go type string
func parseGoType(goType string) (isArray, isPointer, isElementPointer bool, baseType string) {
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

func generateValidationCode(method *MethodInfo) string {
	var sb strings.Builder
	
	var walk func(validators []MetaInfo, accessor, jsonPath, goType string, isEnum bool, refModel *ModelInfo, indent string)
	walk = func(validators []MetaInfo, accessor, jsonPath, goType string, isEnum bool, refModel *ModelInfo, indent string) {
		isArray, isPointer, isElementPointer, baseType := parseGoType(goType)

		// 1. Process array
		if isArray {
			if isPointer {
				sb.WriteString(fmt.Sprintf("%sif %s != nil {\n", indent, accessor))
				indent += "\t"
			}
			
			// A simple way to avoid conflicts is to base it on depth, but 'item' is usually fine if we don't nest arrays. 
			// If we do, we might need i1, item1. For simplicity, we just use i, item (resgen currently doesn't support multidimensional arrays)
			
			sb.WriteString(fmt.Sprintf("%sfor i, item := range %s {\n", indent, accessor))
			
			itemAccessor := "item"
			if isElementPointer {
				sb.WriteString(fmt.Sprintf("%s\tif item != nil {\n", indent))
				// inner block
				itemJSONPath := fmt.Sprintf("%s + \"[\" + strconv.Itoa(i) + \"]\"", jsonPath)
				// Re-call walk for the item type
				walk(validators, itemAccessor, itemJSONPath, "*"+baseType, isEnum, refModel, indent+"\t\t")
				
				sb.WriteString(fmt.Sprintf("%s\t}\n", indent))
			} else {
				itemJSONPath := fmt.Sprintf("%s + \"[\" + strconv.Itoa(i) + \"]\"", jsonPath)
				walk(validators, itemAccessor, itemJSONPath, baseType, isEnum, refModel, indent+"\t")
			}
			
			sb.WriteString(fmt.Sprintf("%s}\n", indent))
			
			if isPointer {
				indent = indent[:len(indent)-1]
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
				if isPointer {
					// wait, if it's a pointer to struct, the struct field is accessed via accessor.Name directly in Go
					// e.g. input.User.Email
				}
				fieldJSONPath := fmt.Sprintf("%s + \".%s\"", jsonPath, field.JSONName)
				walk(field.Validators, fieldAccessor, fieldJSONPath, field.GoType, field.IsEnum, field.RefModel, innerIndent)
			}
		}

		if isPointer {
			sb.WriteString(fmt.Sprintf("%s}\n", indent))
		}
	}

	if method.IsArgsWrapped {
		for _, arg := range method.Args {
			walk(arg.Validators, "input."+arg.GoName, `"`+arg.Name+`"`, arg.GoType, arg.IsEnum, arg.RefModel, "\t")
		}
	} else if len(method.Args) > 0 {
		inputModel := method.Args[0].RefModel
		if inputModel != nil {
			for _, field := range inputModel.Fields {
				walk(field.Validators, "input."+field.Name, `"`+field.JSONName+`"`, field.GoType, field.IsEnum, field.RefModel, "\t")
			}
		}
	}

	return sb.String()
}
