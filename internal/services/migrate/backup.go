package migrate

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Allinost/go-backend-core/internal/services/crypto"
)

func CreateBackup(ctx context.Context, adapter DBAdapter, opts BackupOptions) (*BackupMeta, error) {
	tables := opts.Tables
	if len(tables) == 0 {
		var err error
		tables, err = adapter.GetTables(ctx)
		if err != nil {
			return nil, fmt.Errorf("migrate: 获取表列表失败: %w", err)
		}
	}

	dumpResult, err := Dump(ctx, adapter, DumpOptions{
		Tables:    tables,
		Format:    FormatJSON,
		BatchSize: 5000,
	})
	if err != nil {
		return nil, fmt.Errorf("migrate: 备份导出失败: %w", err)
	}

	data := dumpResult.Data

	if opts.CompressAlgo != "" {
		compressed, err := crypto.CompressAuto(data, opts.CompressAlgo, gzip.DefaultCompression, "")
		if err != nil {
			return nil, fmt.Errorf("migrate: 压缩失败: %w", err)
		}
		data = compressed
	}

	if opts.EncryptKey != "" {
		aes, err := crypto.NewAESGCMFromPassword(opts.EncryptKey, nil)
		if err != nil {
			return nil, fmt.Errorf("migrate: 加密初始化失败: %w", err)
		}
		encrypted, err := aes.Encrypt(data)
		if err != nil {
			return nil, fmt.Errorf("migrate: 加密失败: %w", err)
		}
		data = encrypted
	}

	checksum := fmt.Sprintf("%x", sha256.Sum256(data))
	now := time.Now()

	ext := "json"
	if opts.CompressAlgo != "" {
		ext = opts.CompressAlgo
	}
	filename := opts.Filename
	if filename == "" {
		filename = fmt.Sprintf("backup_%s_%s.%s", adapter.Type(), now.Format("20060102_150405"), ext)
	}

	meta := &BackupMeta{
		Version:      "1",
		CreatedAt:    now,
		DBType:       string(adapter.Type()),
		Tables:       tables,
		Format:       "json",
		CompressAlgo: opts.CompressAlgo,
		Encrypted:    opts.EncryptKey != "",
		RowCount:     dumpResult.RowCount,
		FileSize:     int64(len(data)),
		Checksum:     checksum,
	}

	outputPath := opts.OutputDir
	if outputPath == "" {
		outputPath = "."
	}

	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return nil, fmt.Errorf("migrate: 创建输出目录失败: %w", err)
	}

	metaBytes, _ := json.Marshal(meta)
	metaFile := filepath.Join(outputPath, filename+".meta.json")
	if err := os.WriteFile(metaFile, metaBytes, 0644); err != nil {
		return nil, fmt.Errorf("migrate: 写入元数据文件失败: %w", err)
	}

	dataFile := filepath.Join(outputPath, filename)
	if err := os.WriteFile(dataFile, data, 0644); err != nil {
		return nil, fmt.Errorf("migrate: 写入备份文件失败: %w", err)
	}

	return meta, nil
}

func RestoreBackup(ctx context.Context, adapter DBAdapter, backupPath string, password string) (*RestoreResult, error) {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return nil, fmt.Errorf("migrate: 读取备份文件失败: %w", err)
	}

	metaPath := backupPath + ".meta.json"
	metaData, err := os.ReadFile(metaPath)
	if err == nil {
		var meta BackupMeta
		if json.Unmarshal(metaData, &meta) == nil {
			if meta.Encrypted && password == "" {
				return nil, fmt.Errorf("migrate: 备份已加密，需要密码")
			}
		}
	}

	if password != "" {
		aes, err := crypto.NewAESGCMFromPassword(password, nil)
		if err != nil {
			return nil, fmt.Errorf("migrate: 解密初始化失败: %w", err)
		}
		decrypted, err := aes.Decrypt(data)
		if err != nil {
			return nil, fmt.Errorf("migrate: 解密失败: %w", err)
		}
		data = decrypted
	}

	if isCompressed(data) {
		decompressed, err := crypto.DecompressAuto(data, "")
		if err != nil {
			return nil, fmt.Errorf("migrate: 解压失败: %w", err)
		}
		data = decompressed
	}

	return Restore(ctx, adapter, data, RestoreOptions{
		Format:    FormatJSON,
		BatchSize: 1000,
	})
}

func ListBackups(dir string) ([]BackupMeta, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("migrate: 读取备份目录失败: %w", err)
	}

	var metas []BackupMeta
	for _, entry := range entries {
		if entry.IsDir() || !stringsHasSuffix(entry.Name(), ".meta.json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var meta BackupMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		metas = append(metas, meta)
	}
	return metas, nil
}

func DeleteBackup(dir, name string) error {
	os.Remove(filepath.Join(dir, name+".meta.json"))
	return os.Remove(filepath.Join(dir, name))
}

func isCompressed(data []byte) bool {
	if len(data) < 2 {
		return false
	}
	if data[0] == 0x1f && data[1] == 0x8b {
		return true
	}
	if data[0] == 0x78 {
		return true
	}
	return false
}

func stringsHasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func gzipCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gzipDecompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func zlibCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func zlibDecompress(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
