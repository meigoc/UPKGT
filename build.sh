#!/bin/bash

# build.sh
# Author: NurOS-Linux
# Build script for UPKGT (Universal Package Tool)
# Created: 2025-02-01 13:56:35 UTC

set -e # Exit on error

# Configuration
PROJECT_NAME="upkgt"
VERSION="1.0.0"
AUTHOR="NurOS-Linux"
BUILD_DATE="2025-02-01 13:56:35"
GO_VERSION="1.21"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging function
log() {
    echo -e "${2:-$BLUE}[$(date +"%Y-%m-%d %H:%M:%S")] $1${NC}"
}

# Check requirements
check_requirements() {
    log "Checking requirements..." "$YELLOW"
    
    # Check Go version
    if ! command -v go >/dev/null; then
        log "Go is not installed!" "$RED"
        exit 1
    }
    
    GO_CURRENT_VERSION=$(go version | cut -d " " -f 3 | tr -d "go")
    if [[ "$GO_CURRENT_VERSION" < "$GO_VERSION" ]]; then
        log "Required Go version $GO_VERSION or higher (current: $GO_CURRENT_VERSION)" "$RED"
        exit 1
    }
    
    # Check required commands
    for cmd in git make tar gzip; do
        if ! command -v $cmd >/dev/null; then
            log "$cmd is not installed!" "$RED"
            exit 1
        fi
    done
    
    log "All requirements met" "$GREEN"
}

# Clean build artifacts
clean() {
    log "Cleaning build artifacts..." "$YELLOW"
    rm -rf ./bin
    rm -rf ./dist
    go clean
    log "Clean completed" "$GREEN"
}

# Run tests
run_tests() {
    log "Running tests..." "$YELLOW"
    go test -v ./...
    log "Tests completed" "$GREEN"
}

# Build binary for current platform
build_current() {
    log "Building for current platform..." "$YELLOW"
    
    mkdir -p bin
    
    # Inject build info
    BUILD_INFO="-X 'main.Version=$VERSION' \
                -X 'main.BuildDate=$BUILD_DATE' \
                -X 'main.Author=$AUTHOR'"
    
    go build -ldflags "$BUILD_INFO" -o "./bin/$PROJECT_NAME"
    
    log "Build completed: ./bin/$PROJECT_NAME" "$GREEN"
}

# Build for all platforms
build_all() {
    log "Building for all platforms..." "$YELLOW"
    
    mkdir -p dist
    
    # Platforms to build for
    platforms=("linux/amd64" "linux/386" "linux/arm64" "linux/arm" 
              "darwin/amd64" "darwin/arm64"
              "windows/amd64" "windows/386")
    
    for platform in "${platforms[@]}"; do
        platform_split=(${platform//\// })
        GOOS=${platform_split[0]}
        GOARCH=${platform_split[1]}
        
        output_name=$PROJECT_NAME
        if [ $GOOS = "windows" ]; then
            output_name+='.exe'
        fi
        
        # Inject build info
        BUILD_INFO="-X 'main.Version=$VERSION' \
                    -X 'main.BuildDate=$BUILD_DATE' \
                    -X 'main.Author=$AUTHOR'"
        
        log "Building for $GOOS/$GOARCH..." "$YELLOW"
        
        env GOOS=$GOOS GOARCH=$GOARCH \
            go build -ldflags "$BUILD_INFO" \
            -o "./dist/${PROJECT_NAME}-${GOOS}-${GOARCH}/${output_name}"
        
        # Create archive
        cd dist
        if [ $GOOS = "windows" ]; then
            zip -r "${PROJECT_NAME}-${GOOS}-${GOARCH}.zip" "${PROJECT_NAME}-${GOOS}-${GOARCH}"
        else
            tar -czf "${PROJECT_NAME}-${GOOS}-${GOARCH}.tar.gz" "${PROJECT_NAME}-${GOOS}-${GOARCH}"
        fi
        cd ..
        
        log "Built: dist/${PROJECT_NAME}-${GOOS}-${GOARCH}" "$GREEN"
    done
    
    log "All platforms built successfully" "$GREEN"
}

# Create distribution packages
create_packages() {
    log "Creating distribution packages..." "$YELLOW"
    
    mkdir -p dist/packages
    
    # Build binary
    build_current
    
    # Create DEB package
    if command -v dpkg-deb >/dev/null; then
        log "Creating DEB package..." "$YELLOW"
        
        # Create package structure
        PKG_DIR="dist/packages/$PROJECT_NAME"
        mkdir -p "$PKG_DIR/DEBIAN"
        mkdir -p "$PKG_DIR/usr/local/bin"
        
        # Copy binary
        cp "bin/$PROJECT_NAME" "$PKG_DIR/usr/local/bin/"
        
        # Create control file
        cat > "$PKG_DIR/DEBIAN/control" << EOF
Package: $PROJECT_NAME
Version: $VERSION
Section: utils
Priority: optional
Architecture: $(dpkg --print-architecture)
Depends: dpkg, rpm, eopkg | pisi, pacman, apk-tools
Maintainer: $AUTHOR
Description: Universal Package Manager Tool
 UPKGT is a universal package management tool that supports
 multiple package formats including DEB, RPM, EOPKG, Pacman, and APK.
EOF
        
        # Build package
        dpkg-deb --build "$PKG_DIR" "dist/packages/${PROJECT_NAME}_${VERSION}_$(dpkg --print-architecture).deb"
        
        log "DEB package created" "$GREEN"
    fi
    
    # Create RPM package
    if command -v rpmbuild >/dev/null; then
        log "Creating RPM package..." "$YELLOW"
        
        # Create package structure
        mkdir -p dist/packages/rpm/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
        
        # Create spec file
        cat > "dist/packages/rpm/SPECS/$PROJECT_NAME.spec" << EOF
Name:           $PROJECT_NAME
Version:        $VERSION
Release:        1%{?dist}
Summary:        Universal Package Manager Tool
License:        MIT
URL:            https://github.com/$AUTHOR/$PROJECT_NAME
BuildRequires:  golang >= $GO_VERSION

%description
UPKGT is a universal package management tool that supports
multiple package formats including DEB, RPM, EOPKG, Pacman, and APK.

%install
mkdir -p %{buildroot}/usr/local/bin
cp ../../bin/%{name} %{buildroot}/usr/local/bin/

%files
/usr/local/bin/%{name}

%changelog
* $(date "+%a %b %d %Y") $AUTHOR <$AUTHOR@noreply.github.com> - $VERSION-1
- Initial package
EOF
        
        # Build package
        rpmbuild -bb "dist/packages/rpm/SPECS/$PROJECT_NAME.spec" --define "_topdir $(pwd)/dist/packages/rpm"
        
        log "RPM package created" "$GREEN"
    fi
    
    log "Package creation completed" "$GREEN"
}

# Main execution
main() {
    log "Starting build process for $PROJECT_NAME v$VERSION" "$BLUE"
    
    case "$1" in
        "clean")
            clean
            ;;
        "test")
            run_tests
            ;;
        "build")
            check_requirements
            build_current
            ;;
        "build-all")
            check_requirements
            build_all
            ;;
        "packages")
            check_requirements
            create_packages
            ;;
        "all")
            check_requirements
            clean
            run_tests
            build_all
            create_packages
            ;;
        *)
            echo "Usage: $0 {clean|test|build|build-all|packages|all}"
            exit 1
            ;;
    esac
    
    log "Build process completed" "$GREEN"
}

# Execute main with arguments
main "$@"