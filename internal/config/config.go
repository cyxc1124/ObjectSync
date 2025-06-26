package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config 主配置结构
type Config struct {
	Ceph    CephConfig       `mapstructure:"ceph" yaml:"ceph"`
	Backup  BackupFileConfig `mapstructure:"backup" yaml:"backup"`
	Bucket  string           `mapstructure:"bucket" yaml:"bucket,omitempty"`   // 单桶模式（向后兼容）
	Buckets []BucketConfig   `mapstructure:"buckets" yaml:"buckets,omitempty"` // 多桶模式
}

// CephConfig Ceph连接配置
type CephConfig struct {
	Endpoint  string `mapstructure:"endpoint" yaml:"endpoint"`
	AccessKey string `mapstructure:"access_key" yaml:"access_key"`
	SecretKey string `mapstructure:"secret_key" yaml:"secret_key"`
	Bucket    string `mapstructure:"bucket" yaml:"bucket,omitempty"` // 单桶模式（向后兼容）
}

// BackupFileConfig 备份文件配置
type BackupFileConfig struct {
	OutputDir   string `mapstructure:"output_dir" yaml:"output_dir"`
	Incremental bool   `mapstructure:"incremental" yaml:"incremental"`
	StateFile   string `mapstructure:"state_file" yaml:"state_file"`
	Workers     int    `mapstructure:"workers" yaml:"workers"`
	Verbose     bool   `mapstructure:"verbose" yaml:"verbose"`
}

// BucketConfig 单个桶的配置
type BucketConfig struct {
	Name      string `mapstructure:"name" yaml:"name"`
	OutputDir string `mapstructure:"output_dir" yaml:"output_dir"`
	StateFile string `mapstructure:"state_file" yaml:"state_file,omitempty"`
	Workers   int    `mapstructure:"workers" yaml:"workers,omitempty"`
	Verbose   bool   `mapstructure:"verbose" yaml:"verbose,omitempty"`
}

// BackupSettings 备份设置 (重命名原来的BackupConfig为BackupSettings)
type BackupSettings struct {
	Endpoint    string
	AccessKey   string
	SecretKey   string
	Bucket      string
	OutputDir   string
	Incremental bool
	StateFile   string
	Workers     int
	Verbose     bool
	ConfigFile  string
}

// MultiBucketSettings 多桶备份设置
type MultiBucketSettings struct {
	Endpoint    string
	AccessKey   string
	SecretKey   string
	Buckets     []BucketSettings
	Incremental bool
	ConfigFile  string
}

// BucketSettings 单个桶的备份设置
type BucketSettings struct {
	Name      string
	OutputDir string
	StateFile string
	Workers   int
	Verbose   bool
}

// 默认配置文件内容
const defaultConfigContent = `# ObjectSync - 对象存储下载工具配置文件
# 支持单桶和多桶两种配置模式

# 对象存储连接配置
ceph:
  endpoint: "http://192.168.1.100:7480"  # 对象存储端点URL
  access_key: "your-access-key"          # 访问密钥
  secret_key: "your-secret-key"          # 秘密密钥

# 方式一：单桶模式（向后兼容）
# 取消注释下面的配置以使用单桶模式
# bucket: "your-bucket-name"

# 方式二：多桶模式（推荐）
# 可以在一个配置文件中配置多个桶
buckets:
  - name: "documents"                    # 桶名称
    output_dir: "./backup/documents"     # 本地输出目录
    state_file: ".state_documents.json" # 状态文件路径
  - name: "photos"
    output_dir: "./backup/photos"
    state_file: ".state_photos.json"
  - name: "videos"
    output_dir: "./backup/videos"
    state_file: ".state_videos.json"

# 全局备份配置
backup:
  incremental: true                      # 启用增量备份
  workers: 5                             # 默认并发下载数
  verbose: false                         # 详细输出

# 重试配置
retry:
  max_attempts: 3                        # 最大重试次数
  delay: "5s"                           # 重试延迟
`

// ConfigManager 配置管理器
type ConfigManager struct {
	configPath string
	config     *Config
}

// NewConfigManager 创建配置管理器
func NewConfigManager(configPath string) *ConfigManager {
	if configPath == "" {
		configPath = "config.yaml"
	}
	return &ConfigManager{
		configPath: configPath,
		config:     &Config{},
	}
}

// LoadConfig 加载配置文件
func (cm *ConfigManager) LoadConfig() (*Config, error) {
	// 检查配置文件是否存在，不存在则创建默认配置文件
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		fmt.Printf("配置文件 %s 不存在，正在创建默认配置文件...\n", cm.configPath)
		if err := cm.createDefaultConfig(); err != nil {
			return nil, fmt.Errorf("创建默认配置文件失败: %w", err)
		}
		fmt.Printf("默认配置文件已创建: %s\n", cm.configPath)
		fmt.Printf("请编辑配置文件并填入正确的Ceph连接信息，然后重新运行程序。\n")
		return nil, fmt.Errorf("请先配置 %s 文件", cm.configPath)
	}

	// 设置配置文件路径和类型
	viper.SetConfigFile(cm.configPath)
	viper.SetConfigType("yaml")

	// 设置默认值
	cm.setDefaults()

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 将配置解析到结构体
	if err := viper.Unmarshal(cm.config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return cm.config, nil
}

// createDefaultConfig 创建默认配置文件
func (cm *ConfigManager) createDefaultConfig() error {
	// 确保配置文件目录存在
	dir := filepath.Dir(cm.configPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// 写入默认配置内容
	file, err := os.Create(cm.configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(defaultConfigContent)
	return err
}

// setDefaults 设置默认值
func (cm *ConfigManager) setDefaults() {
	// Ceph配置默认值
	viper.SetDefault("ceph.endpoint", "")
	viper.SetDefault("ceph.access_key", "")
	viper.SetDefault("ceph.secret_key", "")
	viper.SetDefault("ceph.bucket", "")

	// 单桶模式兼容
	viper.SetDefault("bucket", "")

	// 备份配置默认值
	viper.SetDefault("backup.output_dir", "./backup")
	viper.SetDefault("backup.incremental", true)
	viper.SetDefault("backup.state_file", ".backup_state.json")
	viper.SetDefault("backup.workers", 5)
	viper.SetDefault("backup.verbose", false)
}

// ValidateConfig 验证配置
func (cm *ConfigManager) ValidateConfig() error {
	// 验证基础连接配置
	if cm.config.Ceph.Endpoint == "" || cm.config.Ceph.Endpoint == "http://192.168.1.100:7480" {
		return fmt.Errorf("请在配置文件中设置正确的 ceph.endpoint")
	}
	if cm.config.Ceph.AccessKey == "" || cm.config.Ceph.AccessKey == "your-access-key" {
		return fmt.Errorf("请在配置文件中设置正确的 ceph.access_key")
	}
	if cm.config.Ceph.SecretKey == "" || cm.config.Ceph.SecretKey == "your-secret-key" {
		return fmt.Errorf("请在配置文件中设置正确的 ceph.secret_key")
	}

	// 验证桶配置（单桶或多桶至少要有一种）
	singleBucket := cm.config.Ceph.Bucket != "" && cm.config.Ceph.Bucket != "your-bucket-name"
	if !singleBucket {
		singleBucket = cm.config.Bucket != "" && cm.config.Bucket != "your-bucket-name"
	}

	multiBuckets := len(cm.config.Buckets) > 0

	if !singleBucket && !multiBuckets {
		return fmt.Errorf("请配置要备份的桶：使用 ceph.bucket（单桶）或 buckets（多桶）")
	}

	// 验证多桶配置
	if multiBuckets {
		for i, bucket := range cm.config.Buckets {
			if bucket.Name == "" {
				return fmt.Errorf("buckets[%d] 缺少桶名称", i)
			}
			if bucket.OutputDir == "" {
				return fmt.Errorf("buckets[%d] 缺少输出目录", i)
			}
		}
	}

	return nil
}

// IsMultiBucketMode 检查是否为多桶模式
func (cm *ConfigManager) IsMultiBucketMode() bool {
	return len(cm.config.Buckets) > 0
}

// ToBackupSettings 将配置转换为备份设置（单桶模式）
func (cm *ConfigManager) ToBackupSettings() *BackupSettings {
	bucket := cm.config.Ceph.Bucket
	if bucket == "" {
		bucket = cm.config.Bucket
	}

	return &BackupSettings{
		Endpoint:    cm.config.Ceph.Endpoint,
		AccessKey:   cm.config.Ceph.AccessKey,
		SecretKey:   cm.config.Ceph.SecretKey,
		Bucket:      bucket,
		OutputDir:   viper.GetString("backup.output_dir"),
		Incremental: viper.GetBool("backup.incremental"),
		StateFile:   viper.GetString("backup.state_file"),
		Workers:     viper.GetInt("backup.workers"),
		Verbose:     viper.GetBool("backup.verbose"),
	}
}

// ToMultiBucketSettings 将配置转换为多桶备份设置
func (cm *ConfigManager) ToMultiBucketSettings() *MultiBucketSettings {
	settings := &MultiBucketSettings{
		Endpoint:    cm.config.Ceph.Endpoint,
		AccessKey:   cm.config.Ceph.AccessKey,
		SecretKey:   cm.config.Ceph.SecretKey,
		Incremental: viper.GetBool("backup.incremental"),
		ConfigFile:  cm.configPath,
	}

	// 转换桶配置
	for _, bucketConfig := range cm.config.Buckets {
		bucketSettings := BucketSettings{
			Name:      bucketConfig.Name,
			OutputDir: bucketConfig.OutputDir,
			StateFile: bucketConfig.StateFile,
			Workers:   bucketConfig.Workers,
			Verbose:   bucketConfig.Verbose,
		}

		// 使用全局默认值填充未设置的字段
		if bucketSettings.StateFile == "" {
			bucketSettings.StateFile = fmt.Sprintf(".backup_state_%s.json", bucketConfig.Name)
		}
		if bucketSettings.Workers == 0 {
			bucketSettings.Workers = viper.GetInt("backup.workers")
		}
		if !bucketSettings.Verbose {
			bucketSettings.Verbose = viper.GetBool("backup.verbose")
		}

		settings.Buckets = append(settings.Buckets, bucketSettings)
	}

	return settings
}

// OverrideWithFlags 用命令行参数覆盖配置
func (settings *BackupSettings) OverrideWithFlags(
	endpoint, accessKey, secretKey, bucket, outputDir, stateFile string,
	incremental, verbose bool, workers int,
) {
	if endpoint != "" {
		settings.Endpoint = endpoint
	}
	if accessKey != "" {
		settings.AccessKey = accessKey
	}
	if secretKey != "" {
		settings.SecretKey = secretKey
	}
	if bucket != "" {
		settings.Bucket = bucket
	}
	if outputDir != "./backup" {
		settings.OutputDir = outputDir
	}
	if stateFile != ".backup_state.json" {
		settings.StateFile = stateFile
	}
	if workers != 5 {
		settings.Workers = workers
	}
	// incremental 和 verbose 的默认值处理需要特殊逻辑
	settings.Incremental = incremental
	settings.Verbose = verbose
}
