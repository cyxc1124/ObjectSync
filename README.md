# ObjectSync - 对象存储同步工具

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-1.24+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey.svg)](https://github.com)
[![Release](https://img.shields.io/github/v/release/cyxc1124/ObjectSync.svg)](https://github.com/cyxc1124/ObjectSync/releases)

一个用 Go 编写的对象存储下载工具，支持全量和增量下载，跨平台运行

[📖 详细文档](docs/README.md) | [🚀 快速开始](#-快速开始) | [📥 下载](https://github.com/cyxc1124/ObjectSync/releases)

</div>

---

## 🎯 核心功能

> **一句话说明：从对象存储（S3兼容）批量下载文件到本地，支持增量更新。**

ObjectSync 的**核心功能就是两件事**：

### **📥 全量下载**
- 将对象存储（S3兼容）中的所有文件下载到本地指定目录
- 保持原有的目录结构和文件名
- 支持所有 S3 兼容存储：AWS S3、Ceph、MinIO、阿里云OSS等

### **🔄 增量备份** 
- 检测已下载文件的变化（基于ETag和修改时间）
- 只下载新增或修改的文件，跳过未变化的文件
- 使用状态文件记录下载历史，实现智能增量

### **⚡ 性能优化**
- **多线程并发** - 可配置并发数，充分利用带宽
- **断点续传** - 网络中断后自动恢复下载
- **低内存占用** - 流式下载，不占用大量内存
- **跨平台支持** - Windows、Linux、macOS 全覆盖

### **🔌 兼容性**
支持所有 S3 兼容的对象存储：
- AWS S3、Ceph Object Gateway (RGW)、MinIO
- 阿里云 OSS、腾讯云 COS、华为云 OBS
- 其他符合 S3 API 标准的对象存储

## 🚀 快速开始

### 安装

从 [Releases](https://github.com/cyxc1124/ObjectSync/releases) 下载对应平台的版本，或者使用一键安装脚本：

```bash
# Linux/macOS
curl -fsSL https://raw.githubusercontent.com/cyxc1124/ObjectSync/main/scripts/install.sh | bash

# Windows (PowerShell)
iwr https://raw.githubusercontent.com/cyxc1124/ObjectSync/main/scripts/install.bat | iex
```

### 使用方法

1. **初始化配置**
   ```bash
   objectsync config init
   ```

2. **开始下载**
   ```bash
   objectsync backup --verbose
   ```

3. **查看状态**
   ```bash
   objectsync status
   ```

### 配置文件示例

```yaml
# 最简配置 - 只需填入你的存储信息
ceph:
  endpoint: "http://your-s3-endpoint:7480"    # 对象存储地址
  access_key: "your-access-key"              # 访问密钥
  secret_key: "your-secret-key"              # 秘密密钥
  bucket: "your-bucket-name"                 # 要下载的桶名

backup:
  output_dir: "./downloaded-data"            # 下载到这个目录
  incremental: true                          # 启用增量下载
  workers: 8                                 # 8个并发线程
```

## 🔧 工作原理

### **下载流程：**
1. 连接到对象存储（支持所有S3兼容存储）
2. 列出指定桶中的所有对象
3. 对比本地状态文件，确定需要下载的文件
4. 多线程并发下载文件到本地目录
5. 更新本地状态文件（`.backup_state.json`）

### **状态管理：**
- 记录每个文件的 ETag、修改时间、大小等信息
- 下次运行时对比状态，实现增量下载
- 避免重复下载，节省时间和带宽

### **技术特性：**
- ✅ 支持所有 S3 兼容对象存储
- ✅ 智能增量检测，只下载变化文件
- ✅ 多线程并发，可配置线程数
- ✅ 断点续传，网络中断不怕
- ✅ 跨平台支持 (Windows/Linux/macOS)
- ✅ 多架构支持 (AMD64/ARM64)
- ✅ 配置文件和命令行双重配置
- ✅ 实时进度显示和详细日志

## 🛠️ 构建

需要 Go 1.24+ 版本：

```bash
# 克隆代码
git clone https://github.com/cyxc1124/ObjectSync.git
cd ObjectSync

# 构建
make build

# 运行测试
make test
```

构建多平台版本：
```bash
make build-all
```

## 📚 文档

| 文档 | 说明 |
|------|------|
| [用户手册](docs/README.md) | 详细的使用指南 |
| [配置参考](docs/CONFIG_REFERENCE.md) | 所有的配置选项 |
| [部署指南](docs/DEPLOYMENT.md) | 生产环境部署 |
| [架构设计](docs/ARCHITECTURE.md) | 代码架构说明 |
| [故障排除](docs/TROUBLESHOOTING.md) | 常见问题解决 |
| [开发指南](docs/DEVELOPMENT.md) | 参与开发 |

## 📖 适用场景

这个工具特别适合以下场景：

| 场景 | 说明 | 优势 |
|------|------|------|
| **数据备份** | 定期将对象存储数据下载到本地 | 增量下载，节省时间 |
| **数据迁移** | 批量从对象存储迁移到本地存储 | 多线程加速，支持断点续传 |
| **离线分析** | 下载数据到本地进行处理分析 | 保持目录结构，便于分析 |
| **测试数据** | 快速获取测试数据到本地环境 | 配置简单，一键下载 |
| **灾难恢复** | 应急情况下快速恢复数据 | 全量下载，确保完整性 |

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

如果你发现 bug 或者有新功能建议，请：
1. 先查看 [Issues](https://github.com/cyxc1124/ObjectSync/issues) 看看是否已经有人提出
2. 如果没有，创建新的 Issue 详细描述问题
3. 如果你能修复问题，欢迎提交 PR

## 🐛 问题反馈

- 🐛 [报告 Bug](https://github.com/cyxc1124/ObjectSync/issues)
- 💡 [功能建议](https://github.com/cyxc1124/ObjectSync/issues)
- 💬 [讨论](https://github.com/cyxc1124/ObjectSync/discussions)

## 📝 更新日志

### v0.0.1 (2024-06-26)
- 🎉 首次发布
- ✨ 基本的备份功能
- 🔄 增量备份支持

查看完整的 [更新历史](CHANGELOG.md)

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件

## 🙏 致谢

感谢所有使用和贡献这个项目的朋友们！

如果这个工具对你有帮助，欢迎给个 ⭐️

---

<div align="center">

**让备份变得简单一点**

Made with ❤️ by [cyxc1124](https://github.com/cyxc1124)

</div> 