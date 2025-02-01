// internal/rpm.go
package internal

import (
    "bytes"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "strings"
    "time"
)

// RPM структура для Red Hat Package Manager пакетов
type RPM struct {
    Path       string
    Name       string
    Version    string
    BuildDate  time.Time
    Info       *PackageInfo
}

// RPMMetadata структура метаданных .rpm пакета
type RPMMetadata struct {
    Name         string
    Version      string
    Release      string
    Architecture string
    Group        string
    Size         int64
    License      string
    Signature    string
    BuildDate    time.Time
    Vendor       string
    Description  string
    URL          string
    Dependencies []string
    Provides     []string
    Conflicts    []string
}

// NewRPM создает новый экземпляр RPM
func NewRPM(path string) (*RPM, error) {
    absPath, err := filepath.Abs(path)
    if err != nil {
        return nil, fmt.Errorf("failed to get absolute path: %w", err)
    }

    rpm := &RPM{
        Path:      absPath,
        BuildDate: time.Now().UTC(),
    }

    if err := rpm.validate(); err != nil {
        return nil, err
    }

    return rpm, nil
}

// validate проверяет корректность .rpm файла
func (r *RPM) validate() error {
    if !strings.HasSuffix(r.Path, ".rpm") {
        return fmt.Errorf("invalid package format: must be .rpm")
    }

    fi, err := os.Stat(r.Path)
    if err != nil {
        return fmt.Errorf("failed to stat package file: %w", err)
    }

    if fi.Size() == 0 {
        return fmt.Errorf("invalid package: file is empty")
    }

    // Проверка сигнатуры RPM
    cmd := exec.Command("rpm", "-K", r.Path)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("invalid RPM signature: %w", err)
    }

    return nil
}

// Install устанавливает .rpm пакет
func (r *RPM) Install(force bool) error {
    if err := RequireRoot(); err != nil {
        return err
    }

    logger.Infof("Installing RPM package: %s", r.Path)

    // Создаем резервную копию RPM базы
    backupPath, err := CreateBackup("/var/lib/rpm")
    if err != nil {
        logger.Warnf("Failed to create backup: %v", err)
    } else {
        logger.Infof("Created backup: %s", backupPath)
    }

    // Подготавливаем команду установки
    args := []string{"-i"}
    if force {
        args = append(args, "--force", "--nodeps")
    }
    args = append(args, r.Path)

    // Выполняем установку
    cmd := exec.Command("rpm", args...)
    cmd.Env = append(os.Environ(), "LANG=C")
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("installation failed: %s: %w", string(output), err)
    }

    // Проверяем успешность установки
    if info, err := r.GetInfo(); err == nil {
        cmd = exec.Command("rpm", "-q", info.Name)
        if err := cmd.Run(); err != nil {
            return fmt.Errorf("package verification failed after installation")
        }
    }

    logger.Info("Package installed successfully")
    return nil
}

// Remove удаляет установленный пакет
func (r *RPM) Remove(purge bool) error {
    if err := RequireRoot(); err != nil {
        return err
    }

    if r.Name == "" {
        info, err := r.GetInfo()
        if err != nil {
            return fmt.Errorf("failed to get package info: %w", err)
        }
        r.Name = info.Name
    }

    logger.Infof("Removing RPM package: %s", r.Name)

    // Создаем резервную копию RPM базы
    backupPath, err := CreateBackup("/var/lib/rpm")
    if err != nil {
        logger.Warnf("Failed to create backup: %v", err)
    } else {
        logger.Infof("Created backup: %s", backupPath)
    }

    // Подготавливаем команду удаления
    args := []string{"-e"}
    if !purge {
        args = append(args, "--nodeps")
    }
    args = append(args, r.Name)

    // Выполняем удаление
    cmd := exec.Command("rpm", args...)
    cmd.Env = append(os.Environ(), "LANG=C")
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("removal failed: %s: %w", string(output), err)
    }

    // Проверяем успешность удаления
    cmd = exec.Command("rpm", "-q", r.Name)
    if err := cmd.Run(); err == nil {
        return fmt.Errorf("package still installed after removal")
    }

    logger.Info("Package removed successfully")
    return nil
}

// GetInfo возвращает информацию о пакете
func (r *RPM) GetInfo() (*PackageInfo, error) {
    if r.Info != nil {
        return r.Info, nil
    }

    // Получаем метаданные через rpm команду
    cmd := exec.Command("rpm", "-qip", r.Path)
    cmd.Env = append(os.Environ(), "LANG=C")
    
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("failed to get package info: %w", err)
    }

    metadata, err := parseRPMMetadata(output)
    if err != nil {
        return nil, fmt.Errorf("failed to parse package metadata: %w", err)
    }

    // Получаем зависимости
    cmd = exec.Command("rpm", "-qpR", r.Path)
    cmd.Env = append(os.Environ(), "LANG=C")
    
    deps, err := cmd.Output()
    if err == nil {
        metadata.Dependencies = strings.Split(string(deps), "\n")
    }

    // Создаем информацию о пакете
    info := &PackageInfo{
        Name:         metadata.Name,
        Version:      fmt.Sprintf("%s-%s", metadata.Version, metadata.Release),
        Architecture: metadata.Architecture,
        Description:  metadata.Description,
        Homepage:     metadata.URL,
        Size:         metadata.Size,
        Dependencies: metadata.Dependencies,
        Provides:     metadata.Provides,
        Conflicts:    metadata.Conflicts,
        InstallDate:  metadata.BuildDate,
    }

    r.Info = info
    return info, nil
}

// parseRPMMetadata парсит вывод команды rpm -qip
func parseRPMMetadata(data []byte) (*RPMMetadata, error) {
    metadata := &RPMMetadata{}
    lines := strings.Split(string(data), "\n")

    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }

        parts := strings.SplitN(line, ":", 2)
        if len(parts) != 2 {
            continue
        }

        key := strings.TrimSpace(parts[0])
        value := strings.TrimSpace(parts[1])

        switch key {
        case "Name":
            metadata.Name = value
        case "Version":
            metadata.Version = value
        case "Release":
            metadata.Release = value
        case "Architecture":
            metadata.Architecture = value
        case "Group":
            metadata.Group = value
        case "Size":
            if size, err := strconv.ParseInt(strings.Fields(value)[0], 10, 64); err == nil {
                metadata.Size = size
            }
        case "License":
            metadata.License = value
        case "Signature":
            metadata.Signature = value
        case "Build Date":
            if t, err := time.Parse("Mon Jan 2 15:04:05 2006", value); err == nil {
                metadata.BuildDate = t
            }
        case "Vendor":
            metadata.Vendor = value
        case "URL":
            metadata.URL = value
        case "Summary", "Description":
            if metadata.Description == "" {
                metadata.Description = value
            } else {
                metadata.Description += "\n" + value
            }
        }
    }

    return metadata, nil
}

// GetType возвращает тип пакета
func (r *RPM) GetType() PackageType {
    return TypeRPM
}

// String возвращает строковое представление пакета
func (r *RPM) String() string {
    if r.Info != nil {
        return fmt.Sprintf("%s-%s.rpm", r.Info.Name, r.Info.Version)
    }
    return filepath.Base(r.Path)
}

// VerifyDependencies проверяет зависимости пакета
func (r *RPM) VerifyDependencies() error {
    cmd := exec.Command("rpm", "-qpR", r.Path)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("failed to verify dependencies: %s: %w", string(output), err)
    }
    return nil
}

// GetScripts возвращает установочные скрипты пакета
func (r *RPM) GetScripts() (map[string]string, error) {
    scripts := make(map[string]string)
    scriptTypes := []string{"prein", "postin", "preun", "postun"}

    for _, scriptType := range scriptTypes {
        cmd := exec.Command("rpm", "-qp", "--scripts", r.Path)
        output, err := cmd.Output()
        if err != nil {
            continue
        }

        scripts[scriptType] = string(output)
    }

    return scripts, nil
}