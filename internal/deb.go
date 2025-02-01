// internal/deb.go
package internal

import (
    "archive/tar"
    "bytes"
    "compress/gzip"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
    "time"
)

// Deb структура для Debian пакетов
type Deb struct {
    Path       string
    Name       string
    Version    string
    BuildDate  time.Time
    Info       *PackageInfo
}

// DebControl структура для control файла
type DebControl struct {
    Package      string
    Version      string
    Architecture string
    Maintainer   string
    Description  string
    Homepage     string
    Section      string
    Priority     string
    Depends      []string
    PreDepends   []string
    Recommends   []string
    Suggests     []string
    Conflicts    []string
    Provides     []string
    Replaces     []string
    Size         int64
}

// NewDeb создает новый экземпляр Deb
func NewDeb(path string) (*Deb, error) {
    absPath, err := filepath.Abs(path)
    if err != nil {
        return nil, fmt.Errorf("failed to get absolute path: %w", err)
    }

    deb := &Deb{
        Path:      absPath,
        BuildDate: time.Now().UTC(),
    }

    if err := deb.validate(); err != nil {
        return nil, err
    }

    return deb, nil
}

// validate проверяет корректность .deb файла
func (d *Deb) validate() error {
    if !strings.HasSuffix(d.Path, ".deb") {
        return fmt.Errorf("invalid package format: must be .deb")
    }

    fi, err := os.Stat(d.Path)
    if err != nil {
        return fmt.Errorf("failed to stat package file: %w", err)
    }

    if fi.Size() == 0 {
        return fmt.Errorf("invalid package: file is empty")
    }

    // Проверка магических чисел deb пакета
    f, err := os.Open(d.Path)
    if err != nil {
        return err
    }
    defer f.Close()

    magic := make([]byte, 8)
    if _, err := f.Read(magic); err != nil {
        return fmt.Errorf("failed to read package header: %w", err)
    }

    if !bytes.Equal(magic[:2], []byte("!<")) {
        return fmt.Errorf("invalid package format: not a debian package")
    }

    return nil
}

// Install устанавливает .deb пакет
func (d *Deb) Install(force bool) error {
    if err := RequireRoot(); err != nil {
        return err
    }

    logger.Infof("Installing Debian package: %s", d.Path)

    // Создаем резервную копию
    backupPath, err := CreateBackup("/var/lib/dpkg")
    if err != nil {
        logger.Warnf("Failed to create backup: %v", err)
    } else {
        logger.Infof("Created backup: %s", backupPath)
    }

    // Подготавливаем команду установки
    args := []string{"-i"}
    if force {
        args = append(args, "--force-all")
    }
    args = append(args, d.Path)

    // Выполняем установку
    cmd := exec.Command("dpkg", args...)
    cmd.Env = append(os.Environ(), "LANG=C")
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        // Пытаемся исправить зависимости
        fixCmd := exec.Command("apt-get", "install", "-f", "-y")
        if fixOut, fixErr := fixCmd.CombinedOutput(); fixErr != nil {
            return fmt.Errorf("installation failed: %s\nFix attempt failed: %s", string(output), string(fixOut))
        }
        return fmt.Errorf("installation completed with warnings: %s", string(output))
    }

    // Обновляем кэш
    if err := exec.Command("apt-get", "update").Run(); err != nil {
        logger.Warn("Failed to update package cache")
    }

    logger.Info("Package installed successfully")
    return nil
}

// Remove удаляет установленный пакет
func (d *Deb) Remove(purge bool) error {
    if err := RequireRoot(); err != nil {
        return err
    }

    if d.Name == "" {
        info, err := d.GetInfo()
        if err != nil {
            return fmt.Errorf("failed to get package info: %w", err)
        }
        d.Name = info.Name
    }

    logger.Infof("Removing Debian package: %s", d.Name)

    // Создаем резервную копию
    backupPath, err := CreateBackup("/var/lib/dpkg")
    if err != nil {
        logger.Warnf("Failed to create backup: %v", err)
    } else {
        logger.Infof("Created backup: %s", backupPath)
    }

    // Подготавливаем команду удаления
    args := []string{"remove"}
    if purge {
        args = []string{"purge"} // purge удаляет также конфигурационные файлы
    }
    args = append(args, d.Name)

    // Выполняем удаление
    cmd := exec.Command("dpkg", args...)
    cmd.Env = append(os.Environ(), "LANG=C")
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("removal failed: %s: %w", string(output), err)
    }

    // Очищаем неиспользуемые зависимости
    if err := exec.Command("apt-get", "autoremove", "-y").Run(); err != nil {
        logger.Warn("Failed to remove unused dependencies")
    }

    // Очищаем кэш если указан purge
    if purge {
        if err := exec.Command("apt-get", "clean").Run(); err != nil {
            logger.Warn("Failed to clean package cache")
        }
    }

    logger.Info("Package removed successfully")
    return nil
}

// GetInfo возвращает информацию о пакете
func (d *Deb) GetInfo() (*PackageInfo, error) {
    if d.Info != nil {
        return d.Info, nil
    }

    // Используем dpkg-deb для получения control файла
    cmd := exec.Command("dpkg-deb", "-f", d.Path)
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("failed to read control file: %w", err)
    }

    control, err := parseControl(string(output))
    if err != nil {
        return nil, fmt.Errorf("failed to parse control file: %w", err)
    }

    // Получаем размер установленного пакета
    cmd = exec.Command("dpkg-deb", "-I", d.Path)
    if output, err = cmd.Output(); err == nil {
        re := regexp.MustCompile(`installed size: (\d+)`)
        if matches := re.FindStringSubmatch(strings.ToLower(string(output))); len(matches) > 1 {
            control.Size, _ = strconv.ParseInt(matches[1], 10, 64)
        }
    }

    // Создаем информацию о пакете
    info := &PackageInfo{
        Name:         control.Package,
        Version:      control.Version,
        Architecture: control.Architecture,
        Description:  control.Description,
        Maintainer:   control.Maintainer,
        Homepage:     control.Homepage,
        Size:         control.Size,
        Dependencies: control.Depends,
        Conflicts:    control.Conflicts,
        Provides:     control.Provides,
        Replaces:     control.Replaces,
        InstallDate:  d.BuildDate,
        Section:      control.Section,
        Priority:     control.Priority,
    }

    d.Info = info
    return info, nil
}

// parseControl парсит debian control файл
func parseControl(data string) (*DebControl, error) {
    control := &DebControl{}
    current := ""
    description := []string{}

    for _, line := range strings.Split(data, "\n") {
        if strings.HasPrefix(line, " ") {
            if current == "Description" {
                description = append(description, strings.TrimSpace(line))
            }
            continue
        }

        parts := strings.SplitN(line, ":", 2)
        if len(parts) != 2 {
            continue
        }

        key := strings.TrimSpace(parts[0])
        value := strings.TrimSpace(parts[1])
        current = key

        switch key {
        case "Package":
            control.Package = value
        case "Version":
            control.Version = value
        case "Architecture":
            control.Architecture = value
        case "Maintainer":
            control.Maintainer = value
        case "Description":
            description = append(description, value)
        case "Homepage":
            control.Homepage = value
        case "Section":
            control.Section = value
        case "Priority":
            control.Priority = value
        case "Depends":
            control.Depends = parseDepends(value)
        case "Pre-Depends":
            control.PreDepends = parseDepends(value)
        case "Recommends":
            control.Recommends = parseDepends(value)
        case "Suggests":
            control.Suggests = parseDepends(value)
        case "Conflicts":
            control.Conflicts = parseDepends(value)
        case "Provides":
            control.Provides = parseDepends(value)
        case "Replaces":
            control.Replaces = parseDepends(value)
        case "Installed-Size":
            control.Size, _ = strconv.ParseInt(value, 10, 64)
            control.Size *= 1024 // Convert to bytes
        }
    }

    control.Description = strings.Join(description, "\n")
    return control, nil
}

// parseDepends парсит строку зависимостей debian пакета
func parseDepends(deps string) []string {
    if deps == "" {
        return nil
    }

    var result []string
    for _, dep := range strings.Split(deps, ",") {
        dep = strings.TrimSpace(dep)
        if dep != "" {
            // Оставляем только имя пакета, убираем версионные ограничения
            if i := strings.Index(dep, " "); i > 0 {
                dep = dep[:i]
            }
            result = append(result, dep)
        }
    }
    return result
}

// GetType возвращает тип пакета
func (d *Deb) GetType() PackageType {
    return TypeDeb
}

// String возвращает строковое представление пакета
func (d *Deb) String() string {
    if d.Info != nil {
        return fmt.Sprintf("%s_%s_%s.deb", d.Info.Name, d.Info.Version, d.Info.Architecture)
    }
    return filepath.Base(d.Path)
}

// VerifySignature проверяет подпись пакета
func (d *Deb) VerifySignature() error {
    cmd := exec.Command("dpkg-sig", "--verify", d.Path)
    if output, err := cmd.CombinedOutput(); err != nil {
        return fmt.Errorf("signature verification failed: %s: %w", string(output), err)
    }
    return nil
}

// ExtractControl извлекает control файл из пакета
func (d *Deb) ExtractControl() (string, error) {
    cmd := exec.Command("dpkg-deb", "-I", d.Path)
    output, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("failed to extract control: %w", err)
    }
    return string(output), nil
}