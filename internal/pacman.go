// internal/pacman.go
package internal

import (
    "archive/tar"
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"
)

// Pacman структура для Arch Linux пакетов
type Pacman struct {
    Path       string
    Name       string
    Version    string
    BuildDate  time.Time
    Info       *PackageInfo
}

// PacmanMetadata структура метаданных .pkg.tar.* пакета 
type PacmanMetadata struct {
    Name          string   `json:"name"`
    Version       string   `json:"version"`
    Description   string   `json:"desc"`
    Architecture  string   `json:"arch"`
    URL          string   `json:"url"`
    License      []string `json:"license"`
    Groups       []string `json:"groups"`
    Provides     []string `json:"provides"`
    Depends      []string `json:"depends"`
    OptDepends   []string `json:"optdepends"`
    Conflicts    []string `json:"conflicts"`
    Replaces     []string `json:"replaces"`
    Size         int64    `json:"size"`
    Packager     string   `json:"packager"`
    BuildDate    int64    `json:"builddate"`
    Validpgpkeys []string `json:"validpgpkeys"`
}

// NewPacman создает новый экземпляр Pacman
func NewPacman(path string) (*Pacman, error) {
    absPath, err := filepath.Abs(path)
    if err != nil {
        return nil, fmt.Errorf("failed to get absolute path: %w", err)
    }

    pacman := &Pacman{
        Path:      absPath,
        BuildDate: time.Now().UTC(),
    }

    if err := pacman.validate(); err != nil {
        return nil, err
    }

    return pacman, nil
}

// validate проверяет корректность .pkg.tar.* файла
func (p *Pacman) validate() error {
    if !strings.Contains(p.Path, ".pkg.tar") {
        return fmt.Errorf("invalid package format: must be .pkg.tar.*")
    }

    fi, err := os.Stat(p.Path)
    if err != nil {
        return fmt.Errorf("failed to stat package file: %w", err)
    }

    if fi.Size() == 0 {
        return fmt.Errorf("invalid package: file is empty")
    }

    return nil
}

// Install устанавливает .pkg.tar.* пакет
func (p *Pacman) Install(force bool) error {
    if err := RequireRoot(); err != nil {
        return err
    }

    logger.Infof("Installing Pacman package: %s", p.Path)

    // Создаем резервную копию
    backupPath, err := CreateBackup("/var/lib/pacman")
    if err != nil {
        logger.Warnf("Failed to create backup: %v", err)
    } else {
        logger.Infof("Created backup: %s", backupPath)
    }

    // Подготавливаем команду установки
    args := []string{"-U"}
    if force {
        args = append(args, "--force", "--nodeps")
    }
    args = append(args, p.Path)

    // Выполняем установку
    cmd := exec.Command("pacman", args...)
    cmd.Env = append(os.Environ(), "LANG=C")
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("installation failed: %s: %w", string(output), err)
    }

    // Обновляем базу данных
    if err := exec.Command("pacman", "-Sy").Run(); err != nil {
        logger.Warn("Failed to update package database")
    }

    logger.Info("Package installed successfully")
    return nil
}

// Remove удаляет установленный пакет
func (p *Pacman) Remove(purge bool) error {
    if err := RequireRoot(); err != nil {
        return err
    }

    if p.Name == "" {
        info, err := p.GetInfo()
        if err != nil {
            return fmt.Errorf("failed to get package info: %w", err)
        }
        p.Name = info.Name
    }

    logger.Infof("Removing Pacman package: %s", p.Name)

    // Создаем резервную копию
    backupPath, err := CreateBackup("/var/lib/pacman")
    if err != nil {
        logger.Warnf("Failed to create backup: %v", err)
    } else {
        logger.Infof("Created backup: %s", backupPath)
    }

    // Подготавливаем команду удаления
    args := []string{"-R"}
    if purge {
        args = append(args, "-n", "-s") // -n: удалить конфиги, -s: удалить зависимости
    }
    args = append(args, p.Name)

    // Выполняем удаление
    cmd := exec.Command("pacman", args...)
    cmd.Env = append(os.Environ(), "LANG=C")
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("removal failed: %s: %w", string(output), err)
    }

    // Очищаем кэш если указан purge
    if purge {
        if err := exec.Command("pacman", "-Scc", "--noconfirm").Run(); err != nil {
            logger.Warn("Failed to clean package cache")
        }
    }

    logger.Info("Package removed successfully")
    return nil
}

// GetInfo возвращает информацию о пакете
func (p *Pacman) GetInfo() (*PackageInfo, error) {
    if p.Info != nil {
        return p.Info, nil
    }

    // Читаем .PKGINFO
    cmd := exec.Command("tar", "-xOf", p.Path, ".PKGINFO")
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("failed to read .PKGINFO: %w", err)
    }

    metadata := &PacmanMetadata{}
    if err := parsePacmanMetadata(output, metadata); err != nil {
        return nil, fmt.Errorf("failed to parse package metadata: %w", err)
    }

    // Создаем информацию о пакете
    info := &PackageInfo{
        Name:         metadata.Name,
        Version:      metadata.Version,
        Architecture: metadata.Architecture,
        Description:  metadata.Description,
        Homepage:     metadata.URL,
        Size:         metadata.Size,
        Dependencies: metadata.Depends,
        Conflicts:    metadata.Conflicts,
        Provides:     metadata.Provides,
        Replaces:     metadata.Replaces,
        InstallDate:  time.Unix(metadata.BuildDate, 0),
        License:      strings.Join(metadata.License, ", "),
        Maintainer:   metadata.Packager,
    }

    // Добавляем опциональные зависимости в описание
    if len(metadata.OptDepends) > 0 {
        info.Description += "\n\nOptional Dependencies:\n" + strings.Join(metadata.OptDepends, "\n")
    }

    p.Info = info
    return info, nil
}

// parsePacmanMetadata парсит .PKGINFO файл
func parsePacmanMetadata(data []byte, metadata *PacmanMetadata) error {
    lines := strings.Split(string(data), "\n")
    
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }

        parts := strings.SplitN(line, " = ", 2)
        if len(parts) != 2 {
            continue
        }

        key := strings.TrimSpace(parts[0])
        value := strings.TrimSpace(parts[1])

        switch key {
        case "pkgname":
            metadata.Name = value
        case "pkgver":
            metadata.Version = value
        case "pkgdesc":
            metadata.Description = value
        case "arch":
            metadata.Architecture = value
        case "url":
            metadata.URL = value
        case "license":
            metadata.License = append(metadata.License, value)
        case "group":
            metadata.Groups = append(metadata.Groups, value)
        case "provides":
            metadata.Provides = append(metadata.Provides, value)
        case "depend":
            metadata.Depends = append(metadata.Depends, value)
        case "optdepend":
            metadata.OptDepends = append(metadata.OptDepends, value)
        case "conflict":
            metadata.Conflicts = append(metadata.Conflicts, value)
        case "replaces":
            metadata.Replaces = append(metadata.Replaces, value)
        case "size":
            metadata.Size, _ = parseInt64(value)
        case "packager":
            metadata.Packager = value
        case "builddate":
            metadata.BuildDate, _ = parseInt64(value)
        }
    }

    return nil
}

// GetType возвращает тип пакета
func (p *Pacman) GetType() PackageType {
    return TypePacman
}

// String возвращает строковое представление пакета
func (p *Pacman) String() string {
    if p.Info != nil {
        return fmt.Sprintf("%s-%s-%s.pkg.tar.*", p.Info.Name, p.Info.Version, p.Info.Architecture)
    }
    return filepath.Base(p.Path)
}

// VerifySignature проверяет подпись пакета
func (p *Pacman) VerifySignature() error {
    cmd := exec.Command("pacman-key", "--verify", p.Path)
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("signature verification failed: %s: %w", string(output), err)
    }
    return nil
}

// ExtractFile извлекает файл из пакета
func (p *Pacman) ExtractFile(filename string, dest string) error {
    cmd := exec.Command("tar", "-xf", p.Path, "-C", dest, filename)
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("failed to extract file: %s: %w", string(output), err)
    }
    return nil
}