package cmd

import (
	"github.com/dszqbsm/crawler/cmd/master"
	"github.com/dszqbsm/crawler/cmd/worker"
	"github.com/dszqbsm/crawler/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print version.",
	Long:  "print version.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		version.Printer()
	},
}

func Execute() {
	var rootCmd = &cobra.Command{Use: "crawler"} // 仅用于组织和挂载子命令
	rootCmd.AddCommand(master.MasterCmd, worker.WorkerCmd, versionCmd)
	// 解析并执行命令行指令
	rootCmd.Execute()
}
