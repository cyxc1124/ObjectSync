package progress

import (
	"fmt"
	"sync"
	"time"
)

// Tracker 进度跟踪器
type Tracker struct {
	totalFiles   int64
	totalSize    int64
	currentFiles int64
	currentSize  int64
	startTime    time.Time
	verbose      bool
	mutex        sync.Mutex
}

// New 创建新的进度跟踪器
func New(verbose bool) *Tracker {
	return &Tracker{
		startTime: time.Now(),
		verbose:   verbose,
	}
}

// SetTotal 设置总数
func (t *Tracker) SetTotal(files, size int64) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.totalFiles = files
	t.totalSize = size

	if t.verbose {
		fmt.Printf("开始备份: %d 个文件, 总计 %s\n", files, FormatSize(size))
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	}
}

// AddFile 添加已下载的文件
func (t *Tracker) AddFile(size int64) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.currentFiles++
	t.currentSize += size

	if t.verbose {
		t.printProgress()
	}
}

// printProgress 打印进度信息
func (t *Tracker) printProgress() {
	elapsed := time.Since(t.startTime)

	// 计算百分比
	var sizePercent float64
	if t.totalSize > 0 {
		sizePercent = float64(t.currentSize) / float64(t.totalSize) * 100
	}

	// 计算速度
	speed := float64(t.currentSize) / elapsed.Seconds()

	// 估算剩余时间
	var eta time.Duration
	if speed > 0 && t.totalSize > t.currentSize {
		eta = time.Duration(float64(t.totalSize-t.currentSize)/speed) * time.Second
	}

	// 生成进度条
	progressBar := t.generateProgressBar(sizePercent)

	fmt.Printf("\r[%s] %.1f%% | %d/%d 文件 | %s/%s | %s/s",
		progressBar,
		sizePercent,
		t.currentFiles,
		t.totalFiles,
		FormatSize(t.currentSize),
		FormatSize(t.totalSize),
		FormatSize(int64(speed)))

	if eta > 0 {
		fmt.Printf(" | ETA: %s", formatDuration(eta))
	}
}

// generateProgressBar 生成进度条
func (t *Tracker) generateProgressBar(percent float64) string {
	const width = 20
	filled := int(percent / 100 * width)

	bar := "["
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	bar += "]"

	return bar
}

// PrintFinal 打印最终统计信息
func (t *Tracker) PrintFinal() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	elapsed := time.Since(t.startTime)
	averageSpeed := float64(t.currentSize) / elapsed.Seconds()

	fmt.Printf("\n\n备份完成!\n")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("统计信息:\n")
	fmt.Printf("  文件数量: %d\n", t.currentFiles)
	fmt.Printf("  数据大小: %s\n", FormatSize(t.currentSize))
	fmt.Printf("  用时: %s\n", formatDuration(elapsed))
	fmt.Printf("  平均速度: %s/s\n", FormatSize(int64(averageSpeed)))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// FormatSize 格式化文件大小
func FormatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(size)/float64(div), units[exp])
}

// formatDuration 格式化时间间隔
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm%.0fs", d.Minutes(), float64(d.Seconds())-(d.Minutes()*60))
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) - hours*60
	return fmt.Sprintf("%dh%dm", hours, minutes)
}
