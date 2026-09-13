// Package storage 提供极简本地文件存储能力。
//
// 现有代码库没有通用上传服务，工单附件是本轮第一个需要落盘的功能；
// 这里只实现「按目录保存 + 读取 + 删除」三件事，避免为单点需求引入对象存储依赖。
// 后续内容中心等模块可复用同一接口（需要时再抽出 Store 接口做多实现）。
package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrPathEscape 相对路径越出存储根目录（防目录穿越）。
var ErrPathEscape = errors.New("非法文件路径")

// LocalStore 以本地目录为根的存储实现。
type LocalStore struct {
	root string
}

// NewLocalStore 创建本地存储；root 不存在时自动创建。
func NewLocalStore(root string) (*LocalStore, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve storage root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("create storage root: %w", err)
	}
	return &LocalStore{root: abs}, nil
}

// Root 返回存储根目录的绝对路径。
func (s *LocalStore) Root() string { return s.root }

// Save 保存文件到 root/dir 下，返回相对 root 的路径与写入字节数。
//
// 落盘文件名统一加随机前缀：同名文件互不覆盖，且不暴露原始名（原始名保存在 DB 元数据里，
// 下载时用 Content-Disposition 还原）。maxSize <= 0 表示不限制。
func (s *LocalStore) Save(dir, filename string, r io.Reader, maxSize int64) (string, int64, error) {
	cleanDir, err := s.safeRel(dir)
	if err != nil {
		return "", 0, err
	}
	absDir := filepath.Join(s.root, cleanDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return "", 0, fmt.Errorf("create dir: %w", err)
	}

	suffix := filepath.Ext(filename)
	if len(suffix) > 16 {
		// 超长扩展名视为异常输入，直接丢弃，避免拼出超长文件名。
		suffix = ""
	}
	name := randomHex(8) + suffix
	target := filepath.Join(absDir, name)

	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", 0, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	var written int64
	if maxSize > 0 {
		// 多读 1 字节用于判定超限，避免静默截断。
		written, err = io.Copy(f, io.LimitReader(r, maxSize+1))
		if err == nil && written > maxSize {
			_ = os.Remove(target)
			return "", 0, fmt.Errorf("文件超过大小上限（%d 字节）", maxSize)
		}
	} else {
		written, err = io.Copy(f, r)
	}
	if err != nil {
		_ = os.Remove(target)
		return "", 0, fmt.Errorf("write file: %w", err)
	}
	return filepath.ToSlash(filepath.Join(cleanDir, name)), written, nil
}

// Open 打开相对路径对应的文件（调用方负责 Close）。
func (s *LocalStore) Open(relPath string) (*os.File, error) {
	clean, err := s.safeRel(relPath)
	if err != nil {
		return nil, err
	}
	return os.Open(filepath.Join(s.root, clean))
}

// Remove 删除相对路径对应的文件；文件不存在不报错。
func (s *LocalStore) Remove(relPath string) error {
	clean, err := s.safeRel(relPath)
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(s.root, clean)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// safeRel 归一化并校验相对路径，禁止向上越出根目录。
func (s *LocalStore) safeRel(rel string) (string, error) {
	rel = strings.TrimSpace(strings.ReplaceAll(rel, "\\", "/"))
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return "", ErrPathEscape
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrPathEscape
	}
	return clean, nil
}

// randomHex 生成 n 字节随机数的十六进制串。
func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand 失败极罕见；此处不能吞错返回可预测名字，退化为时间戳由上层重试。
		return "fallback"
	}
	return hex.EncodeToString(buf)
}
