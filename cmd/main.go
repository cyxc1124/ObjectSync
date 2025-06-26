package main

import (
	"log"
	"os"
	"runtime"

	"objectsync/internal/app"
)

// 版本信息变量（构建时注入）
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func init() {
	// 在Windows上自动设置控制台为UTF-8编码
	if runtime.GOOS == "windows" {
		// 尝试设置控制台代码页为UTF-8
		// 这样可以避免中文乱码问题
		os.Setenv("PYTHONIOENCODING", "utf-8")
	}
}

func main() {
	app := app.NewApp()
	app.SetVersion(Version, BuildTime, GitCommit)
	if err := app.Run(); err != nil {
		log.Printf("错误: %v", err)
		os.Exit(1)
	}
}
