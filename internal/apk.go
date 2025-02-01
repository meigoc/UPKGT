// internal/apk.go
package internal

import (
    "archive/tar"
    "bytes"
    "compress/gzip"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"
)

// APK структура для Alpine Linux пакетов
type APK struct {
    Path       string
    Name       string
    Version    string
    BuildDate  time.Time
    Info       *PackageInfo
}

// APKMetadata структура метаданных .apk пакета
type APKMetadata struct {
    Package     string   `json:"package"`
    Version     string   `json:"version"`
    Arch        string   `json:"architecture"`
    Maintainer  string   `json:"maintainer"`
    Description string   `json:"description"`
    URL         string   `json:"url"`
    Size        int64    `json:"size"`
    Depends     []string `json:"depends"`
    Provides    []string `json:"provides"`
    InstallIf   []string `json:"install_if"`
}

// NewAPK создает новый экземпляр APK
func NewAPK(path string) (*APK, error) {
    absPath, err := filepath.Abs(path)
    if err != nil {
        return nil, fmt.Errorf("failed to get absolute path: %w", err)
    }

    apk := &APK{
        Path:      absPath,
        BuildDate: time.Now().UTC(),
    }

    if err := apk.validate(); err != nil {
        return nil, err
    }

    return apk, nil
}

// validate проверяет корректность .apk файла
func (a *APK) validate() error {
    if !strings.HasSuffix(a.Path, ".apk") {
        return fmt.Errorf("invalid package format: must be .apk")
    }

    fi, err := os.Stat(a.Path)
    if err != nil {
        return fmt.Errorf("failed to stat package file: %w", err)
    }

    if fi.Size() == 0 {
        return fmt.Errorf("invalid package: file is empty")
    }

    return nil
}

// Install устанавливает .apk пакет
func (a *APK) Install(force bool) error {
    if err := RequireRoot(); err != nil {
        return err
    }

    logger.Infof("Installing APK package: %s", a.Path)

    // Создаем резервную копию
    backupPath, err := CreateBackup("/etc/apk/world")
    if err != nil {
        logger.Warnf("Failed to create backup: %v", err)
    } else {
        logger.Infof("Created backup: %s", backupPath)
    }

    // Подготавливаем команду установки
    args := []string{"add"}
    if force {
        args = append(args, "--force-overwrite")
    }
    args = append(args, a.Path)

    // Выполняем установку
    cmd := exec.Command("apk", args...)
    cmd.Env = append(os.Environ(), "LANG=C")
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("installation failed: %s: %w", string(output), err)
    }

    logger.Info("Package installed successfully")
    return nil
}

// Remove удаляет установленный пакет
func (a *APK) Remove(purge bool) error {
    if err := RequireRoot(); err != nil {
        return err
    }

    if a.Name == "" {
        info, err := a.GetInfo()
        if err != nil {
            return fmt.Errorf("failed to get package info: %w", err)
        }
        a.Name = info.Name
    }

    logger.Infof("Removing APK package: %s", a.Name)

    // Создаем резервную копию
    backupPath, err := CreateBackup("/etc/apk/world")
    if err != nil {
        logger.Warnf("Failed to create backup: %v", err)
    } else {
        logger.Infof("Created backup: %s", backupPath)
    }

    // Подготавливаем команду удаления
    args := []string{"del"}
    if purge {
        args = append(args, "--purge")
    }
    args = append(args, a.Name)

    // Выполняем удаление
    cmd := exec.Command("apk", args...)
    cmd.Env = append(os.Environ(), "LANG=C")
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("removal failed: %s: %w", string(output), err)
    }

    logger.Info("Package removed successfully")
    return nil
}

// GetInfo возвращает информацию о пакете
func (a *APK) GetInfo() (*PackageInfo, error) {
    if a.Info != nil {
        return a.Info, nil
    }

    // Открываем .apk файл
    f, err := os.Open(a.Path)
    if err != nil {
        return nil, fmt.Errorf("failed to open package: %w", err)
    }
    defer f.Close()

    // Читаем архив
    gzr, err := gzip.NewReader(f)
    if err != nil {
        return nil, fmt.Errorf("failed to create gzip reader: %w", err)
    }
    defer gzr.Close()

    tr := tar.NewReader(gzr)

    var metadata APKMetadata
    var control []byte

    // Ищем .PKGINFO файл
    for {
        header, err := tr.Next()
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, fmt.Errorf("failed to read tar header: %w", err)
        }

        if header.Name == ".PKGINFO" {
            buf := new(bytes.Buffer)
            if _, err := io.Copy(buf, tr); err != nil {
                return nil, fmt.Errorf("failed to read .PKGINFO: %w", err)
            }
            control = buf.Bytes()
            break
        }
    }

    if control == nil {
        return nil, fmt.Errorf("package metadata not found")
    }

    // Парсим метаданные
    if err := parseAPKMetadata(control, &metadata); err != nil {
        return nil, fmt.Errorf("failed to parse metadata: %w", err)
    }

    // Создаем информацию о пакете
    info := &PackageInfo{
        Name:         metadata.Package,
        Version:      metadata.Version,
        Architecture: metadata.Arch,
        Description:  metadata.Description,
        Maintainer:   metadata.Maintainer,
        Homepage:     metadata.URL,
        Size:         metadata.Size,
        Dependencies: metadata.Depends,
        Provides:     metadata.Provides,
        InstallDate:  a.BuildDate,
    }

    // Получаем размер файла
    if fi, err := os.Stat(a.Path); err == nil {
        info.Size = fi.Size()
    }

    a.Info = info
    return info, nil
}

// parseAPKMetadata парсит метаданные .apk пакета
func parseAPKMetadata(data []byte, metadata *APKMetadata) error {
    lines := strings.Split(string(data), "\n")
    current := make(map[string]string)

    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }

        parts := strings.SplitN(line, "=", 2)
        if len(parts) != 2 {
            continue
        }

        key := strings.TrimSpace(parts[0])
        value := strings.TrimSpace(parts[1])

        switch key {
        case "pkgname":
            metadata.Package = value
        case "pkgver":
            metadata.Version = value
        case "arch":
            metadata.Architecture = value
        case "maintainer":
            metadata.Maintainer = value
        case "pkgdesc":
            metadata.Description = value
        case "url":
            metadata.URL = value
        case "size":
            if size, err := parseInt64(value); err == nil {
                metadata.Size = size
            }
        case "depend":
            metadata.Depends = append(metadata.Depends, value)
        case "provides":
            metadata.Provides = append(metadata.Provides, value)
        case "install_if":
            metadata.InstallIf = append(metadata.InstallIf, value)
        }
    }

    return nil
}

// parseInt64 конвертирует строку в int64
func parseInt64(s string) (int64, error) {
    var result int64
    _, err := fmt.Sscanf(s, "%d", &result)
    return result, err
}

// GetType возвращает тип пакета
func (a *APK) GetType() PackageType {
    return TypeAPK
}

// String возвращает строковое представление пакета
func (a *APK) String() string {
    if a.Info != nil {
        return fmt.Sprintf("%s-%s.apk", a.Info.Name, a.Info.Version)
    }
    return filepath.Base(a.Path)
}