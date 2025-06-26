package app

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"objectsync/internal/backup"
	"objectsync/internal/config"
	"objectsync/internal/progress"

	"github.com/spf13/cobra"
)

type App struct {
	rootCmd *cobra.Command
}

func NewApp() *App {
	// 初始化控制台编码设置
	initConsole()

	app := &App{}
	app.initCommands()
	return app
}

// initConsole 初始化控制台设置，主要用于Windows下的UTF-8编码支持
func initConsole() {
	if runtime.GOOS == "windows" {
		// 在Windows下，我们已经通过chcp 65001设置了UTF-8编码
		// 这里可以添加其他初始化逻辑
	}
}

func (a *App) Run() error {
	return a.rootCmd.Execute()
}

func (a *App) initCommands() {
	a.rootCmd = &cobra.Command{
		Use:   "objectsync",
		Short: "对象存储下载工具",
		Long:  "一个用于从S3兼容对象存储下载数据到本地的增量下载工具",
	}

	// 添加子命令
	a.rootCmd.AddCommand(a.newBackupCmd())
	a.rootCmd.AddCommand(a.newConfigCmd())
	a.rootCmd.AddCommand(a.newStatusCmd())
}

func (a *App) newBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "执行下载操作",
		Long:  "从对象存储下载指定桶中的所有内容到本地，支持全量和增量下载",
		RunE:  a.runBackup,
	}

	// 添加命令行参数
	cmd.Flags().StringP("config", "c", "config.yaml", "配置文件路径")
	cmd.Flags().StringP("endpoint", "e", "", "Ceph对象存储端点URL (覆盖配置文件)")
	cmd.Flags().StringP("access-key", "a", "", "访问密钥 (覆盖配置文件)")
	cmd.Flags().StringP("secret-key", "s", "", "秘密密钥 (覆盖配置文件)")
	cmd.Flags().StringP("bucket", "b", "", "要备份的桶名称 (覆盖配置文件)")
	cmd.Flags().StringP("output", "o", "./backup", "本地输出目录")
	cmd.Flags().BoolP("incremental", "i", true, "启用增量备份")
	cmd.Flags().StringP("state-file", "f", ".backup_state.json", "状态文件路径")
	cmd.Flags().IntP("workers", "w", 5, "并发下载工作数")
	cmd.Flags().BoolP("verbose", "v", false, "详细输出")

	return cmd
}

func (a *App) newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "配置管理",
		Long:  "配置文件管理和验证",
	}

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "验证配置",
		Long:  "验证配置文件是否正确，测试Ceph连接",
		RunE:  a.runValidate,
	}
	validateCmd.Flags().StringP("config", "c", "config.yaml", "配置文件路径")

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "初始化配置",
		Long:  "交互式创建配置文件",
		RunE:  a.runInit,
	}
	initCmd.Flags().StringP("output", "o", "config.yaml", "输出配置文件路径")

	cmd.AddCommand(validateCmd)
	cmd.AddCommand(initCmd)

	return cmd
}

func (a *App) newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "查看备份状态",
		Long:  "查看上次备份状态和统计信息",
		RunE:  a.runStatus,
	}

	cmd.Flags().StringP("config", "c", "config.yaml", "配置文件路径")
	cmd.Flags().StringP("state-file", "f", ".backup_state.json", "状态文件路径")

	return cmd
}

func (a *App) runBackup(cmd *cobra.Command, args []string) error {
	// 获取命令行参数
	configFile, _ := cmd.Flags().GetString("config")
	endpoint, _ := cmd.Flags().GetString("endpoint")
	accessKey, _ := cmd.Flags().GetString("access-key")
	secretKey, _ := cmd.Flags().GetString("secret-key")
	bucket, _ := cmd.Flags().GetString("bucket")
	outputDir, _ := cmd.Flags().GetString("output")
	incremental, _ := cmd.Flags().GetBool("incremental")
	stateFile, _ := cmd.Flags().GetString("state-file")
	workers, _ := cmd.Flags().GetInt("workers")
	verbose, _ := cmd.Flags().GetBool("verbose")

	// 创建配置管理器
	configManager := config.NewConfigManager(configFile)

	// 加载配置文件
	_, err := configManager.LoadConfig()
	if err != nil {
		// 如果是因为需要配置文件而失败，直接退出
		if configFile == "config.yaml" {
			return fmt.Errorf("配置加载失败: %w", err)
		} else {
			return fmt.Errorf("配置文件 %s 加载失败: %w", configFile, err)
		}
	}

	// 验证配置
	if err := configManager.ValidateConfig(); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 从配置文件获取基础设置
	settings := configManager.ToBackupSettings()

	// 用命令行参数覆盖配置文件设置
	settings.OverrideWithFlags(endpoint, accessKey, secretKey, bucket, outputDir, stateFile, incremental, verbose, workers)

	// 转换为备份选项
	options := &backup.Options{
		Endpoint:    settings.Endpoint,
		AccessKey:   settings.AccessKey,
		SecretKey:   settings.SecretKey,
		Bucket:      settings.Bucket,
		OutputDir:   settings.OutputDir,
		Incremental: settings.Incremental,
		StateFile:   settings.StateFile,
		Workers:     settings.Workers,
		Verbose:     settings.Verbose,
	}

	fmt.Printf("🚀 开始备份 Ceph 桶: %s\n", options.Bucket)
	if options.Verbose {
		fmt.Printf("📋 配置信息:\n")
		fmt.Printf("  端点: %s\n", options.Endpoint)
		fmt.Printf("  桶名: %s\n", options.Bucket)
		fmt.Printf("  输出目录: %s\n", options.OutputDir)
		fmt.Printf("  增量备份: %v\n", options.Incremental)
		fmt.Printf("  并发数: %d\n", options.Workers)
		fmt.Printf("\n")
	}

	// 创建备份器并执行备份
	b := backup.New(options)
	if err := b.Run(); err != nil {
		return fmt.Errorf("❌ 备份失败: %w", err)
	}

	fmt.Println("✅ 备份完成!")
	return nil
}

func (a *App) runValidate(cmd *cobra.Command, args []string) error {
	configFile, _ := cmd.Flags().GetString("config")

	fmt.Printf("🔍 验证配置文件: %s\n", configFile)

	// 创建配置管理器
	configManager := config.NewConfigManager(configFile)

	// 加载配置文件
	_, err := configManager.LoadConfig()
	if err != nil {
		fmt.Printf("❌ 配置加载失败: %v\n", err)
		return err
	}

	// 验证配置
	if err := configManager.ValidateConfig(); err != nil {
		fmt.Printf("❌ 配置验证失败: %v\n", err)
		return err
	}

	fmt.Println("✅ 配置文件验证通过!")

	// 测试连接
	fmt.Println("🔗 测试Ceph连接...")
	settings := configManager.ToBackupSettings()
	options := &backup.Options{
		Endpoint:  settings.Endpoint,
		AccessKey: settings.AccessKey,
		SecretKey: settings.SecretKey,
		Bucket:    settings.Bucket,
	}

	b := backup.New(options)
	if err := b.TestConnection(); err != nil {
		fmt.Printf("❌ 连接失败: %v\n", err)
		return err
	}

	fmt.Printf("✅ 连接成功!\n")
	return nil
}

func (a *App) runStatus(cmd *cobra.Command, args []string) error {
	configFile, _ := cmd.Flags().GetString("config")
	stateFile, _ := cmd.Flags().GetString("state-file")

	fmt.Printf("📊 查看备份状态\n")
	fmt.Printf("配置文件: %s\n", configFile)
	fmt.Printf("状态文件: %s\n", stateFile)
	fmt.Println()

	// 检查状态文件是否存在
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		fmt.Printf("⚠️  状态文件不存在，可能是首次备份\n")
		return nil
	}

	// 读取状态文件
	file, err := os.Open(stateFile)
	if err != nil {
		return fmt.Errorf("❌ 无法读取状态文件: %w", err)
	}
	defer file.Close()

	var state backup.State
	if err := json.NewDecoder(file).Decode(&state); err != nil {
		return fmt.Errorf("❌ 状态文件格式错误: %w", err)
	}

	// 显示状态信息
	fmt.Printf("📅 最后备份时间: %s\n", state.LastBackup.Format("2006-01-02 15:04:05"))
	fmt.Printf("📁 已备份文件数: %d\n", len(state.Files))

	// 计算总大小
	var totalSize int64
	for _, file := range state.Files {
		totalSize += file.Size
	}
	fmt.Printf("💾 总数据大小: %s\n", progress.FormatSize(totalSize))

	// 显示最近的几个文件
	fmt.Println("\n📋 最近备份的文件:")
	count := 0
	for filename, fileState := range state.Files {
		if count >= 5 {
			break
		}
		fmt.Printf("  %s (%s, %s)\n",
			filename,
			progress.FormatSize(fileState.Size),
			fileState.LastModified.Format("2006-01-02 15:04:05"))
		count++
	}

	if len(state.Files) > 5 {
		fmt.Printf("  ... 还有 %d 个文件\n", len(state.Files)-5)
	}

	return nil
}

func (a *App) runInit(cmd *cobra.Command, args []string) error {
	output, _ := cmd.Flags().GetString("output")

	fmt.Println("🚀 交互式配置初始化")
	fmt.Printf("将创建配置文件: %s\n", output)
	fmt.Println()

	// 检查文件是否已存在
	if _, err := os.Stat(output); err == nil {
		fmt.Printf("⚠️  配置文件 %s 已存在\n", output)
		fmt.Print("是否覆盖? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("操作已取消")
			return nil
		}
	}

	// 收集配置信息
	var endpoint, accessKey, secretKey, bucket, outputDir string
	var workers int
	var incremental, verbose bool

	fmt.Print("请输入Ceph端点URL: ")
	fmt.Scanln(&endpoint)

	fmt.Print("请输入访问密钥: ")
	fmt.Scanln(&accessKey)

	fmt.Print("请输入秘密密钥: ")
	fmt.Scanln(&secretKey)

	fmt.Print("请输入桶名称: ")
	fmt.Scanln(&bucket)

	fmt.Print("请输入输出目录 (默认: ./backup): ")
	fmt.Scanln(&outputDir)
	if outputDir == "" {
		outputDir = "./backup"
	}

	fmt.Print("请输入并发数 (默认: 5): ")
	fmt.Scanf("%d", &workers)
	if workers <= 0 {
		workers = 5
	}

	fmt.Print("启用增量备份? (Y/n): ")
	var incResponse string
	fmt.Scanln(&incResponse)
	incremental = incResponse != "n" && incResponse != "N"

	fmt.Print("启用详细输出? (y/N): ")
	var verbResponse string
	fmt.Scanln(&verbResponse)
	verbose = verbResponse == "y" || verbResponse == "Y"

	// 生成配置内容
	configContent := fmt.Sprintf(`# Ceph Object Storage Incremental Backup Tool Configuration
# Generated by interactive initialization

# Ceph Object Storage Configuration
ceph:
  endpoint: "%s"
  access_key: "%s"
  secret_key: "%s"
  bucket: "%s"

# Backup Configuration
backup:
  output_dir: "%s"
  incremental: %t
  state_file: ".backup_state.json"
  workers: %d
  verbose: %t

# Optional Filter Configuration
filters:
  include:
    - "*"
  exclude:
    - "*.tmp"
    - ".DS_Store"

# Retry Configuration
retry:
  max_attempts: 3
  delay: "5s"
`, endpoint, accessKey, secretKey, bucket, outputDir, incremental, workers, verbose)

	// 写入配置文件
	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("❌ 创建配置文件失败: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(configContent)
	if err != nil {
		return fmt.Errorf("❌ 写入配置文件失败: %w", err)
	}

	fmt.Printf("✅ 配置文件已创建: %s\n", output)
	fmt.Println("现在可以运行: objectsync backup --verbose")
	return nil
}
