#!/bin/bash

# build.sh - Simple cross-platform build script
# Output: ./bin/vsi-$OS-$ARCH (or .exe)

set -e

APP_NAME="vsi"
MAIN_FILE="cmd/vsi/main.go"

echo "Build script for $APP_NAME"

# Build current platform
build_current() {
    OS=$(go env GOOS)
    ARCH=$(go env GOARCH)
    
    echo "Building for current platform: $OS/$ARCH"
    
    mkdir -p ./bin
    
    OUTPUT="./bin/${APP_NAME}-${OS}-${ARCH}"
    if [[ "$OS" == "windows" ]]; then
        OUTPUT="${OUTPUT}.exe"
    fi
    
    echo "Output: $OUTPUT"
    
    go build -o "$OUTPUT" "$MAIN_FILE"
    
    if [[ $? -eq 0 ]]; then
        echo "✓ Build successful"
        if [[ "$OS" != "windows" ]]; then
            chmod +x "$OUTPUT"
        fi
    else
        echo "✗ Build failed"
        exit 1
    fi
}

# Build all platforms
build_all() {
    echo "Building for all platforms..."
    
    # Platform targets
    TARGETS=(
        "linux amd64"
        "linux arm64"
        "linux 386"
        "windows amd64"
        "windows arm64"
        "windows 386"
        "darwin amd64"
        "darwin arm64"
    )
    
    rm -rf ./bin
    mkdir -p ./bin
    
    for target in "${TARGETS[@]}"; do
        read OS ARCH <<< "$target"
        
        OUTPUT="./bin/${APP_NAME}-${OS}-${ARCH}"
        if [[ "$OS" == "windows" ]]; then
            OUTPUT="${OUTPUT}.exe"
        fi
        
        echo "Building: $OS/$ARCH"
        
        if GOOS=$OS GOARCH=$ARCH go build -o "$OUTPUT" "$MAIN_FILE" 2>/dev/null; then
            echo "  ✓ Success"
            if [[ "$OS" != "windows" ]]; then
                chmod +x "$OUTPUT"
            fi
        else
            echo "  ✗ Failed"
        fi
    done
    
    echo "Build completed. Files in ./bin/:"
    ls -lh ./bin/
}

# Main
if [[ "$1" == "all" ]]; then
    build_all
elif [[ "$1" == "clean" ]]; then
    rm -rf ./bin
    echo "Cleaned ./bin/"
elif [[ "$1" == "help" || "$1" == "--help" || "$1" == "-h" ]]; then
    echo "Usage: $0 [all|clean|help]"
    echo "  all    - Build for all platforms"
    echo "  clean  - Clean build directory"
    echo "  help   - Show this help"
    echo "  (none) - Build for current platform"
else
    build_current
fi