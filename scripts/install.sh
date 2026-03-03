#!/usr/bin/env bash
#
# Astrolabe Installation Script
# Supports: Linux, macOS
# Usage: ./install.sh [OPTIONS]
#

set -e  # Exit on error

# Colors for output
if [[ -t 1 ]] && [[ -z "${NO_COLOR:-}" ]] && command -v tput >/dev/null 2>&1; then
    RED=$(tput setaf 1)
    GREEN=$(tput setaf 2)
    YELLOW=$(tput setaf 3)
    BLUE=$(tput setaf 4)
    MAGENTA=$(tput setaf 5)
    CYAN=$(tput setaf 6)
    BOLD=$(tput bold)
    RESET=$(tput sgr0)
else
    RED=""
    GREEN=""
    YELLOW=""
    BLUE=""
    MAGENTA=""
    CYAN=""
    BOLD=""
    RESET=""
fi

# Configuration
BINARY_NAME="astrolabe"
VERSION="0.1.0"
CONFIG_DIR="$HOME/.astrolabe"
DEFAULT_INSTALL_DIR="$HOME/.local/bin"
SYSTEM_INSTALL_DIR="/usr/local/bin"

# Options
SYSTEM_WIDE=false
NO_COMPLETION=false
DRY_RUN=false
INSTALL_DIR=""

# Logging functions
log_info() {
    echo "${BLUE}ℹ${RESET} $*"
}

log_success() {
    echo "${GREEN}✓${RESET} $*"
}

log_warning() {
    echo "${YELLOW}⚠${RESET} $*"
}

log_error() {
    echo "${RED}✗${RESET} $*" >&2
}

log_step() {
    echo "${CYAN}→${RESET} $*"
}

log_header() {
    echo
    echo "${BOLD}$*${RESET}"
    echo
}

# Help message
show_help() {
    cat <<EOF
${BOLD}Astrolabe Installation Script${RESET}

${BOLD}USAGE:${RESET}
    $0 [OPTIONS]

${BOLD}OPTIONS:${RESET}
    --system-wide       Install to /usr/local/bin (requires sudo)
    --no-completion     Skip shell completion installation
    --dry-run           Show what would be done without doing it
    -h, --help          Show this help message

${BOLD}EXAMPLES:${RESET}
    $0                          # Install to ~/.local/bin (recommended)
    $0 --system-wide            # Install to /usr/local/bin
    $0 --dry-run                # Preview installation steps

${BOLD}DESCRIPTION:${RESET}
    This script installs the Astrolabe CLI/TUI tool for data acquisition
    from test benches, devices, and instruments.

    Default installation directory: ~/.local/bin
    Configuration directory: ~/.astrolabe/

EOF
}

# Parse command-line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --system-wide)
                SYSTEM_WIDE=true
                shift
                ;;
            --no-completion)
                NO_COMPLETION=true
                shift
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                echo "Use --help for usage information"
                exit 1
                ;;
        esac
    done

    # Set install directory based on options
    if [[ "$SYSTEM_WIDE" == true ]]; then
        INSTALL_DIR="$SYSTEM_INSTALL_DIR"
    else
        INSTALL_DIR="$DEFAULT_INSTALL_DIR"
    fi
}

# Detect platform and architecture
detect_platform() {
    local os
    os=$(uname -s)

    case "$os" in
        Linux)
            PLATFORM="linux"
            ;;
        Darwin)
            PLATFORM="macos"
            ;;
        *)
            log_error "Unsupported platform: $os"
            log_info "This script supports Linux and macOS only"
            exit 1
            ;;
    esac

    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            log_warning "Unusual architecture detected: $ARCH"
            ;;
    esac

    log_info "Detected platform: $PLATFORM ($ARCH)"
}

# Check if binary exists
check_binary() {
    local binary_path="$1"

    if [[ ! -f "$binary_path" ]]; then
        log_error "Binary not found: $binary_path"
        log_info "Please build the binary first with: mise run build"
        exit 1
    fi

    if [[ ! -x "$binary_path" ]]; then
        log_error "Binary is not executable: $binary_path"
        exit 1
    fi

    log_success "Found binary: $binary_path"
}

# Check installation requirements
check_requirements() {
    log_step "Checking installation requirements..."

    # Check if install directory is writable
    if [[ "$SYSTEM_WIDE" == true ]]; then
        if [[ ! -w "$INSTALL_DIR" ]] && [[ "$DRY_RUN" == false ]]; then
            log_error "No write permission to $INSTALL_DIR"
            log_info "This installation requires sudo. Please run:"
            log_info "  sudo $0 --system-wide"
            exit 1
        fi
    else
        # Create user-local bin directory if it doesn't exist
        if [[ ! -d "$INSTALL_DIR" ]]; then
            log_info "Creating directory: $INSTALL_DIR"
            if [[ "$DRY_RUN" == false ]]; then
                mkdir -p "$INSTALL_DIR"
            fi
        fi
    fi

    log_success "Installation requirements satisfied"
}

# Check if binary is already installed
check_existing_installation() {
    local target="$INSTALL_DIR/$BINARY_NAME"

    if [[ -f "$target" ]]; then
        log_warning "Astrolabe is already installed at: $target"

        if [[ "$DRY_RUN" == false ]]; then
            read -rp "Overwrite existing installation? [y/N] " response
            case "$response" in
                [yY][eE][sS]|[yY])
                    log_info "Will overwrite existing installation"
                    ;;
                *)
                    log_info "Installation cancelled"
                    exit 0
                    ;;
            esac
        fi
    fi
}

# Install binary
install_binary() {
    local source="./astrolabe"
    local target="$INSTALL_DIR/$BINARY_NAME"

    log_step "Installing binary to: $target"

    if [[ "$DRY_RUN" == true ]]; then
        log_info "[DRY RUN] Would copy: $source -> $target"
        log_info "[DRY RUN] Would set permissions: chmod +x $target"
        return
    fi

    cp "$source" "$target"
    chmod +x "$target"

    log_success "Binary installed successfully"
}

# Check PATH configuration
check_path() {
    if [[ "$SYSTEM_WIDE" == true ]]; then
        # System-wide installation, /usr/local/bin should already be in PATH
        return
    fi

    log_step "Checking PATH configuration..."

    if echo "$PATH" | grep -q "$INSTALL_DIR"; then
        log_success "$INSTALL_DIR is in PATH"
        return
    fi

    log_warning "$INSTALL_DIR is NOT in your PATH"
    log_info "Add the following line to your shell configuration file:"
    echo
    echo "  ${CYAN}export PATH=\"\$HOME/.local/bin:\$PATH\"${RESET}"
    echo

    # Detect shell and suggest appropriate RC file
    local shell_name
    shell_name=$(basename "$SHELL")
    local rc_file=""

    case "$shell_name" in
        bash)
            rc_file="$HOME/.bashrc"
            ;;
        zsh)
            rc_file="$HOME/.zshrc"
            ;;
        fish)
            log_info "For fish shell, run: fish_add_path ~/.local/bin"
            return
            ;;
        *)
            log_info "Shell configuration file: ~/.${shell_name}rc"
            return
            ;;
    esac

    if [[ -f "$rc_file" ]] && [[ "$DRY_RUN" == false ]]; then
        read -rp "Add to $rc_file automatically? [y/N] " response
        case "$response" in
            [yY][eE][sS]|[yY])
                echo "" >> "$rc_file"
                echo "# Added by Astrolabe installer" >> "$rc_file"
                echo "export PATH=\"\$HOME/.local/bin:\$PATH\"" >> "$rc_file"
                log_success "Added to $rc_file"
                log_warning "Please restart your shell or run: source $rc_file"
                ;;
            *)
                log_info "Skipped PATH modification"
                ;;
        esac
    fi
}

# Detect user's shell
detect_shell() {
    local shell_name
    shell_name=$(basename "$SHELL")

    case "$shell_name" in
        bash|zsh|fish)
            echo "$shell_name"
            ;;
        *)
            # Try to detect interactively
            if command -v zsh >/dev/null 2>&1; then
                echo "zsh"
            elif command -v bash >/dev/null 2>&1; then
                echo "bash"
            elif command -v fish >/dev/null 2>&1; then
                echo "fish"
            else
                echo ""
            fi
            ;;
    esac
}

# Install shell completion
install_completion() {
    if [[ "$NO_COMPLETION" == true ]]; then
        log_info "Skipping shell completion installation (--no-completion)"
        return
    fi

    log_step "Configuring shell completion..."

    local shell_type
    shell_type=$(detect_shell)

    if [[ -z "$shell_type" ]]; then
        log_warning "Could not detect shell type, skipping completion"
        return
    fi

    log_info "Detected shell: $shell_type"

    if [[ "$DRY_RUN" == true ]]; then
        log_info "[DRY RUN] Would install $shell_type completion"
        return
    fi

    read -rp "Install $shell_type completion? [Y/n] " response
    case "$response" in
        [nN][oO]|[nN])
            log_info "Skipped completion installation"
            return
            ;;
    esac

    case "$shell_type" in
        bash)
            install_bash_completion
            ;;
        zsh)
            install_zsh_completion
            ;;
        fish)
            install_fish_completion
            ;;
    esac
}

# Install bash completion
install_bash_completion() {
    local completion_dir="$HOME/.local/share/bash-completion/completions"
    local completion_file="$completion_dir/$BINARY_NAME"

    mkdir -p "$completion_dir"
    "$INSTALL_DIR/$BINARY_NAME" completion bash > "$completion_file"

    log_success "Bash completion installed to: $completion_file"
    log_info "Restart your shell or run: source $completion_file"
}

# Install zsh completion
install_zsh_completion() {
    local completion_dir="$HOME/.zsh/completion"
    local completion_file="$completion_dir/_$BINARY_NAME"

    mkdir -p "$completion_dir"
    "$INSTALL_DIR/$BINARY_NAME" completion zsh > "$completion_file"

    # Add to fpath if not already there
    local zshrc="$HOME/.zshrc"
    if [[ -f "$zshrc" ]] && ! grep -q "$completion_dir" "$zshrc"; then
        echo "" >> "$zshrc"
        echo "# Added by Astrolabe installer" >> "$zshrc"
        echo "fpath=($completion_dir \$fpath)" >> "$zshrc"
        echo "autoload -Uz compinit && compinit" >> "$zshrc"
    fi

    log_success "Zsh completion installed to: $completion_file"
    log_info "Restart your shell or run: source ~/.zshrc"
}

# Install fish completion
install_fish_completion() {
    local completion_dir="$HOME/.config/fish/completions"
    local completion_file="$completion_dir/$BINARY_NAME.fish"

    mkdir -p "$completion_dir"
    "$INSTALL_DIR/$BINARY_NAME" completion fish > "$completion_file"

    log_success "Fish completion installed to: $completion_file"
    log_info "Fish completion will be available in new shells"
}

# Create config directory
create_config_dir() {
    log_step "Setting up configuration directory..."

    if [[ -d "$CONFIG_DIR" ]]; then
        log_info "Configuration directory already exists: $CONFIG_DIR"
        return
    fi

    if [[ "$DRY_RUN" == true ]]; then
        log_info "[DRY RUN] Would create directory: $CONFIG_DIR"
        return
    fi

    mkdir -p "$CONFIG_DIR"
    chmod 700 "$CONFIG_DIR"  # Secure directory (contains auth tokens)

    log_success "Created configuration directory: $CONFIG_DIR"
}

# Check serial port permissions (Linux only)
check_serial_permissions() {
    if [[ "$PLATFORM" != "linux" ]]; then
        return
    fi

    log_step "Checking serial port permissions..."

    if groups | grep -q dialout; then
        log_success "User is in 'dialout' group (serial port access enabled)"
    else
        log_warning "User is NOT in 'dialout' group"
        log_info "To enable serial port access, run:"
        echo
        echo "  ${CYAN}sudo usermod -a -G dialout \$USER${RESET}"
        echo
        log_info "Then log out and log back in for changes to take effect"
    fi
}

# Validate installation
validate_installation() {
    log_step "Validating installation..."

    if [[ "$DRY_RUN" == true ]]; then
        log_info "[DRY RUN] Would run: $INSTALL_DIR/$BINARY_NAME version"
        log_success "[DRY RUN] Installation validation would succeed"
        return
    fi

    if ! "$INSTALL_DIR/$BINARY_NAME" version >/dev/null 2>&1; then
        log_error "Installation validation failed"
        log_info "Binary exists but does not execute correctly"
        exit 1
    fi

    log_success "Installation validated successfully"
}

# Show post-installation summary
show_summary() {
    log_header "Installation Complete!"

    echo "${GREEN}✓${RESET} Astrolabe v$VERSION installed to: ${CYAN}$INSTALL_DIR/$BINARY_NAME${RESET}"
    echo "${GREEN}✓${RESET} Configuration directory: ${CYAN}$CONFIG_DIR${RESET}"
    echo

    log_info "Next steps:"
    echo
    echo "  1. Verify installation:"
    echo "     ${CYAN}$BINARY_NAME version${RESET}"
    echo
    echo "  2. Launch the TUI (recommended for operators):"
    echo "     ${CYAN}$BINARY_NAME${RESET}"
    echo
    echo "  3. Or use CLI commands directly:"
    echo "     ${CYAN}$BINARY_NAME capture serial /dev/ttyUSB0${RESET}"
    echo "     ${CYAN}$BINARY_NAME capture file data.csv${RESET}"
    echo "     ${CYAN}$BINARY_NAME upload${RESET}"
    echo
    echo "  4. Get help:"
    echo "     ${CYAN}$BINARY_NAME --help${RESET}"
    echo

    if [[ "$PLATFORM" == "linux" ]] && ! groups | grep -q dialout; then
        echo "${YELLOW}⚠${RESET} ${BOLD}Important:${RESET} Don't forget to add your user to the 'dialout' group for serial access!"
        echo
    fi

    log_info "Documentation: https://github.com/cthonicasoftware/astrolabe-cli"
}

# Main installation flow
main() {
    log_header "Astrolabe Installer v$VERSION"

    parse_args "$@"

    if [[ "$DRY_RUN" == true ]]; then
        log_warning "DRY RUN MODE - No changes will be made"
        echo
    fi

    detect_platform
    check_binary "./astrolabe"
    check_requirements
    check_existing_installation
    install_binary
    check_path
    create_config_dir
    install_completion
    check_serial_permissions
    validate_installation
    show_summary

    if [[ "$DRY_RUN" == true ]]; then
        echo
        log_info "Dry run complete. Run without --dry-run to perform actual installation."
    fi
}

# Run main function
main "$@"

