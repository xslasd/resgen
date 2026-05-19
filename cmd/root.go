package cmd

import (
	"fmt"
	"os"

	"github.com/xslasd/resgen/internal/generator"

	"github.com/spf13/cobra"
)

var showVersion bool

var rootCmd = &cobra.Command{
	Use:   "resgen",
	Short: "resgen is a robust RESTful API code generator",
	Long: `resgen is a Schema-first DSL tool designed to generate 
highly customizable, zero-reflection HTTP server stubs, models, and validations.`,
	Run: func(cmd *cobra.Command, args []string) {
		if showVersion {
			fmt.Printf("resgen version %s\n", generator.Version)
			return
		}
		// 如果没有敲具体的子命令，就输出帮助信息
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Print the version number of resgen")
}
