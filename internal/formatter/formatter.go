package formatter

import (
	"fmt"
	"io"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/xslasd/resgen/internal/parser"
)

type Formatter struct {
	IndentWidth int
}

func NewFormatter(indentWidth int) *Formatter {
	if indentWidth <= 0 {
		indentWidth = 4
	}
	return &Formatter{IndentWidth: indentWidth}
}

func (f *Formatter) Format(schema *parser.Schema, w io.Writer) error {
	for i, decl := range schema.Declarations {
		if i > 0 {
			_, _ = io.WriteString(w, "\n")
		}
		if err := f.formatDecl(decl, w, 0); err != nil {
			return err
		}
	}
	return nil
}

func (f *Formatter) formatDecl(decl parser.Declaration, w io.Writer, depth int) error {
	indent := strings.Repeat(" ", depth*f.IndentWidth)

	if decl.Module != nil {
		m := decl.Module
		if m.Doc != "" {
			_, _ = io.WriteString(w, f.formatDoc(m.Doc, indent))
		}
		fmt.Fprintf(w, "%smodule %s\n", indent, m.Name)
	}

	if decl.Scalar != nil {
		s := decl.Scalar
		if s.Doc != "" {
			_, _ = io.WriteString(w, f.formatDoc(s.Doc, indent))
		}
		if s.Base != "" {
			fmt.Fprintf(w, "%sscalar %s: %s\n", indent, s.Name, s.Base)
		} else {
			fmt.Fprintf(w, "%sscalar %s\n", indent, s.Name)
		}
	}

	if decl.Decorator != nil {
		d := decl.Decorator
		if d.Doc != "" {
			_, _ = io.WriteString(w, f.formatDoc(d.Doc, indent))
		}
		kw := "validator"
		if d.IsDec {
			kw = "decorator"
		}
		name := d.Name
		if d.IsDec && !strings.HasPrefix(name, "@") {
			name = "@" + name
		}
		
		var args []string
		for _, arg := range d.Args {
			args = append(args, f.formatArgDecl(arg))
		}
		argsStr := ""
		if len(d.Args) > 0 {
			argsStr = "(" + strings.Join(args, ", ") + ")"
		}

		metaStr := ""
		if len(d.Meta) > 0 {
			var metas []string
			for _, entry := range d.Meta {
				metas = append(metas, f.formatMetaEntry(entry))
			}
			metaStr = "[" + strings.Join(metas, ", ") + "]"
		}

		fmt.Fprintf(w, "%s%s %s%s%s\n", indent, kw, name, argsStr, metaStr)
	}

	if decl.Model != nil {
		m := decl.Model
		if m.Doc != "" {
			_, _ = io.WriteString(w, f.formatDoc(m.Doc, indent))
		}
		for _, dir := range m.Directives {
			fmt.Fprintf(w, "%s%s\n", indent, f.formatDirective(dir))
		}

		typeParamsStr := ""
		if len(m.TypeParams) > 0 {
			typeParamsStr = "<" + strings.Join(m.TypeParams, ", ") + ">"
		}

		fmt.Fprintf(w, "%s%s %s%s {\n", indent, m.Keyword, m.Name, typeParamsStr)
		for _, prop := range m.Properties {
			if prop.Doc != "" {
				_, _ = io.WriteString(w, f.formatDoc(prop.Doc, indent+strings.Repeat(" ", f.IndentWidth)))
			}
			var dirs []string
			for _, dir := range prop.Directives {
				dirs = append(dirs, f.formatDirective(dir))
			}
			dirsStr := ""
			if len(dirs) > 0 {
				dirsStr = " " + strings.Join(dirs, " ")
			}
			fmt.Fprintf(w, "%s%s%s: %s%s\n", 
				indent+strings.Repeat(" ", f.IndentWidth), 
				prop.Name, 
				"", // 原先可能有空格，现在规范为无空格
				f.formatTypeRef(prop.Type), 
				dirsStr,
			)
		}
		fmt.Fprintf(w, "%s}\n", indent)
	}

	if decl.Group != nil {
		g := decl.Group
		if g.Doc != "" {
			_, _ = io.WriteString(w, f.formatDoc(g.Doc, indent))
		}
		for _, dir := range g.Directives {
			fmt.Fprintf(w, "%s%s\n", indent, f.formatDirective(dir))
		}

		namePart := ""
		if g.Name != "" {
			namePart = " " + g.Name
		}
		metaStr := ""
		if len(g.Meta) > 0 {
			var metas []string
			for _, entry := range g.Meta {
				metas = append(metas, f.formatMetaEntry(entry))
			}
			metaStr = " [" + strings.Join(metas, ", ") + "]"
		}

		fmt.Fprintf(w, "%sgroup%s %s%s {\n", indent, namePart, g.Path, metaStr)
		for _, ep := range g.Endpoints {
			if ep.Doc != "" {
				_, _ = io.WriteString(w, f.formatDoc(ep.Doc, indent+strings.Repeat(" ", f.IndentWidth)))
			}
			for _, dir := range ep.Directives {
				fmt.Fprintf(w, "%s%s\n", indent+strings.Repeat(" ", f.IndentWidth), f.formatDirective(dir))
			}

			reqMetaStr := ""
			if len(ep.RequestMeta) > 0 {
				var metas []string
				for _, entry := range ep.RequestMeta {
					metas = append(metas, f.formatMetaEntry(entry))
				}
				reqMetaStr = "[" + strings.Join(metas, ", ") + "]"
			}

			var args []string
			for _, arg := range ep.Args {
				args = append(args, f.formatArgDecl(arg))
			}
			argsStr := strings.Join(args, ", ")

			respMetaStr := ""
			if len(ep.ResponseMeta) > 0 {
				var metas []string
				for _, entry := range ep.ResponseMeta {
					metas = append(metas, f.formatMetaEntry(entry))
				}
				respMetaStr = " [" + strings.Join(metas, ", ") + "]"
			}

			fmt.Fprintf(w, "%s%s %s%s => %s(%s): %s%s\n",
				indent+strings.Repeat(" ", f.IndentWidth),
				strings.ToUpper(ep.Method),
				ep.Path,
				reqMetaStr,
				ep.Name,
				argsStr,
				f.formatTypeRef(ep.ReturnType),
				respMetaStr,
			)
		}
		fmt.Fprintf(w, "%s}\n", indent)
	}

	return nil
}

func (f *Formatter) formatDoc(doc string, indent string) string {
	if doc == "" {
		return ""
	}
	lines := strings.Split(doc, "\n")
	var result []string
	for _, line := range lines {
		result = append(result, indent+"# "+line)
	}
	return strings.Join(result, "\n") + "\n"
}

func (f *Formatter) formatArgDecl(arg parser.ArgDecl) string {
	var dirs []string
	for _, dir := range arg.Directives {
		dirs = append(dirs, f.formatDirective(dir))
	}
	dirsStr := ""
	if len(dirs) > 0 {
		dirsStr = " " + strings.Join(dirs, " ")
	}
	return fmt.Sprintf("%s: %s%s", arg.Name, f.formatTypeRef(arg.Type), dirsStr)
}

func (f *Formatter) formatMetaEntry(entry parser.MetaEntry) string {
	return entry.Key + "=" + f.formatMetaValue(entry.Value)
}

func (f *Formatter) formatMetaValue(val parser.MetaValue) string {
	if val.Str != nil {
		if isValidIdent(*val.Str) {
			return *val.Str
		}
		return fmt.Sprintf(`"%s"`, *val.Str)
	}
	if val.Int != nil {
		return fmt.Sprintf("%d", *val.Int)
	}
	return ""
}

func (f *Formatter) formatDirective(d parser.DirectiveUsage) string {
	var sb strings.Builder
	sb.WriteString("@")
	sb.WriteString(d.Name)
	if len(d.Args) > 0 {
		sb.WriteString("(")
		var args []string
		for _, arg := range d.Args {
			args = append(args, f.formatDirectiveArg(arg))
		}
		sb.WriteString(strings.Join(args, ", "))
		sb.WriteString(")")
	}
	if len(d.Meta) > 0 {
		sb.WriteString("[")
		var metas []string
		for _, entry := range d.Meta {
			metas = append(metas, f.formatMetaEntry(entry))
		}
		sb.WriteString(strings.Join(metas, ", "))
		sb.WriteString("]")
	}
	return sb.String()
}

func (f *Formatter) formatDirectiveArg(arg parser.DirectiveArg) string {
	if arg.Name != "" {
		return arg.Name + ": " + f.formatValue(&arg.Value)
	}
	return f.formatValue(&arg.Value)
}

func (f *Formatter) formatValue(val *parser.Value) string {
	if val == nil {
		return ""
	}
	if val.String != nil {
		return fmt.Sprintf(`"%s"`, *val.String)
	}
	if val.Int != nil {
		return fmt.Sprintf("%d", *val.Int)
	}
	if val.Float != nil {
		return fmt.Sprintf("%g", *val.Float)
	}
	if val.Ident != nil {
		return *val.Ident
	}
	if val.List != nil {
		var parts []string
		for _, v := range val.List {
			parts = append(parts, f.formatValue(v))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return ""
}

func (f *Formatter) formatTypeRef(t parser.TypeRef) string {
	var sb strings.Builder
	if t.IsArray {
		sb.WriteString("[")
	}
	sb.WriteString(t.Name)
	if len(t.TypeArgs) > 0 {
		sb.WriteString("<")
		var args []string
		for _, arg := range t.TypeArgs {
			args = append(args, f.formatTypeRef(arg))
		}
		sb.WriteString(strings.Join(args, ", "))
		sb.WriteString(">")
	}
	if t.ItemNotNull {
		sb.WriteString("!")
	}
	if t.IsArray {
		sb.WriteString("]")
		if t.ArrNotNull {
			sb.WriteString("!")
		}
	}
	return sb.String()
}

func isValidIdent(s string) bool {
	if len(s) == 0 {
		return false
	}
	first := s[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_') {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

func (f *Formatter) formatPos(pos lexer.Position) string {
	return fmt.Sprintf("%s:%d:%d", pos.Filename, pos.Line, pos.Column)
}
