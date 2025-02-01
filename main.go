// main.go
package main

import (
    "fmt"
    "log"
    "os"
    "path/filepath"
    "runtime"
    "strings"
    "time"

    "github.com/spf13/cobra"
    "github.com/fatih/color"
    "github.com/sirupsen/logrus"
)

const (
    ProgramName    = "upkgt"
    ProgramVersion = "1.0.0" 
    ProgramAuthor  = "NurOS-Linux"
    BuildDate      = "2025-02-01 13:40:28"
)

var (
    logger = logrus.New()
    verbose bool
    force bool
    purge bool
)

type PackageType int

const (
    TypeUnknown PackageType = iota
    TypeDeb
    TypeRPM 
    TypeEopkg
    TypePacman
    TypeAPK
)

func (pt PackageType) String() string {
    return [...]string{"unknown", "deb", "rpm", "eopkg", "pacman", "apk"}[pt]
}

type PackageError struct {
    Code    int
    Message string 
    Type    PackageType
    Err     error
}

func (e *PackageError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Err)
    }
    return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

func init() {
    logger.SetFormatter(&logrus.TextFormatter{
        FullTimestamp:   true,
        TimestampFormat: "2006-01-02 15:04:05",
    })
    logger.SetOutput(os.Stdout)
}

func detectPackageType(path string) PackageType {
    ext := strings.ToLower(filepath.Ext(path))
    switch ext {
    case ".deb":
        return TypeDeb
    case ".rpm":
        return TypeRPM
    case ".eopkg":
        return TypeEopkg
    case ".apk":
        return TypeAPK
    }
    
    // Check for pacman packages (.pkg.tar.*)
    if strings.HasPrefix(filepath.Base(path), "pkg.tar.") {
        return TypePacman
    }
    
    return TypeUnknown
}

func isRoot() bool {
    return os.Geteuid() == 0 
}

func handleInstall(path string, force bool) error {
    if !isRoot() {
        return &PackageError{
            Code:    1,
            Message: "Root privileges required for installation",
            Type:    TypeUnknown,
        }
    }

    absPath, err := filepath.Abs(path)
    if err != nil {
        return &PackageError{
            Code:    2,
            Message: "Invalid package path",
            Type:    TypeUnknown,
            Err:     err,
        }
    }

    if _, err := os.Stat(absPath); os.IsNotExist(err) {
        return &PackageError{
            Code:    3,
            Message: "Package file not found",
            Type:    TypeUnknown,
            Err:     err,
        }
    }

    pkgType := detectPackageType(absPath)
    if pkgType == TypeUnknown {
        return &PackageError{
            Code:    4,
            Message: "Unsupported package format",
            Type:    TypeUnknown,
        }
    }

    logger.WithFields(logrus.Fields{
        "path": absPath,
        "type": pkgType,
        "force": force,
    }).Info("Installing package")

    // Create backup
    backupDir := "/var/backups/upkgt"
    if err := os.MkdirAll(backupDir, 0755); err != nil {
        logger.Warn("Could not create backup directory")
    }

    // Install package based on type
    switch pkgType {
    case TypeDeb:
        err = installDeb(absPath, force)
    case TypeRPM:
        err = installRPM(absPath, force)
    case TypeEopkg:
        err = installEopkg(absPath, force)
    case TypePacman:
        err = installPacman(absPath, force)
    case TypeAPK:
        err = installAPK(absPath, force)
    }

    if err != nil {
        return &PackageError{
            Code:    5,
            Message: "Installation failed",
            Type:    pkgType,
            Err:     err,
        }
    }

    logger.Info("Package installed successfully")
    return nil
}

func handleRemove(packageName string, purge bool) error {
    if !isRoot() {
        return &PackageError{
            Code:    6,
            Message: "Root privileges required for removal",
            Type:    TypeUnknown,
        }
    }

    logger.WithFields(logrus.Fields{
        "package": packageName,
        "purge":   purge,
    }).Info("Removing package")

    // Detect installed package type
    pkgType := detectInstalledPackageType(packageName)
    if pkgType == TypeUnknown {
        return &PackageError{
            Code:    7,
            Message: "Package not found or unknown format",
            Type:    TypeUnknown,
        }
    }

    // Remove package based on type
    var err error
    switch pkgType {
    case TypeDeb:
        err = removeDeb(packageName, purge)
    case TypeRPM:
        err = removeRPM(packageName, purge)
    case TypeEopkg:
        err = removeEopkg(packageName, purge)
    case TypePacman:
        err = removePacman(packageName, purge)
    case TypeAPK:
        err = removeAPK(packageName, purge)
    }

    if err != nil {
        return &PackageError{
            Code:    8,
            Message: "Removal failed",
            Type:    pkgType,
            Err:     err,
        }
    }

    logger.Info("Package removed successfully")
    return nil
}

func handleInfo(path string) error {
    absPath, err := filepath.Abs(path)
    if err != nil {
        return &PackageError{
            Code:    9,
            Message: "Invalid package path",
            Type:    TypeUnknown,
            Err:     err,
        }
    }

    if _, err := os.Stat(absPath); os.IsNotExist(err) {
        return &PackageError{
            Code:    10,
            Message: "Package file not found",
            Type:    TypeUnknown,
            Err:     err,
        }
    }

    pkgType := detectPackageType(absPath)
    if pkgType == TypeUnknown {
        return &PackageError{
            Code:    11,
            Message: "Unsupported package format",
            Type:    TypeUnknown,
        }
    }

    // Get package info based on type
    var info *PackageInfo
    switch pkgType {
    case TypeDeb:
        info, err = getDebInfo(absPath)
    case TypeRPM:
        info, err = getRPMInfo(absPath)
    case TypeEopkg:
        info, err = getEopkgInfo(absPath)
    case TypePacman:
        info, err = getPacmanInfo(absPath)
    case TypeAPK:
        info, err = getAPKInfo(absPath)
    }

    if err != nil {
        return &PackageError{
            Code:    12,
            Message: "Could not read package info",
            Type:    pkgType,
            Err:     err,
        }
    }

    // Print package information
    fmt.Println(color.GreenString("Package Information:"))
    fmt.Printf("Name: %s\n", info.Name)
    fmt.Printf("Version: %s\n", info.Version)
    fmt.Printf("Architecture: %s\n", info.Architecture)
    fmt.Printf("Size: %d bytes\n", info.Size)
    fmt.Printf("Type: %s\n", pkgType)

    if info.Description != "" {
        fmt.Printf("\nDescription: %s\n", info.Description)
    }

    if len(info.Dependencies) > 0 {
        fmt.Printf("\nDependencies:\n")
        for _, dep := range info.Dependencies {
            fmt.Printf("  - %s\n", dep)
        }
    }

    return nil
}

func main() {
    startTime := time.Now()

    rootCmd := &cobra.Command{
        Use:     ProgramName,
        Version: ProgramVersion,
        Short:   "Universal Package Manager Tool",
        Long: fmt.Sprintf(`UPKGT - Universal Package Manager Tool
Version: %s
Author:  %s
Build:   %s
Go:      %s
OS/Arch: %s/%s`,
            ProgramVersion, ProgramAuthor, BuildDate,
            runtime.Version(), runtime.GOOS, runtime.GOARCH,
        ),
        PersistentPreRun: func(cmd *cobra.Command, args []string) {
            if verbose {
                logger.SetLevel(logrus.DebugLevel)
            }
        },
    }

    // Install command
    installCmd := &cobra.Command{
        Use:   "install [path]",
        Short: "Install a package",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            return handleInstall(args[0], force)
        },
    }
    installCmd.Flags().BoolVarP(&force, "force", "f", false, "Force installation")

    // Remove command
    removeCmd := &cobra.Command{
        Use:   "remove [package]",
        Short: "Remove a package",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            return handleRemove(args[0], purge)
        },
    }
    removeCmd.Flags().BoolVarP(&purge, "purge", "p", false, "Purge configuration files")

    // Info command
    infoCmd := &cobra.Command{
        Use:   "info [path]",
        Short: "Display package information",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            return handleInfo(args[0])
        },
    }

    rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
    rootCmd.AddCommand(installCmd, removeCmd, infoCmd)

    if err := rootCmd.Execute(); err != nil {
        logger.Errorf("Error: %v", err)
        os.Exit(1)
    }

    if verbose {
        elapsed := time.Since(startTime)
        logger.Debugf("Total execution time: %v", elapsed)
    }
}