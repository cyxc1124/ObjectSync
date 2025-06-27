package app

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"objectsync/internal/backup"
	"objectsync/internal/config"
	"objectsync/internal/progress"
	"objectsync/internal/upload"

	"github.com/spf13/cobra"
)

type App struct {
	rootCmd   *cobra.Command
	version   string
	buildTime string
	gitCommit string
}

func NewApp() *App {
	// 初始化控制台编码设置
	initConsole()

	app := &App{}
	app.initCommands()
	return app
}

// SetVersion 设置版本信息
func (a *App) SetVersion(version, buildTime, gitCommit string) {
	a.version = version
	a.buildTime = buildTime
	a.gitCommit = gitCommit

	// 更新rootCmd的版本信息
	a.rootCmd.Version = version
}

// initConsole 初始化控制台设置，主要用于Windows下的UTF-8编码支持
func initConsole() {
	if runtime.GOOS == "windows" {
		// 在Windows下，我们已经通过chcp 65001设置了UTF-8编码
		// 这里可以添加其他初始化逻辑
	}
}

func (a *App) Run() error {
	// 检查是否有参数，如果没有参数直接启动菜单
	if len(os.Args) == 1 {
		// 没有参数，直接启动交互式菜单
		return a.runMenu(a.rootCmd, []string{})
	}
	// 有参数，正常执行cobra命令
	return a.rootCmd.Execute()
}

func (a *App) initCommands() {
	a.rootCmd = &cobra.Command{
		Use:   "objectsync",
		Short: "对象存储同步工具",
		Long:  "一个用于与S3兼容对象存储进行数据同步的工具，支持下载和上传功能，支持增量同步",
		RunE:  a.runDefault, // 智能默认行为
	}

	// 添加子命令
	a.rootCmd.AddCommand(a.newBackupCmd())
	a.rootCmd.AddCommand(a.newUploadCmd())
	a.rootCmd.AddCommand(a.newConfigCmd())
	a.rootCmd.AddCommand(a.newStatusCmd())
	a.rootCmd.AddCommand(a.newVersionCmd())
	a.rootCmd.AddCommand(a.newMenuCmd()) // 添加交互式菜单命令
}

func (a *App) newBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "执行备份操作",
		Long:  "从配置文件中指定的所有桶下载对象到本地，支持增量备份，自动创建本地目录",
		RunE:  a.runBackup,
	}

	// 添加命令行参数
	cmd.Flags().StringP("config", "c", "config.yaml", "配置文件路径")
	cmd.Flags().StringP("endpoint", "e", "", "Ceph对象存储端点URL (覆盖配置文件)")
	cmd.Flags().StringP("access-key", "a", "", "访问密钥 (覆盖配置文件)")
	cmd.Flags().StringP("secret-key", "s", "", "秘密密钥 (覆盖配置文件)")
	cmd.Flags().BoolP("incremental", "i", true, "启用增量备份")
	cmd.Flags().IntP("workers", "w", 5, "并发下载工作数")
	cmd.Flags().BoolP("verbose", "v", false, "详细输出")

	return cmd
}

func (a *App) newUploadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload",
		Short: "执行上传操作",
		Long:  "将本地文件上传到配置文件中指定的所有桶，支持增量上传，自动创建不存在的存储桶",
		RunE:  a.runUpload,
	}

	// 添加命令行参数
	cmd.Flags().StringP("config", "c", "config.yaml", "配置文件路径")
	cmd.Flags().StringP("endpoint", "e", "", "Ceph对象存储端点URL (覆盖配置文件)")
	cmd.Flags().StringP("access-key", "a", "", "访问密钥 (覆盖配置文件)")
	cmd.Flags().StringP("secret-key", "s", "", "秘密密钥 (覆盖配置文件)")
	cmd.Flags().BoolP("incremental", "i", true, "启用增量上传")
	cmd.Flags().IntP("workers", "w", 5, "并发上传工作数")
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

func (a *App) newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "显示版本信息",
		Long:  "显示程序版本、构建时间和Git提交信息",
		RunE:  a.runVersion,
	}

	return cmd
}

func (a *App) newMenuCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "menu",
		Short: "交互式菜单（默认行为）",
		Long:  "提供交互式菜单界面，这也是直接运行 objectsync 的默认行为",
		RunE:  a.runMenu,
	}

	return cmd
}

// runDefault 智能默认行为：没有子命令时启动交互式菜单
func (a *App) runDefault(cmd *cobra.Command, args []string) error {
	// 没有参数和标志时启动交互式菜单
	return a.runMenu(cmd, args)
}

func (a *App) runBackup(cmd *cobra.Command, args []string) error {
	// 获取命令行参数
	configFile, _ := cmd.Flags().GetString("config")
	endpoint, _ := cmd.Flags().GetString("endpoint")
	accessKey, _ := cmd.Flags().GetString("access-key")
	secretKey, _ := cmd.Flags().GetString("secret-key")
	incremental, _ := cmd.Flags().GetBool("incremental")
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

	// 统一处理所有桶的备份
	return a.runBucketsBackup(configManager, endpoint, accessKey, secretKey, incremental, verbose, workers)
}

// runBucketsBackup 统一执行桶备份
func (a *App) runBucketsBackup(configManager *config.ConfigManager, endpoint, accessKey, secretKey string, incremental, verbose bool, workers int) error {
	// 获取桶配置
	settings := configManager.ToBucketSettings()

	// 用命令行参数覆盖连接配置
	if endpoint != "" {
		settings.Endpoint = endpoint
	}
	if accessKey != "" {
		settings.AccessKey = accessKey
	}
	if secretKey != "" {
		settings.SecretKey = secretKey
	}
	settings.Incremental = incremental

	// 备份配置中的所有桶
	bucketCount := len(settings.Buckets)
	fmt.Printf("开始备份（共 %d 个桶）\n", bucketCount)
	fmt.Printf("连接信息: %s\n", settings.Endpoint)

	if verbose {
		fmt.Printf("桶列表:\n")
		for i, bucket := range settings.Buckets {
			fmt.Printf("  %d. %s -> %s\n", i+1, bucket.Name, bucket.OutputDir)
		}
		fmt.Println()
	}

	// 逐个备份每个桶
	successCount := 0
	failureCount := 0

	for i, bucketSettings := range settings.Buckets {
		fmt.Printf("\n[%d/%d] 备份桶: %s\n", i+1, bucketCount, bucketSettings.Name)

		// 为每个桶创建备份选项
		options := &backup.Options{
			Endpoint:    settings.Endpoint,
			AccessKey:   settings.AccessKey,
			SecretKey:   settings.SecretKey,
			Bucket:      bucketSettings.Name,
			OutputDir:   bucketSettings.OutputDir,
			Incremental: settings.Incremental,
			StateFile:   bucketSettings.StateFile,
			Workers:     bucketSettings.Workers,
			Verbose:     bucketSettings.Verbose || verbose,
		}

		if options.Verbose {
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
			fmt.Printf("桶 %s 备份失败: %v\n", bucketSettings.Name, err)
			failureCount++
			continue
		}

		fmt.Printf("桶 %s 备份完成!\n", bucketSettings.Name)
		successCount++
	}

	// 显示备份总结
	fmt.Printf("\n备份完成!\n")
	fmt.Printf("成功: %d 个桶\n", successCount)
	if failureCount > 0 {
		fmt.Printf("失败: %d 个桶\n", failureCount)
		return fmt.Errorf("部分桶备份失败")
	}

	return nil
}

func (a *App) runValidate(cmd *cobra.Command, args []string) error {
	configFile, _ := cmd.Flags().GetString("config")

	fmt.Printf("验证配置文件: %s\n", configFile)

	// 创建配置管理器
	configManager := config.NewConfigManager(configFile)

	// 加载配置文件
	_, err := configManager.LoadConfig()
	if err != nil {
		fmt.Printf("配置加载失败: %v\n", err)
		return err
	}

	// 验证配置
	if err := configManager.ValidateConfig(); err != nil {
		fmt.Printf("配置验证失败: %v\n", err)
		return err
	}

	fmt.Println("配置文件验证通过!")

	// 测试连接
	fmt.Println("测试Ceph连接...")
	settings := configManager.ToBucketSettings()

	// 测试第一个桶的连接
	if len(settings.Buckets) == 0 {
		fmt.Printf("没有配置要测试的桶\n")
		return fmt.Errorf("配置中没有桶信息")
	}

	firstBucket := settings.Buckets[0]
	options := &backup.Options{
		Endpoint:  settings.Endpoint,
		AccessKey: settings.AccessKey,
		SecretKey: settings.SecretKey,
		Bucket:    firstBucket.Name,
	}

	b := backup.New(options)
	if err := b.TestConnection(); err != nil {
		fmt.Printf("连接失败: %v\n", err)
		return err
	}

	fmt.Printf("连接成功!\n")
	return nil
}

func (a *App) runVersion(cmd *cobra.Command, args []string) error {
	fmt.Printf("ObjectSync 对象存储下载工具\n")
	fmt.Printf("版本: %s\n", a.version)
	fmt.Printf("构建时间: %s\n", a.buildTime)
	fmt.Printf("Git提交: %s\n", a.gitCommit)
	fmt.Printf("Go版本: %s\n", runtime.Version())
	fmt.Printf("操作系统: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	return nil
}

func (a *App) runStatus(cmd *cobra.Command, args []string) error {
	configFile, _ := cmd.Flags().GetString("config")
	stateFile, _ := cmd.Flags().GetString("state-file")

	fmt.Printf("查看备份状态\n")
	fmt.Printf("配置文件: %s\n", configFile)
	fmt.Printf("状态文件: %s\n", stateFile)
	fmt.Println()

	// 检查状态文件是否存在
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		fmt.Printf("状态文件不存在，可能是首次备份\n")
		return nil
	}

	// 读取状态文件
	file, err := os.Open(stateFile)
	if err != nil {
		return fmt.Errorf("无法读取状态文件: %w", err)
	}
	defer file.Close()

	var state backup.State
	if err := json.NewDecoder(file).Decode(&state); err != nil {
		return fmt.Errorf("状态文件格式错误: %w", err)
	}

	// 显示状态信息
	fmt.Printf("最后备份时间: %s\n", state.LastBackup.Format("2006-01-02 15:04:05"))
	fmt.Printf("已备份文件数: %d\n", len(state.Files))

	// 计算总大小
	var totalSize int64
	for _, file := range state.Files {
		totalSize += file.Size
	}
	fmt.Printf("总数据大小: %s\n", progress.FormatSize(totalSize))

	// 显示最近的几个文件
	fmt.Println("\n最近备份的文件:")
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

// runStatusMenu 专为菜单系统设计的状态查看
func (a *App) runStatusMenu() error {
	configFile := "config.yaml"       // 默认配置文件
	stateFile := ".backup_state.json" // 默认状态文件

	fmt.Printf("查看备份状态\n")
	fmt.Printf("配置文件: %s\n", configFile)

	// 先检查配置文件是否存在
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Printf("配置文件不存在，请先进行配置初始化\n")
		return nil
	}

	// 尝试加载配置以获取正确的状态文件路径
	configManager := config.NewConfigManager(configFile)
	if _, err := configManager.LoadConfig(); err == nil {
		// 成功加载配置，显示所有桶的状态
		settings := configManager.ToBucketSettings()
		bucketCount := len(settings.Buckets)

		if bucketCount == 0 {
			fmt.Printf("配置中没有配置桶信息\n")
			return nil
		}

		fmt.Printf("\n显示所有桶的状态（共 %d 个桶）:\n", bucketCount)
		for i, bucket := range settings.Buckets {
			fmt.Printf("\n[%d] 桶: %s\n", i+1, bucket.Name)
			fmt.Printf("    状态文件: %s\n", bucket.StateFile)

			if err := a.showBucketStatus(bucket.StateFile, true); err != nil { // true表示使用缩进
				fmt.Printf("    读取状态失败: %v\n", err)
			}
		}
		return nil
	} else {
		fmt.Printf("配置文件加载失败: %v\n", err)
		fmt.Printf("使用默认状态文件: %s\n", stateFile)
		fmt.Println()

		// 显示默认状态文件的状态
		return a.showBucketStatus(stateFile, false) // false表示不使用缩进
	}
}

// showBucketStatus 显示单个桶的备份状态
func (a *App) showBucketStatus(stateFile string, withIndent bool) error {
	// 根据缩进需要设置前缀
	indent := ""
	if withIndent {
		indent = "    "
	}

	// 检查状态文件是否存在
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		fmt.Printf("%s状态文件不存在，可能是首次备份\n", indent)
		return nil
	}

	// 读取状态文件
	file, err := os.Open(stateFile)
	if err != nil {
		return fmt.Errorf("无法读取状态文件: %w", err)
	}
	defer file.Close()

	var state backup.State
	if err := json.NewDecoder(file).Decode(&state); err != nil {
		return fmt.Errorf("状态文件格式错误: %w", err)
	}

	// 显示状态信息
	fmt.Printf("%s最后备份时间: %s\n", indent, state.LastBackup.Format("2006-01-02 15:04:05"))
	fmt.Printf("%s已备份文件数: %d\n", indent, len(state.Files))

	// 计算总大小
	var totalSize int64
	for _, file := range state.Files {
		totalSize += file.Size
	}
	fmt.Printf("%s总数据大小: %s\n", indent, progress.FormatSize(totalSize))

	// 显示最近的几个文件
	fmt.Printf("%s最近备份的文件:\n", indent)
	count := 0
	for filename, fileState := range state.Files {
		if count >= 3 { // 在菜单模式下显示少一些文件
			break
		}
		fmt.Printf("%s  %s (%s, %s)\n",
			indent,
			filename,
			progress.FormatSize(fileState.Size),
			fileState.LastModified.Format("2006-01-02 15:04:05"))
		count++
	}

	if len(state.Files) > 3 {
		fmt.Printf("%s  ... 还有 %d 个文件\n", indent, len(state.Files)-3)
	}

	return nil
}

func (a *App) runInit(cmd *cobra.Command, args []string) error {
	output, _ := cmd.Flags().GetString("output")

	fmt.Println("交互式配置初始化")
	fmt.Printf("将创建配置文件: %s\n", output)
	fmt.Println()

	// 检查文件是否已存在
	if _, err := os.Stat(output); err == nil {
		fmt.Printf("配置文件 %s 已存在\n", output)
		fmt.Print("是否覆盖? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("操作已取消")
			return nil
		}
	}

	// 收集基础连接信息
	var endpoint, accessKey, secretKey string
	var workers int
	var incremental, verbose bool

	fmt.Print("请输入对象存储端点URL: ")
	fmt.Scanln(&endpoint)

	fmt.Print("请输入访问密钥: ")
	fmt.Scanln(&accessKey)

	fmt.Print("请输入秘密密钥: ")
	fmt.Scanln(&secretKey)

	fmt.Print("请输入默认并发数 (默认: 5): ")
	var workersInput string
	fmt.Scanln(&workersInput)
	if workersInput == "" {
		workers = 5
	} else {
		fmt.Sscanf(workersInput, "%d", &workers)
		if workers <= 0 {
			workers = 5
		}
	}

	fmt.Print("启用增量备份? (Y/n): ")
	var incResponse string
	fmt.Scanln(&incResponse)
	incremental = incResponse != "n" && incResponse != "N"

	fmt.Print("启用详细输出? (y/N): ")
	var verbResponse string
	fmt.Scanln(&verbResponse)
	verbose = verbResponse == "y" || verbResponse == "Y"

	// 生成默认配置（包含示例桶配置）
	fmt.Println("\n生成配置文件...")
	configContent := a.generateDefaultConfig(endpoint, accessKey, secretKey, workers, incremental, verbose)

	// 写入配置文件
	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("创建配置文件失败: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(configContent)
	if err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	fmt.Printf("配置文件已创建: %s\n", output)
	fmt.Println("请编辑配置文件，填入正确的桶名称和输出目录")
	fmt.Println("然后运行: objectsync backup --verbose")
	return nil
}

// runInitMenu 专为菜单系统设计的配置初始化
func (a *App) runInitMenu() error {
	output := "config.yaml" // 固定使用默认配置文件名

	fmt.Println("交互式配置初始化")
	fmt.Printf("将创建配置文件: %s\n", output)
	fmt.Println()

	// 检查文件是否已存在
	if _, err := os.Stat(output); err == nil {
		fmt.Printf("配置文件 %s 已存在\n", output)
		fmt.Print("是否覆盖? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("操作已取消")
			return nil
		}
	}

	// 收集基础连接信息
	var endpoint, accessKey, secretKey string
	var workers int
	var incremental, verbose bool

	fmt.Print("请输入对象存储端点URL: ")
	fmt.Scanln(&endpoint)

	fmt.Print("请输入访问密钥: ")
	fmt.Scanln(&accessKey)

	fmt.Print("请输入秘密密钥: ")
	fmt.Scanln(&secretKey)

	fmt.Print("请输入默认并发数 (默认: 5): ")
	var workersInput string
	fmt.Scanln(&workersInput)
	if workersInput == "" {
		workers = 5
	} else {
		fmt.Sscanf(workersInput, "%d", &workers)
		if workers <= 0 {
			workers = 5
		}
	}

	fmt.Print("启用增量备份? (Y/n): ")
	var incResponse string
	fmt.Scanln(&incResponse)
	incremental = incResponse != "n" && incResponse != "N"

	fmt.Print("启用详细输出? (y/N): ")
	var verbResponse string
	fmt.Scanln(&verbResponse)
	verbose = verbResponse == "y" || verbResponse == "Y"

	// 生成默认配置（包含示例桶配置）
	fmt.Println("\n生成配置文件...")
	configContent := a.generateDefaultConfig(endpoint, accessKey, secretKey, workers, incremental, verbose)

	// 写入配置文件
	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("创建配置文件失败: %w", err)
	}
	defer file.Close()

	_, err = file.WriteString(configContent)
	if err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	fmt.Printf("配置文件已创建: %s\n", output)
	fmt.Println("请编辑配置文件，填入正确的桶名称和输出目录")
	fmt.Println("然后运行: objectsync backup --verbose")
	return nil
}

// generateDefaultConfig 生成默认配置（统一处理）
func (a *App) generateDefaultConfig(endpoint, accessKey, secretKey string, workers int, incremental, verbose bool) string {
	return fmt.Sprintf(`# ObjectSync - 对象存储下载工具配置文件
# 由交互式初始化生成

# 对象存储连接配置
ceph:
  endpoint: "%s"
  access_key: "%s"
  secret_key: "%s"

# 桶配置 - 请根据实际情况修改
# 单桶：保留一个桶配置，删除其他
# 多桶：添加更多桶配置
buckets:
  - name: "your-bucket-name"              # 请修改为实际的桶名称
    output_dir: "./backup"                # 请修改为实际的输出目录
    state_file: ".backup_state.json"
  # 示例：多桶配置（如不需要请删除）
  # - name: "documents"
  #   output_dir: "./backup/documents"
  #   state_file: ".state_documents.json"
  # - name: "photos"
  #   output_dir: "./backup/photos"
  #   state_file: ".state_photos.json"
  #   workers: 8                          # 可选：为特定桶设置不同的并发数
  #   verbose: true                       # 可选：为特定桶启用详细输出

# 全局备份配置
backup:
  incremental: %t                         # 启用增量备份
  workers: %d                             # 默认并发数
  verbose: %t                             # 默认详细输出

# 重试配置
retry:
  max_attempts: 3
  delay: "5s"
`, endpoint, accessKey, secretKey, incremental, workers, verbose)
}

func (a *App) runMenu(cmd *cobra.Command, args []string) error {
	for {
		// 清屏（跨平台兼容）
		a.clearScreen()

		// 显示标题
		fmt.Println("========================================")
		fmt.Println("       ObjectSync - 交互式菜单")
		fmt.Println("========================================")
		fmt.Println()
		fmt.Println("欢迎使用 ObjectSync 对象存储下载工具！")
		fmt.Println()

		// 显示菜单
		fmt.Println("========================================")
		fmt.Println("            主菜单")
		fmt.Println("========================================")
		fmt.Println()
		fmt.Println("[1] 初始化配置")
		fmt.Println("[2] 开始下载")
		fmt.Println("[3] 开始上传")
		fmt.Println("[4] 查看状态")
		fmt.Println("[5] 查看配置")
		fmt.Println("[6] 查看帮助")
		fmt.Println("[0] 退出")
		fmt.Println()
		fmt.Print("请选择操作 (0-6): ")

		var choice string
		fmt.Scanln(&choice)
		fmt.Println()

		switch choice {
		case "1":
			// 初始化配置
			fmt.Println("[信息] 启动配置向导...")
			if err := a.runInitMenu(); err != nil {
				fmt.Printf("配置初始化失败: %v\n", err)
			}
			a.pauseAndContinue()

		case "2":
			// 开始下载
			fmt.Println("[信息] 开始下载...")
			if err := a.runBackup(cmd, args); err != nil {
				fmt.Printf("下载失败: %v\n", err)
			}
			a.pauseAndContinue()

		case "3":
			// 开始上传
			fmt.Println("[信息] 开始上传...")
			if err := a.runUploadMenu(); err != nil {
				fmt.Printf("上传失败: %v\n", err)
			}
			a.pauseAndContinue()

		case "4":
			// 查看状态
			fmt.Println("[信息] 查看备份状态...")
			if err := a.runStatusMenu(); err != nil {
				fmt.Printf("查看状态失败: %v\n", err)
			}
			a.pauseAndContinue()

		case "5":
			// 查看配置
			fmt.Println("[信息] 当前配置文件内容:")
			fmt.Println("========================================")
			a.showCurrentConfig()
			fmt.Println("========================================")
			a.pauseAndContinue()

		case "6":
			// 查看帮助
			fmt.Println("[信息] 显示帮助信息...")
			a.rootCmd.Help()
			a.pauseAndContinue()

		case "0":
			fmt.Println()
			fmt.Println("[信息] 感谢使用 ObjectSync 工具！")
			fmt.Println()
			return nil

		default:
			fmt.Println("[错误] 无效的选择，请重新输入")
			time.Sleep(1 * time.Second)
		}
	}
}

// pauseAndContinue 暂停并等待用户按键继续
func (a *App) pauseAndContinue() {
	fmt.Println()
	fmt.Print("[信息] 按回车键返回主菜单...")
	fmt.Scanln()
}

// clearScreen 跨平台清屏
func (a *App) clearScreen() {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	} else {
		fmt.Print("\033[2J\033[H")
	}
}

// showCurrentConfig 显示当前配置文件
func (a *App) showCurrentConfig() {
	configFile := "config.yaml"
	if data, err := os.ReadFile(configFile); err != nil {
		fmt.Println("[警告] 配置文件不存在或无法读取，请先进行配置")
	} else {
		fmt.Print(string(data))
	}
}

func (a *App) runUpload(cmd *cobra.Command, args []string) error {
	// 获取命令行参数
	configFile, _ := cmd.Flags().GetString("config")
	endpoint, _ := cmd.Flags().GetString("endpoint")
	accessKey, _ := cmd.Flags().GetString("access-key")
	secretKey, _ := cmd.Flags().GetString("secret-key")
	incremental, _ := cmd.Flags().GetBool("incremental")
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

	// 获取桶配置
	settings := configManager.ToBucketSettings()

	// 用命令行参数覆盖连接配置
	if endpoint != "" {
		settings.Endpoint = endpoint
	}
	if accessKey != "" {
		settings.AccessKey = accessKey
	}
	if secretKey != "" {
		settings.SecretKey = secretKey
	}
	settings.Incremental = incremental

	// 上传到配置中的所有桶
	bucketCount := len(settings.Buckets)
	fmt.Printf("开始上传（共 %d 个桶）\n", bucketCount)
	fmt.Printf("连接信息: %s\n", settings.Endpoint)

	if verbose {
		fmt.Printf("桶列表:\n")
		for i, bucket := range settings.Buckets {
			fmt.Printf("  %d. %s <- %s\n", i+1, bucket.Name, bucket.OutputDir)
		}
		fmt.Println()
	}

	// 逐个上传每个桶
	successCount := 0
	failureCount := 0

	for i, bucketSettings := range settings.Buckets {
		fmt.Printf("\n[%d/%d] 上传桶: %s\n", i+1, bucketCount, bucketSettings.Name)

		// 为每个桶创建上传选项
		options := &upload.Options{
			Endpoint:    settings.Endpoint,
			AccessKey:   settings.AccessKey,
			SecretKey:   settings.SecretKey,
			Bucket:      bucketSettings.Name,
			InputDir:    bucketSettings.OutputDir, // 从各自的输出目录上传
			Incremental: incremental,
			StateFile:   fmt.Sprintf(".upload_%s_state.json", bucketSettings.Name), // 每个桶独立的状态文件
			Workers:     workers,
			Verbose:     verbose,
		}

		if options.Verbose {
			fmt.Printf("  端点: %s\n", options.Endpoint)
			fmt.Printf("  桶名: %s\n", options.Bucket)
			fmt.Printf("  输入目录: %s\n", options.InputDir)
			fmt.Printf("  增量上传: %v\n", options.Incremental)
			fmt.Printf("  并发数: %d\n", options.Workers)
			fmt.Printf("\n")
		}

		// 创建上传器并执行上传
		u := upload.New(options)
		if err := u.Run(); err != nil {
			fmt.Printf("桶 %s 上传失败: %v\n", bucketSettings.Name, err)
			failureCount++
			continue
		}

		fmt.Printf("桶 %s 上传完成!\n", bucketSettings.Name)
		successCount++
	}

	// 显示上传总结
	fmt.Printf("\n上传完成!\n")
	fmt.Printf("成功: %d 个桶\n", successCount)
	if failureCount > 0 {
		fmt.Printf("失败: %d 个桶\n", failureCount)
		return fmt.Errorf("部分桶上传失败")
	}

	return nil
}

func (a *App) runUploadMenu() error {
	fmt.Println("========================================")
	fmt.Println("            上传设置")
	fmt.Println("========================================")
	fmt.Println()

	// 创建配置管理器
	configManager := config.NewConfigManager("config.yaml")

	// 加载配置文件
	_, err := configManager.LoadConfig()
	if err != nil {
		return fmt.Errorf("配置加载失败: %w", err)
	}

	// 验证配置
	if err := configManager.ValidateConfig(); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 获取桶配置信息
	settings := configManager.ToBucketSettings()

	fmt.Printf("发现 %d 个已配置的桶:\n", len(settings.Buckets))
	for i, bucket := range settings.Buckets {
		fmt.Printf("  %d. 桶名: %s\n", i+1, bucket.Name)
		fmt.Printf("     本地目录: %s\n", bucket.OutputDir)

		// 检查目录是否存在
		if _, err := os.Stat(bucket.OutputDir); os.IsNotExist(err) {
			fmt.Printf("     状态: 目录不存在 ❌\n")
		} else {
			fmt.Printf("     状态: 目录存在 ✅\n")
		}
		fmt.Println()
	}

	fmt.Println("上传逻辑:")
	fmt.Println("  • 每个桶将从其配置的本地目录上传数据")
	fmt.Println("  • 只有存在本地目录的桶才会被上传")
	fmt.Println("  • 每个桶使用独立的上传状态文件")
	fmt.Println()

	// 询问是否继续
	fmt.Print("是否继续上传? (Y/n): ")
	var continueInput string
	fmt.Scanln(&continueInput)
	if continueInput == "n" || continueInput == "N" {
		fmt.Println("上传已取消")
		return nil
	}

	// 询问是否使用详细模式
	fmt.Print("是否启用详细输出? (y/N): ")
	var verboseInput string
	fmt.Scanln(&verboseInput)
	verbose := verboseInput == "y" || verboseInput == "Y"

	fmt.Println()
	fmt.Println("开始上传...")

	// 执行上传逻辑
	// 获取桶配置并处理上传
	if len(settings.Buckets) == 0 {
		return fmt.Errorf("没有配置的桶")
	}

	// 上传到配置中的所有桶
	bucketCount := len(settings.Buckets)
	fmt.Printf("开始上传（共 %d 个桶）\n", bucketCount)
	fmt.Printf("连接信息: %s\n", settings.Endpoint)

	if verbose {
		fmt.Printf("桶列表:\n")
		for i, bucket := range settings.Buckets {
			fmt.Printf("  %d. %s <- %s\n", i+1, bucket.Name, bucket.OutputDir)
		}
		fmt.Println()
	}

	// 逐个上传每个桶
	successCount := 0
	failureCount := 0

	for i, bucketSettings := range settings.Buckets {
		fmt.Printf("\n[%d/%d] 上传桶: %s\n", i+1, bucketCount, bucketSettings.Name)

		// 检查桶对应的目录是否存在
		if _, err := os.Stat(bucketSettings.OutputDir); os.IsNotExist(err) {
			fmt.Printf("桶 %s 对应的目录不存在: %s，跳过上传\n", bucketSettings.Name, bucketSettings.OutputDir)
			failureCount++
			continue
		}

		// 为每个桶创建上传选项
		options := &upload.Options{
			Endpoint:    settings.Endpoint,
			AccessKey:   settings.AccessKey,
			SecretKey:   settings.SecretKey,
			Bucket:      bucketSettings.Name,
			InputDir:    bucketSettings.OutputDir, // 从各自的输出目录上传
			Incremental: true,
			StateFile:   fmt.Sprintf(".upload_%s_state.json", bucketSettings.Name), // 每个桶独立的状态文件
			Workers:     5,
			Verbose:     verbose,
		}

		if options.Verbose {
			fmt.Printf("  端点: %s\n", options.Endpoint)
			fmt.Printf("  桶名: %s\n", options.Bucket)
			fmt.Printf("  输入目录: %s\n", options.InputDir)
			fmt.Printf("  增量上传: %v\n", options.Incremental)
			fmt.Printf("  并发数: %d\n", options.Workers)
			fmt.Printf("\n")
		}

		// 创建上传器并执行上传
		u := upload.New(options)
		if err := u.Run(); err != nil {
			fmt.Printf("桶 %s 上传失败: %v\n", bucketSettings.Name, err)
			failureCount++
			continue
		}

		fmt.Printf("桶 %s 上传完成!\n", bucketSettings.Name)
		successCount++
	}

	// 显示上传总结
	fmt.Printf("\n上传完成!\n")
	fmt.Printf("成功: %d 个桶\n", successCount)
	if failureCount > 0 {
		fmt.Printf("失败: %d 个桶\n", failureCount)
		return fmt.Errorf("部分桶上传失败")
	}

	return nil
}
