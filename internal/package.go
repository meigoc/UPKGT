// internal/package.go
package internal

import (
    "fmt"
    "time"
)

// BuildInfo содержит информацию о сборке
var BuildInfo = struct {
    Version   string
    BuildDate string
    Author    string
}{
    Version:   "1.0.0",
    BuildDate: "2025-02-01 13:49:06",
    Author:    "NurOS-Linux",
}

// PackageType тип пакета
type PackageType int

const (
    TypeUnknown PackageType = iota
    TypeDeb     // Debian/Ubuntu
    TypeRPM     // RedHat/Fedora
    TypeEopkg   // Solus
    TypePacman  // Arch Linux
    TypeAPK     // Alpine Linux
)

// String возвращает строковое представление типа пакета
func (pt PackageType) String() string {
    return [...]string{
        "unknown",
        "deb",
        "rpm",
        "eopkg",
        "pacman",
        "apk",
    }[pt]
}

// Package интерфейс для всех типов пакетов
type Package interface {
    // Install устанавливает пакет
    Install(force bool) error
    
    // Remove удаляет пакет
    Remove(purge bool) error
    
    // GetInfo возвращает информацию о пакете
    GetInfo() (*PackageInfo, error)
    
    // GetType возвращает тип пакета
    GetType() PackageType
    
    // String возвращает строковое представление пакета
    String() string
}

// PackageInfo содержит метаданные пакета
type PackageInfo struct {
    Name            string    // Имя пакета
    Version         string    // Версия
    Architecture    string    // Архитектура
    Description     string    // Описание
    Maintainer      string    // Сопровождающий
    Homepage        string    // Домашняя страница
    Size            int64     // Размер в байтах
    InstalledSize   int64     // Размер после установки
    Dependencies    []string  // Зависимости
    Conflicts      []string  // Конфликты
    Provides       []string  // Предоставляет
    Replaces       []string  // Заменяет
    InstallDate    time.Time // Дата установки
    License        string    // Лицензия
    Section        string    // Секция/категория
    Priority       string    // Приоритет
}

// PackageError ошибка при работе с пакетом
type PackageError struct {
    Code     int         // Код ошибки
    Message  string      // Сообщение об ошибке
    Package  string      // Имя пакета
    Type     PackageType // Тип пакета
    Original error       // Оригинальная ошибка
}

func (e *PackageError) Error() string {
    if e.Original != nil {
        return fmt.Sprintf("[%s] %s: %v", e.Package, e.Message, e.Original)
    }
    return fmt.Sprintf("[%s] %s", e.Package, e.Message)
}

// PackageManager интерфейс для управления пакетами
type PackageManager interface {
    // CreatePackage создает новый пакет из файла
    CreatePackage(path string) (Package, error)
    
    // ListInstalled возвращает список установленных пакетов
    ListInstalled() ([]PackageInfo, error)
    
    // IsInstalled проверяет установлен ли пакет
    IsInstalled(name string) bool
    
    // GetDependencies возвращает список зависимостей
    GetDependencies(pkg Package) ([]string, error)
    
    // ValidateSystem проверяет систему на совместимость
    ValidateSystem() error
    
    // GetType возвращает тип пакетного менеджера
    GetType() PackageType
}

// Constants для путей и настроек
const (
    // Корневая директория для установки
    DefaultInstallRoot = "/"
    
    // Директория для резервных копий
    BackupDir = "/var/backups/upkgt"
    
    // Директория для базы данных
    DBDir = "/var/lib/upkgt"
    
    // Директория для кэша
    CacheDir = "/var/cache/upkgt"
    
    // Временная директория
    TempDir = "/tmp/upkgt"
)

// Error codes
const (
    ErrUnknown = iota + 1
    ErrInvalidPackage
    ErrNotFound
    ErrPermissionDenied
    ErrDependencyMissing
    ErrConflict
    ErrSystemIncompatible
    ErrBackupFailed
    ErrInstallFailed
    ErrRemoveFailed
    ErrDatabaseError
)

// Package validation errors
var (
    ErrEmptyPackage     = &PackageError{Code: ErrInvalidPackage, Message: "package is empty"}
    ErrInvalidFormat    = &PackageError{Code: ErrInvalidPackage, Message: "invalid package format"}
    ErrCorruptedPackage = &PackageError{Code: ErrInvalidPackage, Message: "package is corrupted"}
    ErrNotSupported     = &PackageError{Code: ErrSystemIncompatible, Message: "package type not supported"}
)

// CreatePackageFromPath создает пакет нужного типа на основе расширения файла
func CreatePackageFromPath(path string) (Package, error) {
    switch {
    case strings.HasSuffix(path, ".deb"):
        return NewDeb(path)
    case strings.HasSuffix(path, ".rpm"):
        return NewRPM(path)
    case strings.HasSuffix(path, ".eopkg"):
        return NewEopkg(path)
    case strings.HasSuffix(path, ".apk"):
        return NewAPK(path)
    case strings.Contains(path, ".pkg.tar"):
        return NewPacman(path)
    default:
        return nil, ErrNotSupported
    }
}

// ValidatePackageName проверяет корректность имени пакета
func ValidatePackageName(name string) error {
    if name == "" {
        return fmt.Errorf("package name cannot be empty")
    }
    
    if strings.ContainsAny(name, "/#%*[](){}<>\\|\"'`~") {
        return fmt.Errorf("package name contains invalid characters")
    }
    
    return nil
}

// FormatSize форматирует размер в человекочитаемый вид
func FormatSize(size int64) string {
    const unit = 1024
    if size < unit {
        return fmt.Sprintf("%d B", size)
    }
    div, exp := int64(unit), 0
    for n := size / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %ciB", float64(size)/float64(div), "KMGTPE"[exp])
}

// CompareVersions сравнивает версии пакетов
// Возвращает:
//   -1 если v1 < v2
//    0 если v1 = v2
//    1 если v1 > v2
func CompareVersions(v1, v2 string) int {
    // TODO: Implement proper version comparison
    return strings.Compare(v1, v2)
}