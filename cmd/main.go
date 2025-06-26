package main

import (
	"log"
	"os"
	"runtime"

	"objectsync/internal/app"
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
	if err := app.Run(); err != nil {
		log.Printf("Error: %v", err)
		os.Exit(1)
	}
}
