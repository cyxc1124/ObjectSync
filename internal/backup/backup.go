package backup

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"objectsync/internal/progress"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// Options 备份配置选项
type Options struct {
	Endpoint    string
	AccessKey   string
	SecretKey   string
	Bucket      string
	OutputDir   string
	Incremental bool
	StateFile   string
	Workers     int
	Verbose     bool
}

// State 备份状态
type State struct {
	LastBackup time.Time            `json:"last_backup"`
	Files      map[string]FileState `json:"files"`
}

// FileState 文件状态
type FileState struct {
	ETag         string    `json:"etag"`
	LastModified time.Time `json:"last_modified"`
	Size         int64     `json:"size"`
}

// Backup 备份器
type Backup struct {
	options  *Options
	s3       *s3.S3
	state    *State
	progress *progress.Tracker
}

// New 创建新的备份器
func New(options *Options) *Backup {
	return &Backup{
		options:  options,
		state:    &State{Files: make(map[string]FileState)},
		progress: progress.New(options.Verbose),
	}
}

// Run 执行备份
func (b *Backup) Run() error {
	// 初始化S3客户端
	if err := b.initS3Client(); err != nil {
		return fmt.Errorf("初始化S3客户端失败: %w", err)
	}

	// 加载备份状态
	if err := b.loadState(); err != nil {
		return fmt.Errorf("加载备份状态失败: %w", err)
	}

	// 创建输出目录
	if err := os.MkdirAll(b.options.OutputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 列出桶中的所有对象
	objects, err := b.listObjects()
	if err != nil {
		return fmt.Errorf("列出对象失败: %w", err)
	}

	if b.options.Verbose {
		fmt.Printf("发现 %d 个对象\n", len(objects))
	}

	// 过滤需要下载的对象
	toDownload := b.filterObjects(objects)
	if b.options.Verbose {
		fmt.Printf("需要下载 %d 个对象\n", len(toDownload))
	}

	if len(toDownload) == 0 {
		fmt.Println("没有需要下载的文件")
		return nil
	}

	// 计算总大小并设置进度跟踪
	var totalSize int64
	for _, obj := range toDownload {
		totalSize += *obj.Size
	}
	b.progress.SetTotal(int64(len(toDownload)), totalSize)

	// 下载对象
	if err := b.downloadObjects(toDownload); err != nil {
		return fmt.Errorf("下载对象失败: %w", err)
	}

	// 显示最终统计信息
	b.progress.PrintFinal()

	// 更新备份状态
	b.updateState(objects)

	// 保存状态
	if err := b.saveState(); err != nil {
		return fmt.Errorf("保存备份状态失败: %w", err)
	}

	return nil
}

// TestConnection 测试连接
func (b *Backup) TestConnection() error {
	// 初始化S3客户端
	if err := b.initS3Client(); err != nil {
		return err
	}

	// 尝试列出桶内容(仅获取第一页)
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(b.options.Bucket),
		MaxKeys: aws.Int64(1),
	}

	_, err := b.s3.ListObjectsV2(input)
	return err
}

// initS3Client 初始化S3客户端
func (b *Backup) initS3Client() error {
	sess, err := session.NewSession(&aws.Config{
		Endpoint:         aws.String(b.options.Endpoint),
		Credentials:      credentials.NewStaticCredentials(b.options.AccessKey, b.options.SecretKey, ""),
		Region:           aws.String("us-east-1"), // Ceph通常使用us-east-1
		S3ForcePathStyle: aws.Bool(true),          // Ceph需要路径样式
	})
	if err != nil {
		return err
	}

	b.s3 = s3.New(sess)
	return nil
}

// loadState 加载备份状态
func (b *Backup) loadState() error {
	if !b.options.Incremental {
		return nil
	}

	file, err := os.Open(b.options.StateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// 状态文件不存在，使用默认状态
			return nil
		}
		return err
	}
	defer file.Close()

	return json.NewDecoder(file).Decode(b.state)
}

// saveState 保存备份状态
func (b *Backup) saveState() error {
	if !b.options.Incremental {
		return nil
	}

	b.state.LastBackup = time.Now()

	file, err := os.Create(b.options.StateFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(b.state)
}

// listObjects 列出桶中的所有对象
func (b *Backup) listObjects() ([]*s3.Object, error) {
	var objects []*s3.Object

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(b.options.Bucket),
	}

	for {
		result, err := b.s3.ListObjectsV2(input)
		if err != nil {
			return nil, err
		}

		objects = append(objects, result.Contents...)

		if !*result.IsTruncated {
			break
		}

		input.ContinuationToken = result.NextContinuationToken
	}

	return objects, nil
}

// filterObjects 过滤需要下载的对象
func (b *Backup) filterObjects(objects []*s3.Object) []*s3.Object {
	var toDownload []*s3.Object

	for _, obj := range objects {
		key := *obj.Key

		// 跳过空文件名
		if key == "" {
			continue
		}

		// 如果不是增量备份，下载所有对象（包括目录标记）
		if !b.options.Incremental {
			toDownload = append(toDownload, obj)
			continue
		}

		// 对于目录标记（以/结尾且大小为0），检查本地目录是否存在
		if strings.HasSuffix(key, "/") && *obj.Size == 0 {
			localPath := filepath.Join(b.options.OutputDir, key)
			if _, err := os.Stat(localPath); os.IsNotExist(err) {
				// 目录不存在，需要创建
				toDownload = append(toDownload, obj)
			} else if b.options.Verbose {
				fmt.Printf("目录已存在: %s\n", key)
			}
			continue
		}

		etag := strings.Trim(*obj.ETag, "\"")

		// 检查文件是否需要下载
		if b.needsDownload(key, etag, *obj.LastModified, *obj.Size) {
			toDownload = append(toDownload, obj)
		}
	}

	return toDownload
}

// needsDownload 检查文件是否需要下载
func (b *Backup) needsDownload(key, etag string, lastModified time.Time, size int64) bool {
	// 检查本地路径是否存在
	localPath := filepath.Join(b.options.OutputDir, key)

	// 对于目录标记，检查目录是否存在
	if strings.HasSuffix(key, "/") && size == 0 {
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			return true // 目录不存在，需要创建
		}
	} else {
		// 对于文件，检查文件是否存在
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			return true // 文件不存在，需要下载
		}
	}

	// 检查状态记录
	state, exists := b.state.Files[key]
	if !exists {
		return true
	}

	// 比较ETag和修改时间
	if state.ETag != etag || !state.LastModified.Equal(lastModified) || state.Size != size {
		return true
	}

	return false
}

// downloadObjects 下载对象
func (b *Backup) downloadObjects(objects []*s3.Object) error {
	objectChan := make(chan *s3.Object, len(objects))
	errorChan := make(chan error, b.options.Workers)
	var wg sync.WaitGroup

	// 启动工作协程
	for i := 0; i < b.options.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for obj := range objectChan {
				if err := b.downloadObject(obj); err != nil {
					errorChan <- fmt.Errorf("下载 %s 失败: %w", *obj.Key, err)
					return
				}
			}
		}()
	}

	// 发送下载任务
	go func() {
		for _, obj := range objects {
			objectChan <- obj
		}
		close(objectChan)
	}()

	// 等待所有工作完成
	go func() {
		wg.Wait()
		close(errorChan)
	}()

	// 检查错误
	for err := range errorChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// downloadObject 下载单个对象
func (b *Backup) downloadObject(obj *s3.Object) error {
	key := *obj.Key
	localPath := filepath.Join(b.options.OutputDir, key)

	if b.options.Verbose {
		fmt.Printf("下载: %s -> %s\n", key, localPath)
	}

	// 如果是目录标记（以/结尾且大小为0），只创建目录
	if strings.HasSuffix(key, "/") && *obj.Size == 0 {
		if err := os.MkdirAll(localPath, 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}

		// 设置目录修改时间
		if err := os.Chtimes(localPath, *obj.LastModified, *obj.LastModified); err != nil {
			// 忽略时间设置错误，不是致命的
			if b.options.Verbose {
				fmt.Printf("警告: 设置目录时间失败 %s: %v\n", localPath, err)
			}
		}

		// 更新进度
		b.progress.AddFile(*obj.Size)
		return nil
	}

	// 创建父目录
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}

	// 下载对象
	input := &s3.GetObjectInput{
		Bucket: aws.String(b.options.Bucket),
		Key:    aws.String(key),
	}

	result, err := b.s3.GetObject(input)
	if err != nil {
		return err
	}
	defer result.Body.Close()

	// 写入本地文件
	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, result.Body)
	if err != nil {
		return err
	}

	// 设置文件修改时间
	if err := os.Chtimes(localPath, *obj.LastModified, *obj.LastModified); err != nil {
		// 忽略时间设置错误，不是致命的
		if b.options.Verbose {
			fmt.Printf("警告: 设置文件时间失败 %s: %v\n", localPath, err)
		}
	}

	// 更新进度
	b.progress.AddFile(*obj.Size)

	return nil
}

// updateState 更新备份状态
func (b *Backup) updateState(objects []*s3.Object) {
	if !b.options.Incremental {
		return
	}

	for _, obj := range objects {
		key := *obj.Key

		// 跳过空文件名
		if key == "" {
			continue
		}

		etag := strings.Trim(*obj.ETag, "\"")

		b.state.Files[key] = FileState{
			ETag:         etag,
			LastModified: *obj.LastModified,
			Size:         *obj.Size,
		}
	}
}
