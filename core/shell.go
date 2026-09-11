package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

var resourceDir string
var workspaceDir string
var localDataDir string
var appRootDir string
var dbgToolDir string
var startUpDir string
var tmpDir string
var resourceDirOnce, localDataDirOnce, workspaceDirOnce, tmpDirOnce sync.Once

func CopyFile(dst, src string) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	if err = os.Link(src, dst); err == nil {
		return
	}
	err = copyFileContents(dst, src)
	return
}

func copyFileContents(dst, src string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

// 静态数据目录
// 用来存放图标, 语言包等和程序一起发布的静态数据
// SILK_RESOURCE_DIR 可指定资源目录；默认支持 macOS Contents/Resources、
// 可执行文件旁资源，以及开发时的当前目录和 GOPATH/bin。只读，不存放用户数据。
func ResourceDir() string {
	resourceDirOnce.Do(func() {
		cwd, _ := os.Getwd()
		resourceDir = resolveResourceDir(AppInstallDir(), cwd, os.Getenv("GOPATH"), os.Getenv("SILK_RESOURCE_DIR"))
	})
	return resourceDir
}

// 本机用户设置目录
// 用来存放和工区无关用户设置, 例如用户习惯, 窗口位置等
// 默认位于 os.UserConfigDir()/silk/<程序名>，不再修改安装目录。
// 便携部署可在启动前设置 SILK_DATA_DIR 指向原有 local 目录。
// 目录创建错误会报告到 stderr；调用方仍须处理具体文件读写错误。
func LocalDataDir() string {
	localDataDirOnce.Do(func() {
		var err error
		localDataDir = os.Getenv("SILK_DATA_DIR")
		if localDataDir == "" {
			var base string
			base, err = os.UserConfigDir()
			if err == nil {
				localDataDir = filepath.Join(base, "silk", AppShortName())
			} else {
				// No configured home: use a private process directory, never cwd
				// or a shared predictable directory in /tmp.
				fmt.Fprintln(os.Stderr, "silk: user configuration unavailable; using temporary data:", err)
				localDataDir = TempDir()
			}
		}
		localDataDir, err = preparePrivateDir(localDataDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "silk: local data directory:", err)
		}
	})
	return localDataDir
}

// 程序安装目录
// 在目前的版本里, 此目录等于程序所在目录
// 但在编写代码时, 请不要用程序所在目录代替此目录, 因为以后会改变
func AppInstallDir() string {
	return ExeFileDir()
}

// 本机工作区目录
// 默认位于 LocalDataDir()/workspace；SILK_WORKSPACE_DIR 可保留便携部署路径。
func WorkspaceDir() string {
	workspaceDirOnce.Do(func() {
		workspaceDir = os.Getenv("SILK_WORKSPACE_DIR")
		if workspaceDir == "" {
			workspaceDir = filepath.Join(LocalDataDir(), "workspace")
		}
		var err error
		workspaceDir, err = preparePrivateDir(workspaceDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "silk: workspace directory:", err)
		}
	})
	return workspaceDir
}

// 调试工具所在目录
// 在目前的版本里, 此目录等于ExeFileDir()+"/dbgtool"
func DbgToolDir() string {
	if dbgToolDir == "" {
		dbgToolDir = ExeFileDir() + "/dbgtool"
	}
	return dbgToolDir
}

// 软件使用的临时文件目录
// 每个进程使用独立的 0700 目录，避免多个用户或应用实例相互覆盖。
func TempDir() string {
	tmpDirOnce.Do(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "silk-"+AppShortName()+"-")
		if err != nil {
			// This API cannot return an error. Do not return "" and let callers
			// accidentally write temporary data into their working directory.
			panic(fmt.Errorf("silk: create temporary directory: %w", err))
		}
	})
	return tmpDir
}
