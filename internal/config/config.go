package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config 应用配置结构
type Config struct {
	Ceph   CephConfig       `mapstructure:"ceph" yaml:"ceph"`
	Backup BackupFileConfig `mapstructure:"backup" yaml:"backup"`
}

// CephConfig Ceph连接配置
type CephConfig struct {
	Endpoint  string `mapstructure:"endpoint" yaml:"endpoint"`
	AccessKey string `mapstructure:"access_key" yaml:"access_key"`
	SecretKey string `mapstructure:"secret_key" yaml:"secret_key"`
	Bucket    string `mapstructure:"bucket" yaml:"bucket"`
}

// BackupFileConfig 备份文件配置
type BackupFileConfig struct {
	OutputDir   string `mapstructure:"output_dir" yaml:"output_dir"`
	Incremental bool   `mapstructure:"incremental" yaml:"incremental"`
	StateFile   string `mapstructure:"state_file" yaml:"state_file"`
	Workers     int    `mapstructure:"workers" yaml:"workers"`
	Verbose     bool   `mapstructure:"verbose" yaml:"verbose"`
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

// 默认配置文件内容
const defaultConfigContent = `# Ceph Object Storage Incremental Backup Tool Configuration
# Please modify the following configuration according to your environment

# Ceph Object Storage Configuration
ceph:
  endpoint: "http://192.168.1.100:7480"  # Ceph RGW endpoint URL
  access_key: "your-access-key"          # Access key
  secret_key: "your-secret-key"          # Secret key
  bucket: "your-bucket-name"             # Bucket name to backup

# Backup Configuration
backup:
  output_dir: "./backup"                 # Local output directory
  incremental: true                      # Enable incremental backup
  state_file: ".backup_state.json"       # State file path
  workers: 5                             # Number of concurrent download workers
  verbose: false                         # Verbose output

# Optional Filter Configuration
filters:
  # Include file patterns (supports wildcards)
  include:
    - "*.jpg"
    - "*.png"
    - "*.pdf"
    - "documents/*"
  
  # Exclude file patterns (supports wildcards)
  exclude:
    - "*.tmp"
    - "temp/*"
    - ".DS_Store"

# Retry Configuration
retry:
  max_attempts: 3                        # Maximum retry attempts
  delay: "5s"                           # Retry delay
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
		fmt.Printf("✅ 默认配置文件已创建: %s\n", cm.configPath)
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

	// 备份配置默认值
	viper.SetDefault("backup.output_dir", "./backup")
	viper.SetDefault("backup.incremental", true)
	viper.SetDefault("backup.state_file", ".backup_state.json")
	viper.SetDefault("backup.workers", 5)
	viper.SetDefault("backup.verbose", false)
}

// ValidateConfig 验证配置
func (cm *ConfigManager) ValidateConfig() error {
	if cm.config.Ceph.Endpoint == "" || cm.config.Ceph.Endpoint == "http://192.168.1.100:7480" {
		return fmt.Errorf("请在配置文件中设置正确的 ceph.endpoint")
	}
	if cm.config.Ceph.AccessKey == "" || cm.config.Ceph.AccessKey == "your-access-key" {
		return fmt.Errorf("请在配置文件中设置正确的 ceph.access_key")
	}
	if cm.config.Ceph.SecretKey == "" || cm.config.Ceph.SecretKey == "your-secret-key" {
		return fmt.Errorf("请在配置文件中设置正确的 ceph.secret_key")
	}
	if cm.config.Ceph.Bucket == "" || cm.config.Ceph.Bucket == "your-bucket-name" {
		return fmt.Errorf("请在配置文件中设置正确的 ceph.bucket")
	}
	return nil
}

// ToBackupSettings 将配置转换为备份设置
func (cm *ConfigManager) ToBackupSettings() *BackupSettings {
	return &BackupSettings{
		Endpoint:    cm.config.Ceph.Endpoint,
		AccessKey:   cm.config.Ceph.AccessKey,
		SecretKey:   cm.config.Ceph.SecretKey,
		Bucket:      cm.config.Ceph.Bucket,
		OutputDir:   viper.GetString("backup.output_dir"),
		Incremental: viper.GetBool("backup.incremental"),
		StateFile:   viper.GetString("backup.state_file"),
		Workers:     viper.GetInt("backup.workers"),
		Verbose:     viper.GetBool("backup.verbose"),
	}
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
