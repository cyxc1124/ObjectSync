package upload

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"objectsync/internal/progress"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// Options 上传配置选项
type Options struct {
	Endpoint    string
	AccessKey   string
	SecretKey   string
	Bucket      string
	InputDir    string
	Incremental bool
	StateFile   string
	Workers     int
	Verbose     bool
}

// State 上传状态
type State struct {
	LastUpload time.Time            `json:"last_upload"`
	Files      map[string]FileState `json:"files"`
}

// FileState 文件状态
type FileState struct {
	ETag         string    `json:"etag"`
	LastModified time.Time `json:"last_modified"`
	Size         int64     `json:"size"`
}

// Upload 上传器
type Upload struct {
	options  *Options
	s3       *s3.S3
	state    *State
	progress *progress.Tracker
}

// New 创建新的上传器
func New(options *Options) *Upload {
	return &Upload{
		options:  options,
		state:    &State{Files: make(map[string]FileState)},
		progress: progress.New(options.Verbose),
	}
}

// Run 执行上传
func (u *Upload) Run() error {
	// 初始化S3客户端
	if err := u.initS3Client(); err != nil {
		return fmt.Errorf("初始化S3客户端失败: %w", err)
	}

	// 确保存储桶存在
	if err := u.ensureBucketExists(); err != nil {
		return fmt.Errorf("确保存储桶存在失败: %w", err)
	}

	// 加载上传状态
	if err := u.loadState(); err != nil {
		return fmt.Errorf("加载上传状态失败: %w", err)
	}

	// 检查输入目录
	if _, err := os.Stat(u.options.InputDir); os.IsNotExist(err) {
		return fmt.Errorf("输入目录不存在: %s", u.options.InputDir)
	}

	// 扫描本地文件
	files, err := u.scanLocalFiles()
	if err != nil {
		return fmt.Errorf("扫描本地文件失败: %w", err)
	}

	if u.options.Verbose {
		fmt.Printf("发现 %d 个文件\n", len(files))
	}

	// 过滤需要上传的文件
	toUpload := u.filterFiles(files)
	if u.options.Verbose {
		fmt.Printf("需要上传 %d 个文件\n", len(toUpload))
	}

	if len(toUpload) == 0 {
		fmt.Println("没有需要上传的文件")
		return nil
	}

	// 计算总大小并设置进度跟踪
	var totalSize int64
	for _, file := range toUpload {
		totalSize += file.Size
	}
	u.progress.SetTotal(int64(len(toUpload)), totalSize)

	// 上传文件
	if err := u.uploadFiles(toUpload); err != nil {
		return fmt.Errorf("上传文件失败: %w", err)
	}

	// 显示最终统计信息
	u.progress.PrintFinal()

	// 更新上传状态
	u.updateState(toUpload)

	// 保存状态
	if err := u.saveState(); err != nil {
		return fmt.Errorf("保存上传状态失败: %w", err)
	}

	return nil
}

// LocalFile 本地文件信息
type LocalFile struct {
	Path         string
	Key          string
	Size         int64
	LastModified time.Time
	IsDir        bool
}

// TestConnection 测试连接
func (u *Upload) TestConnection() error {
	// 初始化S3客户端
	if err := u.initS3Client(); err != nil {
		return err
	}

	// 尝试列出桶
	_, err := u.s3.ListBuckets(&s3.ListBucketsInput{})
	return err
}

// initS3Client 初始化S3客户端
func (u *Upload) initS3Client() error {
	sess, err := session.NewSession(&aws.Config{
		Endpoint:         aws.String(u.options.Endpoint),
		Credentials:      credentials.NewStaticCredentials(u.options.AccessKey, u.options.SecretKey, ""),
		Region:           aws.String("us-east-1"), // Ceph通常使用us-east-1
		S3ForcePathStyle: aws.Bool(true),          // Ceph需要路径样式
	})
	if err != nil {
		return err
	}

	u.s3 = s3.New(sess)
	return nil
}

// ensureBucketExists 确保存储桶存在
func (u *Upload) ensureBucketExists() error {
	// 检查桶是否存在
	_, err := u.s3.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(u.options.Bucket),
	})

	if err != nil {
		// 如果是404错误，说明桶不存在，需要创建
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "NotFound" {
			if u.options.Verbose {
				fmt.Printf("存储桶 %s 不存在，正在创建...\n", u.options.Bucket)
			}

			// 创建桶
			_, err = u.s3.CreateBucket(&s3.CreateBucketInput{
				Bucket: aws.String(u.options.Bucket),
			})
			if err != nil {
				return fmt.Errorf("创建存储桶失败: %w", err)
			}

			if u.options.Verbose {
				fmt.Printf("存储桶 %s 创建成功\n", u.options.Bucket)
			}
		} else {
			return fmt.Errorf("检查存储桶失败: %w", err)
		}
	}

	return nil
}

// loadState 加载上传状态
func (u *Upload) loadState() error {
	if !u.options.Incremental {
		return nil
	}

	file, err := os.Open(u.options.StateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// 状态文件不存在，使用默认状态
			return nil
		}
		return err
	}
	defer file.Close()

	return json.NewDecoder(file).Decode(u.state)
}

// saveState 保存上传状态
func (u *Upload) saveState() error {
	if !u.options.Incremental {
		return nil
	}

	u.state.LastUpload = time.Now()

	file, err := os.Create(u.options.StateFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(u.state)
}

// scanLocalFiles 扫描本地文件
func (u *Upload) scanLocalFiles() ([]*LocalFile, error) {
	var files []*LocalFile

	err := filepath.Walk(u.options.InputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 计算相对路径作为对象键
		relPath, err := filepath.Rel(u.options.InputDir, path)
		if err != nil {
			return err
		}

		// 跳过根目录
		if relPath == "." {
			return nil
		}

		// 将路径分隔符转换为正斜杠（对象存储标准）
		key := strings.ReplaceAll(relPath, "\\", "/")

		file := &LocalFile{
			Path:         path,
			Key:          key,
			Size:         info.Size(),
			LastModified: info.ModTime(),
			IsDir:        info.IsDir(),
		}

		// 如果是目录，添加目录标记（以/结尾）
		if info.IsDir() {
			file.Key += "/"
			file.Size = 0
		}

		files = append(files, file)
		return nil
	})

	return files, err
}

// filterFiles 过滤需要上传的文件
func (u *Upload) filterFiles(files []*LocalFile) []*LocalFile {
	var toUpload []*LocalFile

	for _, file := range files {
		// 如果不是增量上传，上传所有文件
		if !u.options.Incremental {
			toUpload = append(toUpload, file)
			continue
		}

		// 检查文件是否需要上传
		if u.needsUpload(file) {
			toUpload = append(toUpload, file)
		}
	}

	return toUpload
}

// needsUpload 检查文件是否需要上传
func (u *Upload) needsUpload(file *LocalFile) bool {
	// 检查状态记录
	state, exists := u.state.Files[file.Key]
	if !exists {
		return true
	}

	// 比较修改时间和大小
	if !state.LastModified.Equal(file.LastModified) || state.Size != file.Size {
		return true
	}

	return false
}

// uploadFiles 上传文件
func (u *Upload) uploadFiles(files []*LocalFile) error {
	fileChan := make(chan *LocalFile, len(files))
	errorChan := make(chan error, u.options.Workers)
	var wg sync.WaitGroup

	// 启动工作协程
	for i := 0; i < u.options.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range fileChan {
				if err := u.uploadFile(file); err != nil {
					errorChan <- fmt.Errorf("上传 %s 失败: %w", file.Key, err)
					return
				}
			}
		}()
	}

	// 发送上传任务
	go func() {
		for _, file := range files {
			fileChan <- file
		}
		close(fileChan)
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

// uploadFile 上传单个文件
func (u *Upload) uploadFile(file *LocalFile) error {
	if u.options.Verbose {
		fmt.Printf("上传: %s -> %s\n", file.Path, file.Key)
	}

	// 如果是目录标记，只需要创建一个空对象
	if file.IsDir {
		input := &s3.PutObjectInput{
			Bucket: aws.String(u.options.Bucket),
			Key:    aws.String(file.Key),
			Body:   strings.NewReader(""),
		}

		_, err := u.s3.PutObject(input)
		if err != nil {
			return fmt.Errorf("创建目录标记失败: %w", err)
		}

		// 更新进度
		u.progress.AddFile(0)
		return nil
	}

	// 打开本地文件
	localFile, err := os.Open(file.Path)
	if err != nil {
		return err
	}
	defer localFile.Close()

	// 上传文件
	input := &s3.PutObjectInput{
		Bucket: aws.String(u.options.Bucket),
		Key:    aws.String(file.Key),
		Body:   localFile,
	}

	_, err = u.s3.PutObject(input)
	if err != nil {
		return err
	}

	// 更新进度
	u.progress.AddFile(file.Size)

	return nil
}

// updateState 更新上传状态
func (u *Upload) updateState(files []*LocalFile) {
	if !u.options.Incremental {
		return
	}

	for _, file := range files {
		u.state.Files[file.Key] = FileState{
			ETag:         "", // 上传后可以从响应中获取ETag，这里简化处理
			LastModified: file.LastModified,
			Size:         file.Size,
		}
	}
}
