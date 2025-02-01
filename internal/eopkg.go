// internal/eopkg.go
package internal

import (
    "archive/tar"
    "bytes"
    "compress/gzip"
    "encoding/xml"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"
)

// Eopkg структура для Solus пакетов
type Eopkg struct {
    Path       string
    Name       string
    Version    string
    BuildDate  time.Time
    Info       *PackageInfo
}

// EopkgMetadata структура метаданных .eopkg пакета
type EopkgMetadata struct {
    XMLName      xml.Name `xml:"PISI"`
    Source       Source   `xml:"Source"`
    Package      Package  `xml:"Package"`
    History      History  `xml:"History"`
}

type Source struct {
    Name        string   `xml:"Name"`
    Homepage    string   `xml:"Homepage"`
    Packager    Packager `xml:"Packager"`
}

type Packager struct {
    Name  string `xml:"Name"`
    Email string `xml:"Email"`
}

type Package struct {
    Name         string       `xml:"Name"`
    Summary      string       `xml:"Summary"`
    Description  string       `xml:"Description"`
    RuntimeDeps  Dependencies `xml:"RuntimeDependencies"`
    Files        Files        `xml:"Files"`
    Architecture string       `xml:"Architecture"`
}

type Dependencies struct {
    Dependency []string `xml:"Dependency"`
}

type Files struct {
    File []File `xml:"File"`
}

type File struct {
    Path      string `xml:"Path,attr"`
    Type      string `xml:"type,attr"`
    Size      int64  `xml:"Size"`
    Hash      string `xml:"Hash"`
}

type History struct {
    Update []Update `xml:"Update"`
}

type Update struct {
    Version     string    `xml:"Version"`
    Date        time.Time `xml:"Date"`
    Name        string    `xml:"Name"`
    Email       string    `xml:"Email"`
    Comment     string    `xml:"Comment"`
}

// NewEopkg создает новый экземпляр Eopkg
func NewEopkg(path string) (*Eopkg, error) {
    absPath, err := filepath.Abs(path)
    if err != nil {
        return nil, fmt.Errorf("failed to get absolute path: %w", err)
    }

    eopkg := &Eopkg{
        Path:      absPath,
        BuildDate: time.Now().UTC(),
    }

    if err := eopkg.validate(); err != nil {
        return nil, err
    }

    return eopkg, nil
}

// validate проверяет корректность .eopkg файла
func (e *Eopkg) validate() error {
    if !strings.HasSuffix(e.Path, ".eopkg") {
        return fmt.Errorf("invalid package format: must be .eopkg")
    }

    fi, err := os.Stat(e.Path)
    if err != nil {
        return fmt.Errorf("failed to stat package file: %w", err)
    }

    if fi.Size() == 0 {
        return fmt.Errorf("invalid package: file is empty")
    }

    return nil
}

// Install устанавливает .eopkg пакет
func (e *Eopkg) Install(force bool) error {
    if err := RequireRoot(); err != nil {
        return err
    }

    logger.Infof("Installing Eopkg package: %s", e.Path)

    // Создаем резервную копию
    backupPath, err := CreateBackup("/var/lib/eopkg")
    if err != nil {
        logger.Warnf("Failed to create backup: %v", err)
    } else {
        logger.Infof("Created backup: %s", backupPath)
    }

    // Подготавливаем команду установки
    args := []string{"install"}
    if force {
        args = append(args, "--ignore-dependency", "--ignore-safety")
    }
    args = append(args, e.Path)

    // Выполняем установку
    cmd := exec.Command("eopkg", args...)
    cmd.Env = append(os.Environ(), "LANG=C")
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("installation failed: %s: %w", string(output), err)
    }

    // Обновляем кэш
    if err := exec.Command("eopkg", "index", "--rebuild-db").Run(); err != nil {
        logger.Warn("Failed to rebuild package database")
    }

    logger.Info("Package installed successfully")
    return nil
}

// Remove удаляет установленный пакет
func (e *Eopkg) Remove(purge bool) error {
    if err := RequireRoot(); err != nil {
        return err
    }

    if e.Name == "" {
        info, err := e.GetInfo()
        if err != nil {
            return fmt.Errorf("failed to get package info: %w", err)
        }
        e.Name = info.Name
    }

    logger.Infof("Removing Eopkg package: %s", e.Name)

    // Создаем резервную копию
    backupPath, err := CreateBackup("/var/lib/eopkg")
    if err != nil {
        logger.Warnf("Failed to create backup: %v", err)
    } else {
        logger.Infof("Created backup: %s", backupPath)
    }

    // Подготавливаем команду удаления
    args := []string{"remove"}
    if purge {
        args = append(args, "--purge")
    }
    args = append(args, e.Name)

    // Выполняем удаление
    cmd := exec.Command("eopkg", args...)
    cmd.Env = append(os.Environ(), "LANG=C")
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("removal failed: %s: %w", string(output), err)
    }

    // Очищаем кэш если указан purge
    if purge {
        if err := exec.Command("eopkg", "delete-cache").Run(); err != nil {
            logger.Warn("Failed to clean package cache")
        }
    }

    logger.Info("Package removed successfully")
    return nil
}

// GetInfo возвращает информацию о пакете
func (e *Eopkg) GetInfo() (*PackageInfo, error) {
    if e.Info != nil {
        return e.Info, nil
    }

    // Открываем .eopkg файл
    f, err := os.Open(e.Path)
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

    var metadata *EopkgMetadata

    // Ищем metadata.xml
    for {
        header, err := tr.Next()
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, fmt.Errorf("failed to read tar header: %w", err)
        }

        if header.Name == "metadata.xml" {
            buf := new(bytes.Buffer)
            if _, err := io.Copy(buf, tr); err != nil {
                return nil, fmt.Errorf("failed to read metadata.xml: %w", err)
            }

            metadata = &EopkgMetadata{}
            if err := xml.Unmarshal(buf.Bytes(), metadata); err != nil {
                return nil, fmt.Errorf("failed to parse metadata: %w", err)
            }
            break
        }
    }

    if metadata == nil {
        return nil, fmt.Errorf("package metadata not found")
    }

    // Получаем размер файла
    var totalSize int64
    for _, file := range metadata.Package.Files.File {
        totalSize += file.Size
    }

    // Создаем информацию о пакете
    info := &PackageInfo{
        Name:         metadata.Package.Name,
        Version:      metadata.History.Update[0].Version,
        Architecture: metadata.Package.Architecture,
        Description:  metadata.Package.Description,
        Maintainer:   fmt.Sprintf("%s <%s>", metadata.Source.Packager.Name, metadata.Source.Packager.Email),
        Homepage:     metadata.Source.Homepage,
        Size:         totalSize,
        InstallDate:  metadata.History.Update[0].Date,
    }

    // Добавляем зависимости
    if metadata.Package.RuntimeDeps.Dependency != nil {
        info.Dependencies = metadata.Package.RuntimeDeps.Dependency
    }

    e.Info = info
    return info, nil
}

// GetType возвращает тип пакета
func (e *Eopkg) GetType() PackageType {
    return TypeEopkg
}

// String возвращает строковое представление пакета
func (e *Eopkg) String() string {
    if e.Info != nil {
        return fmt.Sprintf("%s-%s.eopkg", e.Info.Name, e.Info.Version)
    }
    return filepath.Base(e.Path)
}