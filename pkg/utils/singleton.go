package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// SingletonLock 单实例锁
type SingletonLock struct {
	file    *os.File
	lockDir string
}

// NewSingletonLock 创建单实例锁
func NewSingletonLock(name string) (*SingletonLock, error) {
	// 锁文件目录
	lockDir := filepath.Join(os.Getenv("HOME"), ".lingguard", "locks")
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}

	lockFile := filepath.Join(lockDir, name+".lock")

	// 打开或创建锁文件
	file, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	// 尝试获取排他锁
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("another instance is already running (lock file: %s)", lockFile)
	}

	// 写入当前 PID
	file.Truncate(0)
	file.Seek(0, 0)
	fmt.Fprintf(file, "%d\n", os.Getpid())

	return &SingletonLock{
		file:    file,
		lockDir: lockDir,
	}, nil
}

// Release 释放锁
func (s *SingletonLock) Release() error {
	if s.file != nil {
		syscall.Flock(int(s.file.Fd()), syscall.LOCK_UN)
		s.file.Close()
		s.file = nil
	}
	return nil
}
