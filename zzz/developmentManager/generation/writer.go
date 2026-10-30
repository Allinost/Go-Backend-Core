package generation

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WriteFiles 把生成文件写入 root 目录，返回实际写入的相对路径（slash 形式）
func WriteFiles(root string, files []File) ([]string, error) {
	written := make([]string, 0, len(files))
	for _, f := range files {
		rel, err := safeRelPath(f.RelPath)
		if err != nil {
			return written, err
		}
		target := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return written, fmt.Errorf("创建目录失败: %w", err)
		}
		if err := os.WriteFile(target, []byte(f.Content), 0o644); err != nil {
			return written, fmt.Errorf("写入文件失败: %w", err)
		}
		written = append(written, rel)
	}
	return written, nil
}

// safeRelPath 校验相对路径，拒绝绝对路径与目录穿越
func safeRelPath(rel string) (string, error) {
	normalized := filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel)))
	if normalized == "." || normalized == "" {
		return "", fmt.Errorf("非法产物路径: %q", rel)
	}
	if strings.HasPrefix(normalized, "/") || filepath.IsAbs(filepath.FromSlash(rel)) {
		return "", fmt.Errorf("产物路径必须为相对路径: %q", rel)
	}
	if normalized == ".." || strings.HasPrefix(normalized, "../") {
		return "", fmt.Errorf("产物路径不得越出仓库根目录: %q", rel)
	}
	return normalized, nil
}

// CommitFiles 在 root 仓库中仅提交指定相对路径，返回所在分支与 commit hash。
// 安全约束：当前分支为 main/master 时拒绝自动提交。
func CommitFiles(root, message string, relPaths []string) (branch, hash string, err error) {
	if len(relPaths) == 0 {
		return "", "", fmt.Errorf("没有可提交的产物")
	}
	if _, statErr := os.Stat(filepath.Join(root, ".git")); statErr != nil {
		return "", "", fmt.Errorf("当前目录不是 git 仓库: %s", root)
	}

	branchOut, err := runGit(root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", "", fmt.Errorf("读取当前分支失败: %w", err)
	}
	branch = strings.TrimSpace(branchOut)
	if branch == "main" || branch == "master" || branch == "HEAD" {
		return "", "", fmt.Errorf("当前分支 %s 不允许自动提交，请先切换到特性分支", branch)
	}

	addArgs := append([]string{"add", "--"}, relPaths...)
	if _, err := runGit(root, addArgs...); err != nil {
		return "", "", fmt.Errorf("git add 失败: %w", err)
	}

	commitArgs := append([]string{"commit", "-m", message, "--"}, relPaths...)
	if _, err := runGit(root, commitArgs...); err != nil {
		return "", "", fmt.Errorf("git commit 失败: %w", err)
	}

	hashOut, err := runGit(root, "rev-parse", "HEAD")
	if err != nil {
		return branch, "", fmt.Errorf("读取 commit hash 失败: %w", err)
	}
	return branch, strings.TrimSpace(hashOut), nil
}

// runGit 执行 git 命令并返回标准输出（错误时附带 stderr）
func runGit(root string, args ...string) (string, error) {
	full := append([]string{"-C", root}, args...)
	cmd := exec.Command("git", full...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
