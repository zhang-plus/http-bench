# HTTP 接口性能基准测试工具

一个使用 Go 语言编写的高性能 HTTP 基准测试工具，基于go高性能并发http框架  fasthttp  fiber ,支持并发压力测试、详细的性能指标统计和结果导出功能。已编译win系统、Linux系统可执行文件便于直接使用，文件在发行版。

## 使用截图

<img width="1446" height="1288" alt="ScreenShot_Using" src="https://github.com/user-attachments/assets/39b4c08a-78e8-42c6-a679-6f6487b7509f" />
<img width="1058" height="1116" alt="ScreenShot_Using2" src="https://github.com/user-attachments/assets/fdd53d82-566b-44cf-950d-d864d7135f6e" />
<img width="2120" height="1096" alt="ScreenShot_Using3" src="https://github.com/user-attachments/assets/5c6d507b-920b-4636-8657-de537177a464" />


## 功能特性

### 🚀 核心功能
- **多并发测试**：支持自定义并发协程数和总请求数
- **HTTP 方法支持**：GET、POST 方法
- **智能结果分析**：成功率、响应时间统计、吞吐量计算
- **分位数统计**：P50、P90、P99 响应时间
- **实时资源监控**：CPU 和内存使用率监控

### 📊 详细指标
- 请求成功率/失败率
- 平均、最小、最大响应时间
- 分位数响应时间（P50、P90、P99）
- 系统吞吐量（请求/秒）
- 客户端系统资源使用情况

### 💾 数据导出
- **JSON 格式**：完整的结构化数据
- **CSV 格式**：便于电子表格分析
- **自定义文件名**：支持按时间戳自动命名

## 快速开始

### 系统要求
- Go 1.16 或更高版本
- 支持的操作系统：Windows、Linux、macOS

### 安装步骤

1. **克隆仓库**
```bash
git clone https://github.com/zhang-plus/http-bench.git
cd http-bench
```

2. **安装依赖**
```bash
go mod init http-bench
go mod tidy
```

3. **编译工具**
```bash
go build -o http-bench_v1 http-bench_v1.go
```

### 使用方法

**运行工具：**
```bash
./http-bench_v1
```

在 Windows 上：
```cmd
http-bench_v1.exe
```

## 交互式配置

运行工具后，按提示输入测试参数：

1. **并发协程数**：模拟的并发用户数（默认：100）
2. **总请求数**：要发送的总请求数量（默认：50000）
3. **目标 URL**：要测试的接口地址（默认：http://localhost:9999/hello）
4. **HTTP 方法**：GET 或 POST（默认：GET）
5. **POST 请求体**：如果选择 POST 方法，可设置请求体内容

## 示例测试场景

### 测试本地服务
```bash
目标URL: http://localhost:8080/api/users
并发数: 50
总请求数: 10000
方法: GET
```

### 测试 POST 接口
```bash
目标URL: http://api.example.com/v1/login
并发数: 100
总请求数: 20000
方法: POST
请求体: {"username": "test", "password": "123456"}
```

## 输出结果详解

### 测试结果示例
```
成功请求数: 49875
失败请求数: 125
请求总时间: 45.23s
请求响应速率: 1103.45 次/秒
平均响应时间: 23.4ms
最小响应时间: 1.2ms
最大响应时间: 450.6ms
P50 响应时间: 18.7ms
P90 响应时间: 56.3ms
P99 响应时间: 189.4ms
```

### 系统信息
```
操作系统: linux
架构类型: amd64
CPU 型号: Intel(R) Xeon(R) CPU E5-2680 v4 @ 2.40GHz
CPU 核心数: 8
系统总内存: 16.00 GB
测试程序CPU平均占用使用率: 45.23%
测试程序平均内存占用大小: 125.67 MB
```

## 项目结构

```
http-benchmark/
├── http-bench_v1.go    # 主程序文件
├── go.mod              # Go 模块文件   go mod init http-bench
├── go.sum              # 依赖校验文件  go  mod tidy
├── README.md           # 项目说明文档
```

## 技术架构

### 核心组件
- **Fasthttp**：高性能 HTTP 客户端
- **gopsutil**：系统资源监控
- **Go 原生并发**：goroutine 和 channel

### 性能优化
- 连接复用：减少 TCP 连接开销
- 对象池：复用请求/响应对象
- 内存预分配：减少 GC 压力
- 并发安全：互斥锁保护共享数据

## 命令行选项

工具默认以交互模式运行，所有参数通过交互式提示配置：

- 无需命令行标志
- 用户友好的交互界面
- 所有参数都提供默认值

## 结果导出

测试完成后，系统会提示是否导出结果：

1. 选择导出格式（JSON/CSV）
2. 指定输出文件名（可选）
3. 结果文件自动包含时间戳，便于跟踪

导出文件示例：
- `benchmark_result_20231201_143052.json`
- `benchmark_result_20231201_143052.csv`

## 开发指南

### 从源码构建
```bash
git clone https://github.com/zhang-plus/http-bench_v1.git
cd http-benchmark
go build -o http-bench_v1 http-bench_v1.go
```

### 运行测试
```bash
go run http-bench_v1.go
```

### 代码结构
- `main()`：程序入口和用户交互
- 使用 goroutine 的测试执行逻辑
- 独立协程的资源监控
- 结果计算和导出功能

## 开发计划

### 近期功能
- [ ] 支持 HTTP/2 协议
- [x] 添加请求超时配置
- [ ] 支持自定义 HTTP 头部
- [ ] 添加图形化报告生成

### 长期规划
- [ ] 分布式压力测试
- [ ] 实时监控仪表板
- [ ] 自动化测试场景
- [ ] 性能趋势分析

## 贡献指南

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 许可证

本项目采用 Apache 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情

## 技术支持

如有问题或建议，请通过以下方式联系：
- 提交 [GitHub Issue](https://github.com/zhang-plus/http-bench/issues)

## 致谢

- [Fasthttp](https://github.com/valyala/fasthttp) 提供高性能 HTTP 客户端
- [gopsutil](https://github.com/shirou/gopsutil) 提供系统指标收集
- [Fiber](https://github.com/gofiber/fiber) 提供 HTTP 工具

---

<div align="center">

**使用 Go 语言精心打造 ❤️**

</div>

---

**重要提示**：请确保在合法授权的情况下对目标系统进行压力测试，遵守相关服务的使用条款。本工具仅用于合法的性能测试和开发目的。
