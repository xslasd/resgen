package cmd

import (
	"fmt"
	"log"
	
	"github.com/xslasd/resgen/internal/config"
	"github.com/xslasd/resgen/internal/generator"
	"github.com/xslasd/resgen/internal/parser"

	"github.com/spf13/cobra"
)

var sourceFiles []string
var targetDir string
var configFile string

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate code from resgen DSL file",
	Long:  `Read the resgen schema definition file and generate HTTP stub code according to the configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Starting code generation for files: %v\n", sourceFiles)

		// 加载配置
		conf, err := config.LoadConfig(configFile)
		if err != nil {
			log.Fatalf("\n❌ 配置文件加载失败: %v", err)
		}

		// 调用 parser 读取并进行词法/语法树分析
		globalAst, err := parser.ParseSchema(sourceFiles...)
		if err != nil {
			log.Fatalf("\n❌ 解析或校验错误: \n%v", err)
		}

		if globalAst == nil || len(globalAst.Declarations) == 0 {
			log.Fatalf("\n❌ 错误: 未提供任何有效的 DSL 文件")
		}
		
		fmt.Printf("\n✅ 恭喜！解析器成功读取并构建了核心 AST (大小: %d 条顶层声明)！\n", len(globalAst.Declarations))
		
		// 执行代码生成
		if err := generator.Generate(globalAst, targetDir, conf); err != nil {
			log.Fatalf("\n❌ 生成代码失败 (Generate Error): \n%v", err)
		}

		fmt.Printf("🚀 代码已成功生成至目录: %s\n", targetDir)
	},
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().StringSliceVarP(&sourceFiles, "file", "f", []string{"schema.res"}, "Path to the DSL definition file(s) or directory")
	generateCmd.Flags().StringVarP(&targetDir, "out", "o", "resolver", "Output directory for generated files")
	generateCmd.Flags().StringVarP(&configFile, "config", "c", "resgen.yaml", "Path to the resgen configuration file")
}
