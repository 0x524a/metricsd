#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PACKAGING_DIR="$PROJECT_ROOT/packaging"
BUILD_DIR="$PROJECT_ROOT/dist"

# Get version from git or use default
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")}"
VERSION="${VERSION#v}"  # Remove 'v' prefix if present

# Architecture mappings: Go GOARCH -> Debian arch
declare -A ARCH_MAP
ARCH_MAP["amd64"]="amd64"
ARCH_MAP["arm64"]="arm64"

# Build for specified architectures or all
ARCHS="${ARCHS:-amd64 arm64}"

echo "Building metricsd DEB packages..."
echo "Version: $VERSION"
echo "Architectures: $ARCHS"

# Create build directory
mkdir -p "$BUILD_DIR"

for GOARCH in $ARCHS; do
    DEBARCH="${ARCH_MAP[$GOARCH]}"
    echo ""
    echo "=== Building for $GOARCH (Debian: $DEBARCH) ==="

    # Build binary
    echo "Building binary..."
    GOOS=linux GOARCH=$GOARCH go build -ldflags "-w -s -X main.Version=$VERSION" \
        -o "$BUILD_DIR/metricsd-$GOARCH" "$PROJECT_ROOT/cmd/metricsd"

    # Create package directory structure
    PKG_DIR="$BUILD_DIR/metricsd-${VERSION}-${DEBARCH}"
    rm -rf "$PKG_DIR"
    mkdir -p "$PKG_DIR"/{DEBIAN,usr/bin,etc/metricsd/certs,usr/lib/metricsd/plugins,lib/systemd/system,var/lib/metricsd,usr/share/doc/metricsd}

    # Copy binary
    cp "$BUILD_DIR/metricsd-$GOARCH" "$PKG_DIR/usr/bin/metricsd"
    chmod 755 "$PKG_DIR/usr/bin/metricsd"

    # Copy configuration
    cp "$PROJECT_ROOT/config.example.json" "$PKG_DIR/etc/metricsd/config.json"
    cp "$PROJECT_ROOT/config.example.json" "$PKG_DIR/etc/metricsd/config.example.json"

    # Update plugins_dir in config.json to point to installed location
    if command -v jq &> /dev/null; then
        jq '.collector.plugins.plugins_dir = "/usr/lib/metricsd/plugins"' "$PKG_DIR/etc/metricsd/config.json" > "$PKG_DIR/etc/metricsd/config.json.tmp"
        mv "$PKG_DIR/etc/metricsd/config.json.tmp" "$PKG_DIR/etc/metricsd/config.json"
    else
        echo "Warning: jq not found, manually update plugins_dir in config.json"
    fi

    # Copy plugins (executables and .json.example files)
    for plugin in "$PROJECT_ROOT/plugins"/*; do
        if [ -f "$plugin" ]; then
            basename=$(basename "$plugin")
            if [[ "$basename" == *.json ]]; then
                # Rename .json to .json.example
                cp "$plugin" "$PKG_DIR/usr/lib/metricsd/plugins/${basename}.example"
            else
                # Copy executable
                cp "$plugin" "$PKG_DIR/usr/lib/metricsd/plugins/"
                chmod 755 "$PKG_DIR/usr/lib/metricsd/plugins/$basename"
            fi
        fi
    done

    # Copy systemd service
    cp "$PACKAGING_DIR/debian/metricsd.service" "$PKG_DIR/lib/systemd/system/"

    # Copy documentation
    cp "$PROJECT_ROOT/README.md" "$PKG_DIR/usr/share/doc/metricsd/"

    # Generate control file from template
    sed -e "s/{{VERSION}}/$VERSION/g" \
        -e "s/{{ARCH}}/$DEBARCH/g" \
        "$PACKAGING_DIR/debian/control.template" > "$PKG_DIR/DEBIAN/control"

    # Copy maintainer scripts
    cp "$PACKAGING_DIR/debian/postinst" "$PKG_DIR/DEBIAN/"
    cp "$PACKAGING_DIR/debian/prerm" "$PKG_DIR/DEBIAN/"
    cp "$PACKAGING_DIR/debian/postrm" "$PKG_DIR/DEBIAN/"
    cp "$PACKAGING_DIR/debian/conffiles" "$PKG_DIR/DEBIAN/"
    chmod 755 "$PKG_DIR/DEBIAN/postinst" "$PKG_DIR/DEBIAN/prerm" "$PKG_DIR/DEBIAN/postrm"

    # Build package
    echo "Building DEB package..."
    dpkg-deb --build "$PKG_DIR" "$BUILD_DIR/metricsd_${VERSION}_${DEBARCH}.deb"

    echo "✓ Package created: $BUILD_DIR/metricsd_${VERSION}_${DEBARCH}.deb"

    # Cleanup intermediate directory
    rm -rf "$PKG_DIR"
done

echo ""
echo "=== Build complete ==="
ls -lh "$BUILD_DIR"/*.deb 2>/dev/null || echo "No .deb files found"
