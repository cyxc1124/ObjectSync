# ObjectSync 更新日志

所有重要的更新都会记录在这个文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
项目遵循 [语义化版本](https://semver.org/spec/v2.0.0.html)。

## [0.0.1] - 2024-12-28

### 🎉 首次发布

这是 ObjectSync 对象存储下载工具的首个版本！一个用Go编写的对象存储批量下载工具，支持全量和增量下载。

### ✨ 核心功能

- **📥 对象存储下载**: 批量下载S3兼容对象存储中的所有文件到本地
- **🔄 增量备份**: 基于ETag和修改时间的智能增量检测，避免重复下载
- **⚡ 并发下载**: 可配置的多线程并发下载，充分利用带宽
- **📊 实时进度**: 精美的进度条显示，包含下载速度和预计时间
- **💾 状态管理**: JSON状态文件记录下载历史，支持断点续传
- **🔧 多桶支持**: 支持在单一配置文件中管理多个存储桶
- **🎮 交互式界面**: 用户友好的中文交互式菜单系统
- **⚙️ 灵活配置**: YAML配置文件，支持全局和桶级别的独立配置

### 🎨 用户体验

- **🇨🇳 完整中文界面**: 从菜单到日志输出的全中文用户体验
- **🖥️ 智能启动模式**: 无参数运行时自动启动交互式菜单
- **📱 多命令支持**: 提供`backup`、`config`、`status`、`version`等子命令
- **🔍 配置验证**: 自动验证连接配置，启动前检测问题
- **📋 详细状态**: 查看备份历史、文件统计和上次运行状态

### 🔧 技术架构

- **🏗️ 模块化设计**: 采用清晰的分层架构(`cmd`/`internal`结构)
- **📚 依赖管理**: 使用`cobra`CLI框架和`viper`配置管理
- **☁️ S3兼容性**: 基于AWS SDK Go，支持所有S3兼容存储
- **🧩 跨平台**: 支持Windows、Linux、macOS的AMD64和ARM64架构
- **🔒 安全连接**: 支持HTTPS连接和访问密钥认证
- **🎯 UTF-8编码**: Windows平台自动UTF-8控制台设置

### 📦 配置结构

```yaml
# 对象存储连接配置
ceph:
  endpoint: "http://your-storage-endpoint:7480"
  access_key: "your-access-key"
  secret_key: "your-secret-key"

# 多桶配置支持
buckets:
  - name: "my-bucket"
    output_dir: "./backup/my-bucket"
    state_file: ".backup_state.json"
    workers: 8        # 可选：桶级别并发数
    verbose: true     # 可选：桶级别详细输出

# 全局备份配置
backup:
  incremental: true   # 启用增量备份
  workers: 5         # 默认并发数
  verbose: false     # 详细输出
```

### 🚀 使用方式

#### 交互式模式（推荐）
```bash
# 直接运行启动交互菜单
objectsync

# 或明确调用菜单命令
objectsync menu
```

#### 命令行模式
```bash
# 初始化配置
objectsync config init

# 验证配置
objectsync config validate

# 开始备份
objectsync backup --verbose

# 查看状态
objectsync status

# 查看版本
objectsync version
```

### 🛠️ 构建与发布

- **📦 Makefile支持**: 完整的构建、测试、发布工具链
- **🤖 GitHub Actions**: 自动多平台构建和发布
- **📋 动态Changelog**: 从CHANGELOG.md自动生成发布说明
- **🏷️ 版本注入**: 构建时自动注入版本、时间和Git提交信息
- **🗜️ 压缩优化**: 构建时使用`-s -w`标志减小文件体积

### 🔗 兼容性

支持所有S3兼容的对象存储服务：
- **AWS S3**: 原生支持
- **Ceph Object Gateway (RGW)**: 完整支持
- **MinIO**: 完整支持  
- **阿里云OSS**: S3兼容API支持
- **腾讯云COS**: S3兼容API支持
- **华为云OBS**: S3兼容API支持

### 🎯 适用场景

- **数据备份**: 定期将对象存储数据同步到本地
- **数据迁移**: 从对象存储批量迁移到本地存储
- **离线分析**: 下载数据到本地进行分析处理
- **灾难恢复**: 快速恢复对象存储数据
- **开发测试**: 获取生产数据到开发环境

### 🚧 已知限制

- 仅支持下载方向（对象存储→本地），暂不支持上传
- 状态文件基于JSON格式，大量文件时可能较大
- 目前仅支持桶级别的配置，不支持前缀过滤

---

## [Unreleased]

### 计划中的功能
- 增量同步优化
- 更多存储后端支持 