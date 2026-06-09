package cmd

import (
	"github.com/spf13/cobra"
	"github.com/xslasd/resgen/internal/generator"
	"github.com/xslasd/resgen/internal/lsp"
)

var lspCmd = &cobra.Command{
	Use:   "lsp",
	Short: "Start the Resgen Language Server (LSP)",
	Long:  `Run a standard Language Server Protocol (LSP) server listening on Stdio.`,
	Run: func(cmd *cobra.Command, args []string) {
		lsp.RunServer(generator.Version)
	},
}

func init() {
	rootCmd.AddCommand(lspCmd)
	// 注册 --stdio 标志以兼容 VSCode 语言客户端的启动参数
	lspCmd.Flags().Bool("stdio", false, "Use stdio for communication")
}
