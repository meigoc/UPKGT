// internal/utils.go
package internal

import (
    "archive/tar"
    "compress/gzip"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "syscall"
    "time"

    "github.com/sirupsen/logrus"
    "github.com/ulikunitz/xz"
)

var logger = logrus.New()

func init() {
    logger.SetFormatter(&logrus.TextFormatter{
        FullTimestamp:   true,
        TimestampFormat: "2006-01-02 15:04:05",
    })
    logger.SetOutput(os.Stdout)
}

// FileInfo содержит информацию о файле
type FileInfo struct {
    Path        string
    Size        int64
    Mode        os.FileMode
    ModTime     time.Time
    Hash        string
    IsDir       bool
}

// CheckRoot проверяет root права
func CheckRoot() bool {
    return os.Geteuid() == 0
}

// RequireRoot проверяет root права и возвращает ошибку если их нет
func RequireRoot() error {
    if !CheckRoot() {
        return fmt.Errorf("root privileges required")
    }
    return nil
}

// CreateDirectory создает директорию с нужными правами
func CreateDirectory(path string, mode os.FileMode) error {
    if err := os.MkdirAll(path, mode); err != nil {
        return fmt.Errorf("failed to create directory %s: %w", path, err)
    }
    return nil
}

// RemoveDirectory удаляет директорию рекурсивно
func RemoveDirectory(path string) error {
    if err := os.RemoveAll(path); err != nil {
        return fmt.Errorf("failed to remove directory %s: %w", path, err)
    }
    return nil
}

// CopyFile копирует файл с сохранением прав
func CopyFile(src, dst string) error {
    sourceFileStat, err := os.Stat(src)
    if err != nil {
        return fmt.Errorf("failed to stat source file: %w", err)
    }

    if !sourceFileStat.Mode().IsRegular() {
        return fmt.Errorf("%s is not a regular file", src)
    }

    source, err := os.Open(src)
    if err != nil {
        return fmt.Errorf("failed to open source file: %w", err)
    }
    defer source.Close()

    destination, err := os.CreateTemp(filepath.Dir(dst), ".tmp")
    if err != nil {
        return fmt.Errorf("failed to create temporary file: %w", err)
    }
    tempPath := destination.Name()
    defer os.Remove(tempPath)

    if _, err = io.Copy(destination, source); err != nil {
        destination.Close()
        return fmt.Errorf("failed to copy file contents: %w", err)
    }

    if err = destination.Close(); err != nil {
        return fmt.Errorf("failed to close destination file: %w", err)
    }

    if err = os.Chmod(tempPath, sourceFileStat.Mode()); err != nil {
        return fmt.Errorf("failed to set file permissions: %w", err)
    }

    if err = os.Rename(tempPath, dst); err != nil {
        return fmt.Errorf("failed to move file to destination: %w", err)
    }

    return nil
}

// CalculateFileHash вычисляет SHA256 хеш файла
func CalculateFileHash(path string) (string, error) {
    file, err := os.Open(path)
    if err != nil {
        return "", fmt.Errorf("failed to open file: %w", err)
    }
    defer file.Close()

    hash := sha256.New()
    if _, err := io.Copy(hash, file); err != nil {
        return "", fmt.Errorf("failed to calculate hash: %w", err)
    }

    return hex.EncodeToString(hash.Sum(nil)), nil
}

// ExtractTarGz распаковывает tar.gz архив
func ExtractTarGz(src, dst string) error {
    file, err := os.Open(src)
    if err != nil {
        return fmt.Errorf("failed to open archive: %w", err)
    }
    defer file.Close()

    gzr, err := gzip.NewReader(file)
    if err != nil {
        return fmt.Errorf("failed to create gzip reader: %w", err)
    }
    defer gzr.Close()

    tr := tar.NewReader(gzr)

    for {
        header, err := tr.Next()
        if err == io.EOF {
            break
        }
        if err != nil {
            return fmt.Errorf("failed to read tar header: %w", err)
        }

        target := filepath.Join(dst, header.Name)
        
        switch header.Typeflag {
        case tar.TypeDir:
            if err := CreateDirectory(target, os.FileMode(header.Mode)); err != nil {
                return err
            }
        case tar.TypeReg:
            dir := filepath.Dir(target)
            if err := CreateDirectory(dir, 0755); err != nil {
                return err
            }

            f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
            if err != nil {
                return fmt.Errorf("failed to create file: %w", err)
            }

            if _, err := io.Copy(f, tr); err != nil {
                f.Close()
                return fmt.Errorf("failed to write file contents: %w", err)
            }
            f.Close()
        }
    }

    return nil
}

// ExtractTarXz распаковывает tar.xz архив
func ExtractTarXz(src, dst string) error {
    file, err := os.Open(src)
    if err != nil {
        return fmt.Errorf("failed to open archive: %w", err)
    }
    defer file.Close()

    xzr, err := xz.NewReader(file)
    if err != nil {
        return fmt.Errorf("failed to create xz reader: %w", err)
    }

    tr := tar.NewReader(xzr)

    for {
        header, err := tr.Next()
        if err == io.EOF {
            break
        }
        if err != nil {
            return fmt.Errorf("failed to read tar header: %w", err)
        }

        target := filepath.Join(dst, header.Name)

        switch header.Typeflag {
        case tar.TypeDir:
            if err := CreateDirectory(target, os.FileMode(header.Mode)); err != nil {
                return err
            }
        case tar.TypeReg:
            dir := filepath.Dir(target)
            if err := CreateDirectory(dir, 0755); err != nil {
                return err
            }

            f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
            if err != nil {
                return fmt.Errorf("failed to create file: %w", err)
            }

            if _, err := io.Copy(f, tr); err != nil {
                f.Close()
                return fmt.Errorf("failed to write file contents: %w", err)
            }
            f.Close()
        }
    }

    return nil
}

// CreateBackup создает резервную копию файла или директории
func CreateBackup(path string) (string, error) {
    backupDir := "/var/backups/upkgt"
    if err := CreateDirectory(backupDir, 0755); err != nil {
        return "", err
    }

    timestamp := time.Now().Format("20060102-150405")
    backupName := fmt.Sprintf("%s-%s.tar.gz", filepath.Base(path), timestamp)
    backupPath := filepath.Join(backupDir, backupName)

    file, err := os.Create(backupPath)
    if err != nil {
        return "", fmt.Errorf("failed to create backup file: %w", err)
    }
    defer file.Close()

    gzw := gzip.NewWriter(file)
    defer gzw.Close()

    tw := tar.NewWriter(gzw)
    defer tw.Close()

    err = filepath.Walk(path, func(file string, fi os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        header, err := tar.FileInfoHeader(fi, file)
        if err != nil {
            return fmt.Errorf("failed to create tar header: %w", err)
        }

        relPath, err := filepath.Rel(path, file)
        if err != nil {
            return fmt.Errorf("failed to get relative path: %w", err)
        }
        header.Name = relPath

        if err := tw.WriteHeader(header); err != nil {
            return fmt.Errorf("failed to write tar header: %w", err)
        }

        if !fi.Mode().IsRegular() {
            return nil
        }

        f, err := os.Open(file)
        if err != nil {
            return fmt.Errorf("failed to open file: %w", err)
        }
        defer f.Close()

        if _, err := io.Copy(tw, f); err != nil {
            return fmt.Errorf("failed to write file contents: %w", err)
        }

        return nil
    })

    if err != nil {
        return "", fmt.Errorf("failed to create backup: %w", err)
    }

    return backupPath, nil
}

// ExecuteCommand выполняет команду и возвращает вывод
func ExecuteCommand(name string, args ...string) (string, error) {
    cmd := exec.Command(name, args...)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return "", fmt.Errorf("command failed: %s: %w", string(output), err)
    }
    return string(output), nil
}

// IsSymlink проверяет является ли путь символической ссылкой
func IsSymlink(path string) bool {
    fi, err := os.Lstat(path)
    if err != nil {
        return false
    }
    return fi.Mode()&os.ModeSymlink != 0
}

// GetFileOwnership возвращает владельца и группу файла
func GetFileOwnership(path string) (uid, gid int, err error) {
    fi, err := os.Stat(path)
    if err != nil {
        return 0, 0, err
    }
    stat := fi.Sys().(*syscall.Stat_t)
    return int(stat.Uid), int(stat.Gid), nil
}

// ValidatePath проверяет путь на безопасность
func ValidatePath(path string) error {
    if strings.Contains(path, "..") {
        return fmt.Errorf("path contains forbidden sequences")
    }
    
    absPath, err := filepath.Abs(path)
    if err != nil {
        return fmt.Errorf("failed to get absolute path: %w", err)
    }
    
    if !strings.HasPrefix(absPath, "/") {
        return fmt.Errorf("path must be absolute")
    }
    
    return nil
}

// CleanPath очищает и нормализует путь
func CleanPath(path string) string {
    return filepath.Clean(path)
}