package cmd

import (
	"github.com/dszqbsm/crawler/cmd/master"
	"github.com/dszqbsm/crawler/cmd/worker"
	"github.com/dszqbsm/crawler/version"
	"github.com/spf13/cobra"
)

// cmd.go借助cobra库定义了一个命令行界面，提供了三个子命令，worker用于启动工作节点服务，master用于启动主节点服务，version用于打印版本信息
// 用户执行go run main.go master命令时，会执行masterCmd.Run()函数，启动主节点服务
// 用户执行go run main.go worker命令时，会执行workerCmd.Run()函数，启动工作节点服务
// 用户执行make build构建程序后，执行./main version命令时，会执行versionCmd.Run()函数，打印版本信息；此外执行./main -h还能看到Cobra自动生成的帮助文档

// worker子命令
/* var workerCmd = &cobra.Command{
	Use: "worker",
	// 提供命令的简短活详细描述
	Short: "run worker service.",
	Long:  "run worker service.",
	Args:  cobra.NoArgs, // 规定该命令不接受任何参数
	Run: func(cmd *cobra.Command, args []string) { // 定义命令执行时的逻辑，调用worker.Run()函数启动工作节点服务
		worker.Run()
	},
} */

/* var masterCmd = &cobra.Command{
	Use:   "master",
	Short: "run master service.",
	Long:  "run master service.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		master.Run()
	},
} */

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
	rootCmd.Execute()
}
