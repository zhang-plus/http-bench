package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"github.com/valyala/fasthttp"
)

// 定义测试结果结构
type TestResult struct {
	Timestamp       string           `json:"timestamp"`
	Concurrency     int              `json:"concurrency"`
	TotalRequests   int              `json:"total_requests"`
	SuccessCount    int              `json:"success_count"`
	FailCount       int              `json:"fail_count"`
	TotalDuration   string           `json:"total_duration"`
	Throughput      float64          `json:"throughput"`
	AvgResponseTime string           `json:"avg_response_time,omitempty"`
	MinResponseTime string           `json:"min_response_time,omitempty"`
	MaxResponseTime string           `json:"max_response_time,omitempty"`
	P50ResponseTime string           `json:"p50_response_time,omitempty"`
	P90ResponseTime string           `json:"p90_response_time,omitempty"`
	P99ResponseTime string           `json:"p99_response_time,omitempty"`
	TargetURL       string           `json:"target_url"`
	HTTPMethod      string           `json:"http_method"`
	ClientConfig    ClientConfigInfo `json:"client_config,omitempty"` // 客户端配置信息
}

func main() {
	// 1. 定义默认参数
	concurrency := 100
	totalRequests := 50000
	targetURL := "http://localhost:9999/hello"
	httpMethod := "GET" // 默认使用GET方法
	postBody := ""

	// 创建reader用于读取用户输入
	reader := bufio.NewReader(os.Stdin)

	// 2. 交互式输入参数
	fmt.Println("===== 接口测试参数设置 =====")
	fmt.Println("请输入以下参数值，直接按Enter键使用默认值")

	// 输入并发协程数
	fmt.Printf("并发协程数[默认: %d]: ", concurrency)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		if val, err := strconv.Atoi(input); err == nil {
			concurrency = val
		} else {
			fmt.Printf("输入无效，使用默认值: %d\n", concurrency)
		}
	}

	// 输入总请求数
	fmt.Printf("总请求数 [默认: %d]: ", totalRequests)
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		if val, err := strconv.Atoi(input); err == nil {
			totalRequests = val
		} else {
			fmt.Printf("输入无效，使用默认值: %d\n", totalRequests)
		}
	}

	// 输入目标URL
	fmt.Printf("目标URL [默认: %s]: ", targetURL)
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		targetURL = input
	}

	// 输入HTTP方法
	fmt.Printf("HTTP方法 [默认: GET, 当前支持: GET/POST]: ")
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		// 将输入转为大写以保持一致性
		input = strings.ToUpper(input)
		// 检查是否支持该方法
		supportedMethods := []string{"GET", "POST"}
		supported := false
		for _, method := range supportedMethods {
			if input == method {
				supported = true
				httpMethod = input
				break
			}
		}
		if !supported {
			fmt.Printf("当前不支持该方法，使用默认值: GET\n")
		}
	}

	// 如果选择POST方法，询问是否需要设置请求体
	if httpMethod == "POST" {
		fmt.Printf("是否需要设置POST请求体? (y/n) [默认: n]: ")
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "y" || input == "yes" {
			fmt.Printf("请输入POST请求体内容: ")
			postBody, _ = reader.ReadString('\n')
			postBody = strings.TrimSpace(postBody)
		}
	}

	// 显示最终配置信息
	fmt.Printf("\n======= 最终测试配置 =======\n")
	fmt.Printf("并发协程数: %d\n", concurrency)
	fmt.Printf("总请求数: %d\n", totalRequests)
	fmt.Printf("目标URL: %s\n", targetURL)
	fmt.Printf("HTTP方法: %s\n", httpMethod)
	if httpMethod == "POST" && postBody != "" {
		fmt.Printf("POST请求体: %s\n", postBody)
	}
	// 3. 统计变量（并发安全控制）
	var wg sync.WaitGroup
	var mutex sync.Mutex
	var successCount, failCount int
	var totalTime time.Duration
	var responseTimes []time.Duration
	var minTime, maxTime time.Duration = time.Hour, 0

	// 4. 初始化 fasthttp 客户端
	fasthttpClient := &fasthttp.Client{
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		MaxConnsPerHost: concurrency,
	}

	// 保存客户端配置值用于显示
	readTimeout := "5s"
	writeTimeout := "5s"
	maxConnsPerHost := concurrency

	startTime := time.Now()

	// 添加CPU使用率采样相关变量
	var cpuUsageSamples []float64
	var memoryUsageSamples []float64   // 程序内存占用百分比样本
	var memoryUsageMBSamples []float64 // 程序内存占用MB样本
	var cpuMutex sync.Mutex
	var memoryMutex sync.Mutex // 保护内存采样数据的互斥锁
	stopSampling := make(chan bool)

	// 启动系统资源采样协程
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond) // 每100ms采样一次
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// 获取CPU使用率
				usage, err := cpu.Percent(0, false) // 立即返回当前CPU使用率
				if err == nil && len(usage) > 0 {
					cpuMutex.Lock()
					cpuUsageSamples = append(cpuUsageSamples, usage[0])
					cpuMutex.Unlock()
				}

				// 获取程序内存占用百分比
				var memStats runtime.MemStats
				runtime.ReadMemStats(&memStats)
				memInfo, err := mem.VirtualMemory()
				if err == nil {
					systemTotalMemGB := float64(memInfo.Total) / 1024 / 1024 / 1024
					goRuntimeMemGB := float64(memStats.Sys) / 1024 / 1024 / 1024
					goRuntimeMemMB := float64(memStats.Sys) / 1024 / 1024 // 新增：MB为单位的内存使用量
					currentMemUsagePercent := (goRuntimeMemGB / systemTotalMemGB) * 100
					memoryMutex.Lock()
					memoryUsageSamples = append(memoryUsageSamples, currentMemUsagePercent)
					memoryUsageMBSamples = append(memoryUsageMBSamples, goRuntimeMemMB) // 新增：记录MB内存使用量
					memoryMutex.Unlock()
				}
			case <-stopSampling:
				return
			}
		}
	}()

	// 5. 并发请求逻辑
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reqPerGoroutine := totalRequests / concurrency

			// 复用请求/响应对象
			req := fasthttp.AcquireRequest()
			resp := fasthttp.AcquireResponse()
			defer func() {
				fasthttp.ReleaseRequest(req)
				fasthttp.ReleaseResponse(resp)
			}()
			req.SetRequestURI(targetURL)
			req.Header.SetMethod(httpMethod)

			// 如果是POST方法且有请求体，设置请求体
			if httpMethod == "POST" && postBody != "" {
				req.SetBody([]byte(postBody))
				req.Header.SetContentType("application/json")
			}

			for j := 0; j < reqPerGoroutine; j++ {
				reqStart := time.Now()

				// 发起请求
				err := fasthttpClient.Do(req, resp)

				// 耗时统计与结果处理
				reqDuration := time.Since(reqStart)
				mutex.Lock()
				totalTime += reqDuration
				mutex.Unlock()

				if err != nil {
					mutex.Lock()
					failCount++
					mutex.Unlock()
					fmt.Printf("请求失败: %v\n", err)
				} else {
					mutex.Lock()
					successCount++
					responseTimes = append(responseTimes, reqDuration)
					// 更新最小/最大响应时间
					if reqDuration < minTime {
						minTime = reqDuration
					}
					if reqDuration > maxTime {
						maxTime = reqDuration
					}
					mutex.Unlock()

					// 验证状态码（对于不同的HTTP方法，成功的状态码可能不同）
					switch httpMethod {
					case "GET":
						if resp.StatusCode() != fiber.StatusOK {
							mutex.Lock()
							failCount++
							successCount--
							mutex.Unlock()
							fmt.Printf("响应状态码异常: %d (URL: %s)\n", resp.StatusCode(), targetURL)
						}
					}
					if httpMethod == "POST" {
						// 对于POST，200和201通常都表示成功
						if resp.StatusCode() != fiber.StatusOK && resp.StatusCode() != fiber.StatusCreated {
							mutex.Lock()
							failCount++
							successCount--
							mutex.Unlock()
							fmt.Printf("响应状态码异常: %d (URL: %s)\n", resp.StatusCode(), targetURL)
						}
					}
				}
			}
		}()
	}

	// 6. 结果输出
	wg.Wait()
	stopSampling <- true // 停止采样
	totalDuration := time.Since(startTime)
	// 计算分位数
	p50, p90, p99 := calculatePercentiles(responseTimes)

	// 计算平均CPU使用率
	var avgCPUUsage float64 = 0
	cpuMutex.Lock()
	if len(cpuUsageSamples) > 0 {
		totalCPU := 0.0
		for _, usage := range cpuUsageSamples {
			totalCPU += usage
		}

		avgCPUUsage = totalCPU / float64(len(cpuUsageSamples))
	}
	cpuMutex.Unlock()

	// 计算平均程序内存占用百分比
	var avgMemoryUsagePercent float64 = 0
	var avgMemoryUsageMB float64 = 0 // 新增：平均内存占用大小(MB)
	memoryMutex.Lock()
	if len(memoryUsageSamples) > 0 {
		totalMemory := 0.0
		for _, usage := range memoryUsageSamples {
			totalMemory += usage
		}
		avgMemoryUsagePercent = totalMemory / float64(len(memoryUsageSamples))
	}

	// 新增：计算平均内存占用大小(MB)
	if len(memoryUsageMBSamples) > 0 {
		totalMemoryMB := 0.0
		for _, usage := range memoryUsageMBSamples {
			totalMemoryMB += usage
		}
		avgMemoryUsageMB = totalMemoryMB / float64(len(memoryUsageMBSamples))
	}
	memoryMutex.Unlock()

	// 设置客户端配置信息
	// 设置客户端配置信息
	clientConfig := getClientConfigInfo()
	clientConfig.AvgCPUUsagePercent = avgCPUUsage               // 设置平均CPU使用率
	clientConfig.ProcessMemUsagePercent = avgMemoryUsagePercent // 设置平均程序内存占用百分比
	clientConfig.AvgMemoryUsageMB = avgMemoryUsageMB            // 设置平均内存占用大小

	// 准备测试结果数据
	testResult := TestResult{
		Timestamp:     time.Now().Format(time.RFC3339),
		Concurrency:   concurrency,
		TotalRequests: totalRequests,
		SuccessCount:  successCount,
		FailCount:     failCount,
		TotalDuration: totalDuration.String(),
		Throughput:    float64(totalRequests) / totalDuration.Seconds(),
		TargetURL:     targetURL,
		HTTPMethod:    httpMethod,
		ClientConfig:  clientConfig, // 添加这一行，将clientConfig设置到TestResult结构体中
	}

	if successCount > 0 {
		averageTime := totalTime / time.Duration(successCount)
		testResult.AvgResponseTime = averageTime.String()
		testResult.MinResponseTime = minTime.String()
		testResult.MaxResponseTime = maxTime.String()
		testResult.P50ResponseTime = p50.String()
		testResult.P90ResponseTime = p90.String()
		testResult.P99ResponseTime = p99.String()

		fmt.Printf("成功请求数: %d\n", successCount)
		fmt.Printf("失败请求数: %d\n", failCount)
		fmt.Printf("请求总时间: %v\n", totalDuration)
		fmt.Printf("请求响应速率: %.2f 次/秒\n", testResult.Throughput)
		fmt.Printf("平均响应时间: %v\n", averageTime)
		fmt.Printf("最小响应时间: %v\n", minTime)
		fmt.Printf("最大响应时间: %v\n", maxTime)
		fmt.Printf("P50 响应时间: %v\n", p50)
		fmt.Printf("P90 响应时间: %v\n", p90)
		fmt.Printf("P99 响应时间: %v\n", p99)
	} else {
		fmt.Printf("平均响应时间: 无（无成功请求）\n")
	}

	// 打印客户端配置信息

	fmt.Printf("\n====== 客户端配置信息 =====\n")
	fmt.Printf("操作系统: %s\n", clientConfig.OS)
	fmt.Printf("架构类型: %s\n", clientConfig.Arch)
	fmt.Printf("CPU 型号: %s\n", clientConfig.CPUModel)
	fmt.Printf("CPU 核心数: %d\n", clientConfig.NumCPU)
	fmt.Printf("Goroutine 数量: %d\n", clientConfig.NumGoroutine)
	fmt.Printf("GOMAXPROCS值: %d\n", clientConfig.GOMAXPROCS)
	// fmt.Printf("Go运行时当前使用内存: %.2f MB\n", clientConfig.CurrentAllocMB)
	// fmt.Printf("Go运行时从系统获取的内存: %.2f MB\n", clientConfig.SysMB)
	// fmt.Printf("测试程序总分配内存: %d MB\n", clientConfig.TotalAllocMB)
	fmt.Printf("系统总内存: %.2f GB\n", clientConfig.SystemTotalMemGB)
	fmt.Printf("系统可用内存: %.2f GB\n", clientConfig.SystemAvailableGB)
	fmt.Printf("系统内存使用率: %.2f%%\n", clientConfig.MemoryUsagePercent)
	fmt.Printf("测试程序内存平均占用使用率: %.4f%%\n", clientConfig.ProcessMemUsagePercent) // 保留原有百分比显示
	fmt.Printf("测试程序平均内存占用大小: %.2f MB\n", clientConfig.AvgMemoryUsageMB)       // 新增平均内存占用大小显示
	fmt.Printf("测试程序CPU平均占用使用率: %.2f%%\n", clientConfig.AvgCPUUsagePercent)
	fmt.Printf("可执行文件完整路径: %s\n", clientConfig.ExePath)
	fmt.Printf("主机名: %s\n", clientConfig.Hostname)
	fmt.Printf("\n===== HTTP 客户端配置 =====\n")
	fmt.Printf("ReadTimeout: %s (请求读取超时时间)\n", readTimeout)
	fmt.Printf("WriteTimeout: %s (请求写入超时时间)\n", writeTimeout)
	fmt.Printf("MaxConnsPerHost: %d (每个主机的最大连接数)\n", maxConnsPerHost)

	// 询问是否导出结果
	// 修改导出结果的交互逻辑，允许用户选择格式
	// 询问是否导出结果
	fmt.Printf("\n是否需要导出测试结果? (y/n) [默认: n]: ")
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "y" || input == "yes" {
		// 让用户选择导出格式
		fmt.Printf("请选择导出格式 (csv/json) [默认: json]: ")
		formatInput, _ := reader.ReadString('\n')
		formatInput = strings.TrimSpace(strings.ToLower(formatInput))
		format := "json"
		if formatInput == "csv" {
			format = "csv"
		}

		// 根据选择的格式生成默认文件名
		extension := "json"
		if format == "csv" {
			extension = "csv"
		}
		defaultFilename := fmt.Sprintf("benchmark_result_%s.%s", time.Now().Format("20060102_150405"), extension)

		fmt.Printf("请输入导出文件路径 [默认: %s]: ", defaultFilename)
		filePathInput, _ := reader.ReadString('\n')
		filePathInput = strings.TrimSpace(filePathInput)
		filePath := defaultFilename
		if filePathInput != "" {
			filePath = filePathInput
		}

		// 导出结果 - 使用用户选择的格式
		err := exportResults(testResult, filePath, format)
		if err != nil {
			fmt.Printf("导出结果失败: %v\n", err)
		} else {
			fmt.Printf("测试结果已成功导出到: %s\n", filePath)
		}
	}

	// 等待用户输入，保持CMD窗口不关闭
	fmt.Println("\n按Enter键退出...")
	reader.ReadString('\n')
}

// 1. 在ClientConfigInfo结构体中添加CPUModel字段
type ClientConfigInfo struct {
	OS           string `json:"os"`            // 操作系统
	Arch         string `json:"arch"`          // 架构类型
	NumCPU       int    `json:"num_cpu"`       // CPU核心数
	CPUModel     string `json:"cpu_model"`     // CPU型号
	NumGoroutine int    `json:"num_goroutine"` // Goroutine数量
	GOMAXPROCS   int    `json:"gomaxprocs"`    // GOMAXPROCS值
	// CurrentAllocMB         float64 `json:"current_alloc_mb"`          // Go运行时当前使用内存(MB)
	// SysMB                  float64 `json:"sys_mb"`                    // Go运行时从系统获取的内存(MB)
	// TotalAllocMB           int64   `json:"total_alloc_mb"`            // 总分配内存(MB)
	SystemTotalMemGB       float64 `json:"system_total_mem_gb"`       // 系统总内存(GB)
	SystemAvailableGB      float64 `json:"system_available_gb"`       // 系统可用内存(GB)
	MemoryUsagePercent     float64 `json:"memory_usage_percent"`      // 内存使用率(%)
	ExePath                string  `json:"exe_path"`                  // 可执行文件路径
	Hostname               string  `json:"hostname"`                  // 主机名
	AvgCPUUsagePercent     float64 `json:"avg_cpu_usage_percent"`     // 平均CPU使用率(%)
	ProcessMemUsagePercent float64 `json:"process_mem_usage_percent"` // 程序内存占用百分比(%)
	AvgMemoryUsageMB       float64 `json:"avg_memory_usage_mb"`       // 平均内存占用大小(MB)
}

// 2. 在getClientConfigInfo函数中获取并设置CPU型号
func getClientConfigInfo() ClientConfigInfo {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 获取系统内存信息
	memInfo, err := mem.VirtualMemory()
	systemTotalMemGB := 0.0
	systemAvailableGB := 0.0
	memoryUsagePercent := 0.0
	processMemUsagePercent := 0.0 // 程序内存占用百分比
	if err == nil {
		systemTotalMemGB = float64(memInfo.Total) / 1024 / 1024 / 1024
		systemAvailableGB = float64(memInfo.Available) / 1024 / 1024 / 1024
		memoryUsagePercent = memInfo.UsedPercent

		// 计算程序内存占用百分比 = (Go运行时从系统获取的内存 / 系统总内存) * 100%
		// 注意单位转换：Go运行时内存是MB，系统总内存是GB
		goRuntimeMemGB := float64(memStats.Sys) / 1024 / 1024 / 1024
		processMemUsagePercent = (goRuntimeMemGB / systemTotalMemGB) * 100
	}

	// 获取可执行文件路径
	exePath := ""
	exePathValue, err := os.Executable()
	if err == nil {
		exePath = exePathValue
	}

	// 获取主机名
	hostname := ""
	hostnameValue, err := os.Hostname()
	if err == nil {
		hostname = hostnameValue
	}

	// 获取CPU型号
	cpuModel := ""
	cpuInfo, err := cpu.Info()
	if err == nil && len(cpuInfo) > 0 {
		cpuModel = cpuInfo[0].ModelName
	}

	return ClientConfigInfo{
		OS:                     runtime.GOOS,
		Arch:                   runtime.GOARCH,
		NumCPU:                 runtime.NumCPU(),
		CPUModel:               cpuModel,
		NumGoroutine:           runtime.NumGoroutine(),
		GOMAXPROCS:             runtime.GOMAXPROCS(0),
		SystemTotalMemGB:       systemTotalMemGB,
		SystemAvailableGB:      systemAvailableGB,
		MemoryUsagePercent:     memoryUsagePercent,
		ProcessMemUsagePercent: processMemUsagePercent,
		ExePath:                exePath,
		Hostname:               hostname,
		AvgCPUUsagePercent:     0.0, // 初始化字段，实际值会在后续设置
		AvgMemoryUsageMB:       0.0, // 初始化字段，实际值会在后续设置
	}
}

// 3. 在exportResults函数的CSV导出中添加CPUModel
func exportResults(result TestResult, filePath string, format string) error {
	// 创建文件
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 根据格式导出
	switch format {
	case "json":
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ") // 美化输出
		return encoder.Encode(result)
	case "csv":
		writer := csv.NewWriter(file)
		defer writer.Flush()

		// 写入CSV头部 - 确保与TestResult和ClientConfigInfo结构体字段完全对应
		headers := []string{"Timestamp", "Concurrency", "TotalRequests", "SuccessCount", "FailCount", "TotalDuration", "Throughput",
			"AvgResponseTime", "MinResponseTime", "MaxResponseTime",
			"P50ResponseTime", "P90ResponseTime", "P99ResponseTime",
			"TargetURL", "HTTPMethod",
			"OS", "Arch", "NumCPU", "CPUModel", "NumGoroutine", "GOMAXPROCS",
			"SystemTotalMemGB", "SystemAvailableGB", "MemoryUsagePercent",
			"AvgCPUUsagePercent", "ProcessMemUsagePercent", "AvgMemoryUsageMB",
			"ExePath", "Hostname"}
		if err := writer.Write(headers); err != nil {
			return err
		}

		// 写入数据行 - 确保与headers一一对应
		dataRow := []string{
			result.Timestamp,
			strconv.Itoa(result.Concurrency),
			strconv.Itoa(result.TotalRequests),
			strconv.Itoa(result.SuccessCount),
			strconv.Itoa(result.FailCount),
			result.TotalDuration,
			fmt.Sprintf("%.2f", result.Throughput),
			result.AvgResponseTime,
			result.MinResponseTime,
			result.MaxResponseTime,
			result.P50ResponseTime,
			result.P90ResponseTime,
			result.P99ResponseTime,
			result.TargetURL,
			result.HTTPMethod,
			result.ClientConfig.OS,
			result.ClientConfig.Arch,
			strconv.Itoa(result.ClientConfig.NumCPU),
			result.ClientConfig.CPUModel,
			strconv.Itoa(result.ClientConfig.NumGoroutine),
			strconv.Itoa(result.ClientConfig.GOMAXPROCS),
			fmt.Sprintf("%.2f", result.ClientConfig.SystemTotalMemGB),
			fmt.Sprintf("%.2f", result.ClientConfig.SystemAvailableGB),
			fmt.Sprintf("%.2f", result.ClientConfig.MemoryUsagePercent),
			fmt.Sprintf("%.2f", result.ClientConfig.AvgCPUUsagePercent),
			fmt.Sprintf("%.4f", result.ClientConfig.ProcessMemUsagePercent),
			fmt.Sprintf("%.2f", result.ClientConfig.AvgMemoryUsageMB),
			result.ClientConfig.ExePath,
			result.ClientConfig.Hostname,
		}
		return writer.Write(dataRow)
	default:
		return fmt.Errorf("不支持的导出格式: %s", format)
	}
}

// 计算响应时间分位数
func calculatePercentiles(times []time.Duration) (p50, p90, p99 time.Duration) {
	if len(times) == 0 {
		return 0, 0, 0
	}

	sort.Slice(times, func(i, j int) bool {
		return times[i] < times[j]
	})

	p50Index := int(float64(len(times)) * 0.5)
	p90Index := int(float64(len(times)) * 0.9)
	p99Index := int(float64(len(times)) * 0.99)

	return times[p50Index], times[p90Index], times[p99Index]
}
