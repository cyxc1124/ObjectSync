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
		Short: "对象存储下载工具",
		Long:  "一个用于从S3兼容对象存储下载数据到本地的增量下载工具",
		RunE:  a.runDefault, // 智能默认行为
	}

	// 添加子命令
	a.rootCmd.AddCommand(a.newBackupCmd())
	a.rootCmd.AddCommand(a.newConfigCmd())
	a.rootCmd.AddCommand(a.newStatusCmd())
	a.rootCmd.AddCommand(a.newMenuCmd()) // 添加交互式菜单命令
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

	// 检查是多桶模式还是单桶模式
	if configManager.IsMultiBucketMode() {
		return a.runMultiBucketBackup(configManager, endpoint, accessKey, secretKey, incremental, verbose)
	} else {
		return a.runSingleBucketBackup(configManager, endpoint, accessKey, secretKey, bucket, outputDir, stateFile, incremental, verbose, workers)
	}
}

// runSingleBucketBackup 执行单桶备份
func (a *App) runSingleBucketBackup(configManager *config.ConfigManager, endpoint, accessKey, secretKey, bucket, outputDir, stateFile string, incremental, verbose bool, workers int) error {
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

	fmt.Printf("开始备份桶: %s\n", options.Bucket)
	if options.Verbose {
		fmt.Printf("配置信息:\n")
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
		return fmt.Errorf("备份失败: %w", err)
	}

	fmt.Println("备份完成!")
	return nil
}

// runMultiBucketBackup 执行多桶备份
func (a *App) runMultiBucketBackup(configManager *config.ConfigManager, endpoint, accessKey, secretKey string, incremental, verbose bool) error {
	// 获取多桶配置
	settings := configManager.ToMultiBucketSettings()

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

	fmt.Printf("开始多桶备份（共 %d 个桶）\n", len(settings.Buckets))
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
		fmt.Printf("\n[%d/%d] 备份桶: %s\n", i+1, len(settings.Buckets), bucketSettings.Name)

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

	// 显示总结
	fmt.Printf("\n多桶备份完成!\n")
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
	settings := configManager.ToBackupSettings()
	options := &backup.Options{
		Endpoint:  settings.Endpoint,
		AccessKey: settings.AccessKey,
		SecretKey: settings.SecretKey,
		Bucket:    settings.Bucket,
	}

	b := backup.New(options)
	if err := b.TestConnection(); err != nil {
		fmt.Printf("连接失败: %v\n", err)
		return err
	}

	fmt.Printf("连接成功!\n")
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
	fmt.Printf("状态文件: %s\n", stateFile)
	fmt.Println()

	// 先检查配置文件是否存在
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Printf("配置文件不存在，请先进行配置初始化\n")
		return nil
	}

	// 尝试加载配置以获取正确的状态文件路径
	configManager := config.NewConfigManager(configFile)
	if _, err := configManager.LoadConfig(); err == nil {
		// 成功加载配置，检查是否为多桶模式
		if configManager.IsMultiBucketMode() {
			fmt.Println("检测到多桶配置，显示所有桶的状态:")
			settings := configManager.ToMultiBucketSettings()

			for i, bucket := range settings.Buckets {
				fmt.Printf("\n[%d] 桶: %s\n", i+1, bucket.Name)
				fmt.Printf("    状态文件: %s\n", bucket.StateFile)

				if err := a.showBucketStatus(bucket.StateFile); err != nil {
					fmt.Printf("    读取状态失败: %v\n", err)
				}
			}
			return nil
		} else {
			// 单桶模式，使用配置中的状态文件
			settings := configManager.ToBackupSettings()
			stateFile = settings.StateFile
			fmt.Printf("更新状态文件路径: %s\n", stateFile)
		}
	} else {
		fmt.Printf("配置文件加载失败: %v\n", err)
		fmt.Printf("使用默认状态文件: %s\n", stateFile)
	}

	// 显示单桶状态
	return a.showBucketStatus(stateFile)
}

// showBucketStatus 显示单个桶的备份状态
func (a *App) showBucketStatus(stateFile string) error {
	// 检查状态文件是否存在
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		fmt.Printf("    状态文件不存在，可能是首次备份\n")
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
	fmt.Printf("    最后备份时间: %s\n", state.LastBackup.Format("2006-01-02 15:04:05"))
	fmt.Printf("    已备份文件数: %d\n", len(state.Files))

	// 计算总大小
	var totalSize int64
	for _, file := range state.Files {
		totalSize += file.Size
	}
	fmt.Printf("    总数据大小: %s\n", progress.FormatSize(totalSize))

	// 显示最近的几个文件
	fmt.Println("    最近备份的文件:")
	count := 0
	for filename, fileState := range state.Files {
		if count >= 3 { // 在菜单模式下显示少一些文件
			break
		}
		fmt.Printf("      %s (%s, %s)\n",
			filename,
			progress.FormatSize(fileState.Size),
			fileState.LastModified.Format("2006-01-02 15:04:05"))
		count++
	}

	if len(state.Files) > 3 {
		fmt.Printf("      ... 还有 %d 个文件\n", len(state.Files)-3)
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

	// 选择配置模式
	fmt.Println("\n选择配置模式:")
	fmt.Println("1. 单桶模式 - 只备份一个桶")
	fmt.Println("2. 多桶模式 - 备份多个桶 (推荐)")
	fmt.Print("请选择 (1/2, 默认: 2): ")
	var modeChoice string
	fmt.Scanln(&modeChoice)

	var configContent string

	if modeChoice == "1" {
		// 单桶模式
		configContent = a.generateSingleBucketConfig(endpoint, accessKey, secretKey, workers, incremental, verbose)
	} else {
		// 多桶模式（默认）
		configContent = a.generateMultiBucketConfig(endpoint, accessKey, secretKey, workers, incremental, verbose)
	}

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

	// 选择配置模式
	fmt.Println("\n选择配置模式:")
	fmt.Println("1. 单桶模式 - 只备份一个桶")
	fmt.Println("2. 多桶模式 - 备份多个桶 (推荐)")
	fmt.Print("请选择 (1/2, 默认: 2): ")
	var modeChoice string
	fmt.Scanln(&modeChoice)

	var configContent string

	if modeChoice == "1" {
		// 单桶模式
		configContent = a.generateSingleBucketConfig(endpoint, accessKey, secretKey, workers, incremental, verbose)
	} else {
		// 多桶模式（默认）
		configContent = a.generateMultiBucketConfig(endpoint, accessKey, secretKey, workers, incremental, verbose)
	}

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

// generateSingleBucketConfig 生成单桶配置
func (a *App) generateSingleBucketConfig(endpoint, accessKey, secretKey string, workers int, incremental, verbose bool) string {
	return fmt.Sprintf(`# ObjectSync - 对象存储下载工具配置文件（单桶模式）
# 由交互式初始化生成

# 对象存储连接配置
ceph:
  endpoint: "%s"
  access_key: "%s"
  secret_key: "%s"
  bucket: "your-bucket-name"              # 请修改为实际的桶名称

# 备份配置
backup:
  output_dir: "./backup"                  # 请修改为实际的输出目录
  incremental: %t
  state_file: ".backup_state.json"
  workers: %d
  verbose: %t

# 重试配置
retry:
  max_attempts: 3
  delay: "5s"
`, endpoint, accessKey, secretKey, incremental, workers, verbose)
}

// generateMultiBucketConfig 生成多桶配置
func (a *App) generateMultiBucketConfig(endpoint, accessKey, secretKey string, workers int, incremental, verbose bool) string {
	return fmt.Sprintf(`# ObjectSync - 对象存储下载工具配置文件（多桶模式）
# 由交互式初始化生成

# 对象存储连接配置
ceph:
  endpoint: "%s"
  access_key: "%s"
  secret_key: "%s"

# 多桶配置 - 请根据实际情况修改桶名称和输出目录
buckets:
  - name: "documents"                     # 修改为实际的桶名称
    output_dir: "./backup/documents"      # 修改为实际的输出目录
    state_file: ".state_documents.json"
  - name: "photos"
    output_dir: "./backup/photos"
    state_file: ".state_photos.json"
  - name: "videos"
    output_dir: "./backup/videos"
    state_file: ".state_videos.json"
    workers: 8                            # 可选：为特定桶设置不同的并发数
    verbose: true                         # 可选：为特定桶启用详细输出

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
		fmt.Println("[3] 查看状态")
		fmt.Println("[4] 查看配置")
		fmt.Println("[5] 查看帮助")
		fmt.Println("[0] 退出")
		fmt.Println()
		fmt.Print("请选择操作 (0-5): ")

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
			// 查看状态
			fmt.Println("[信息] 查看备份状态...")
			if err := a.runStatusMenu(); err != nil {
				fmt.Printf("查看状态失败: %v\n", err)
			}
			a.pauseAndContinue()

		case "4":
			// 查看配置
			fmt.Println("[信息] 当前配置文件内容:")
			fmt.Println("========================================")
			a.showCurrentConfig()
			fmt.Println("========================================")
			a.pauseAndContinue()

		case "5":
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
