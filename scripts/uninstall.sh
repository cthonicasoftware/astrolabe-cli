#!/usr/bin/env bash
#
# Astrolabe Uninstallation Script
# Supports: Linux, macOS
# Usage: ./uninstall.sh [OPTIONS]
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
CONFIG_DIR="$HOME/.astrolabe"
USER_BIN_DIR="$HOME/.local/bin"
SYSTEM_BIN_DIR="/usr/local/bin"

# Options
COMPLETE_REMOVAL=false
KEEP_CACHE=false
DRY_RUN=false

# Track what was found/removed
FOUND_BINARY=""
REMOVED_ITEMS=()

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
${BOLD}Astrolabe Uninstallation Script${RESET}

${BOLD}USAGE:${RESET}
    $0 [OPTIONS]

${BOLD}OPTIONS:${RESET}
    --complete          Remove everything including cache and configs
    --keep-cache        Remove binary and configs but keep cache
    --dry-run           Show what would be removed without doing it
    -h, --help          Show this help message

${BOLD}EXAMPLES:${RESET}
    $0                          # Interactive mode (ask about each item)
    $0 --complete               # Remove everything
    $0 --keep-cache             # Keep cached run data
    $0 --dry-run                # Preview what would be removed

${BOLD}DESCRIPTION:${RESET}
    This script uninstalls the Astrolabe CLI/TUI tool.

    Interactive mode (default):
      - Removes binary
      - Prompts about shell completion
      - Prompts about cache and config cleanup

    Complete mode (--complete):
      - Removes binary
      - Removes shell completion
      - Removes cache AND configs (WARNING: data loss!)

    Keep cache mode (--keep-cache):
      - Removes binary and configs
      - Preserves cached run data in ~/.astrolabe/runs/

${BOLD}IMPORTANT:${RESET}
    Your cached run data in ~/.astrolabe/runs/ may contain test data
    that has not been uploaded yet. Review before deleting!

EOF
}

# Parse command-line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --complete)
                COMPLETE_REMOVAL=true
                shift
                ;;
            --keep-cache)
                KEEP_CACHE=true
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

    # Validate option combinations
    if [[ "$COMPLETE_REMOVAL" == true ]] && [[ "$KEEP_CACHE" == true ]]; then
        log_error "Cannot use --complete and --keep-cache together"
        exit 1
    fi
}

# Find installed binary
find_binary() {
    log_step "Locating installed binary..."

    if [[ -f "$USER_BIN_DIR/$BINARY_NAME" ]]; then
        FOUND_BINARY="$USER_BIN_DIR/$BINARY_NAME"
        log_success "Found binary: $FOUND_BINARY (user installation)"
    elif [[ -f "$SYSTEM_BIN_DIR/$BINARY_NAME" ]]; then
        FOUND_BINARY="$SYSTEM_BIN_DIR/$BINARY_NAME"
        log_success "Found binary: $FOUND_BINARY (system installation)"
    else
        log_warning "Binary not found in standard locations"
        log_info "Checked: $USER_BIN_DIR and $SYSTEM_BIN_DIR"

        # Check if it's in PATH but elsewhere
        if command -v $BINARY_NAME >/dev/null 2>&1; then
            FOUND_BINARY=$(command -v $BINARY_NAME)
            log_warning "Found binary in PATH: $FOUND_BINARY"
        fi
    fi
}

# Remove binary
remove_binary() {
    if [[ -z "$FOUND_BINARY" ]]; then
        log_warning "No binary to remove"
        return
    fi

    log_step "Removing binary: $FOUND_BINARY"

    if [[ "$DRY_RUN" == true ]]; then
        log_info "[DRY RUN] Would remove: $FOUND_BINARY"
        return
    fi

    # Check if we need sudo
    local needs_sudo=false
    if [[ ! -w "$FOUND_BINARY" ]]; then
        needs_sudo=true
        log_warning "Removing system binary requires sudo"
    fi

    if [[ "$needs_sudo" == true ]]; then
        if ! sudo rm -f "$FOUND_BINARY"; then
            log_error "Failed to remove binary (permission denied)"
            return 1
        fi
    else
        rm -f "$FOUND_BINARY"
    fi

    REMOVED_ITEMS+=("Binary: $FOUND_BINARY")
    log_success "Binary removed"
}

# Find and remove shell completion files
remove_completions() {
    log_step "Checking for shell completion files..."

    local found_completion=false

    # Bash completion
    local bash_locations=(
        "$HOME/.local/share/bash-completion/completions/$BINARY_NAME"
        "/etc/bash_completion.d/$BINARY_NAME"
        "/usr/share/bash-completion/completions/$BINARY_NAME"
    )

    for location in "${bash_locations[@]}"; do
        if [[ -f "$location" ]]; then
            found_completion=true
            if [[ "$DRY_RUN" == true ]]; then
                log_info "[DRY RUN] Would remove bash completion: $location"
            else
                if [[ -w "$location" ]]; then
                    rm -f "$location"
                    REMOVED_ITEMS+=("Bash completion: $location")
                    log_success "Removed bash completion: $location"
                else
                    log_warning "Cannot remove (no permission): $location"
                    log_info "Run with sudo to remove system completions"
                fi
            fi
        fi
    done

    # Zsh completion
    local zsh_locations=(
        "$HOME/.zsh/completion/_$BINARY_NAME"
        "/usr/local/share/zsh/site-functions/_$BINARY_NAME"
    )

    for location in "${zsh_locations[@]}"; do
        if [[ -f "$location" ]]; then
            found_completion=true
            if [[ "$DRY_RUN" == true ]]; then
                log_info "[DRY RUN] Would remove zsh completion: $location"
            else
                if [[ -w "$location" ]]; then
                    rm -f "$location"
                    REMOVED_ITEMS+=("Zsh completion: $location")
                    log_success "Removed zsh completion: $location"
                else
                    log_warning "Cannot remove (no permission): $location"
                fi
            fi
        fi
    done

    # Fish completion
    local fish_locations=(
        "$HOME/.config/fish/completions/$BINARY_NAME.fish"
        "/usr/share/fish/vendor_completions.d/$BINARY_NAME.fish"
    )

    for location in "${fish_locations[@]}"; do
        if [[ -f "$location" ]]; then
            found_completion=true
            if [[ "$DRY_RUN" == true ]]; then
                log_info "[DRY RUN] Would remove fish completion: $location"
            else
                if [[ -w "$location" ]]; then
                    rm -f "$location"
                    REMOVED_ITEMS+=("Fish completion: $location")
                    log_success "Removed fish completion: $location"
                else
                    log_warning "Cannot remove (no permission): $location"
                fi
            fi
        fi
    done

    if [[ "$found_completion" == false ]]; then
        log_info "No shell completion files found"
    fi
}

# Clean up PATH modifications
cleanup_path_modifications() {
    log_step "Checking for PATH modifications..."

    local shell_rc_files=(
        "$HOME/.bashrc"
        "$HOME/.zshrc"
        "$HOME/.profile"
    )

    local found_modification=false

    for rc_file in "${shell_rc_files[@]}"; do
        if [[ -f "$rc_file" ]] && grep -q "Added by Astrolabe installer" "$rc_file" 2>/dev/null; then
            found_modification=true
            log_info "Found PATH modification in: $rc_file"

            if [[ "$DRY_RUN" == true ]]; then
                log_info "[DRY RUN] Would offer to remove PATH modification"
                continue
            fi

            if [[ "$COMPLETE_REMOVAL" == true ]]; then
                response="y"
            else
                read -rp "Remove PATH modification from $rc_file? [y/N] " response
            fi

            case "$response" in
                [yY][eE][sS]|[yY])
                    # Create backup
                    cp "$rc_file" "${rc_file}.bak.$(date +%Y%m%d_%H%M%S)"

                    # Remove the lines added by installer
                    sed -i.tmp '/# Added by Astrolabe installer/,+1d' "$rc_file"
                    rm -f "${rc_file}.tmp"

                    REMOVED_ITEMS+=("PATH modification: $rc_file")
                    log_success "Removed PATH modification (backup created)"
                    log_info "Restart your shell for changes to take effect"
                    ;;
                *)
                    log_info "Kept PATH modification in $rc_file"
                    ;;
            esac
        fi
    done

    # Check zsh fpath modification
    if [[ -f "$HOME/.zshrc" ]] && grep -q "fpath=.*\.zsh/completion" "$HOME/.zshrc" 2>/dev/null; then
        found_modification=true
        log_info "Found fpath modification in: $HOME/.zshrc"

        if [[ "$DRY_RUN" == false ]]; then
            if [[ "$COMPLETE_REMOVAL" == true ]]; then
                response="y"
            else
                read -rp "Remove fpath modification from ~/.zshrc? [y/N] " response
            fi

            case "$response" in
                [yY][eE][sS]|[yY])
                    cp "$HOME/.zshrc" "$HOME/.zshrc.bak.$(date +%Y%m%d_%H%M%S)"
                    sed -i.tmp '/# Added by Astrolabe installer/,+2d' "$HOME/.zshrc"
                    rm -f "$HOME/.zshrc.tmp"
                    REMOVED_ITEMS+=("fpath modification: ~/.zshrc")
                    log_success "Removed fpath modification (backup created)"
                    ;;
                *)
                    log_info "Kept fpath modification in ~/.zshrc"
                    ;;
            esac
        fi
    fi

    if [[ "$found_modification" == false ]]; then
        log_info "No PATH modifications found"
    fi
}

# Handle config and cache cleanup
cleanup_config_and_cache() {
    if [[ ! -d "$CONFIG_DIR" ]]; then
        log_info "Configuration directory not found: $CONFIG_DIR"
        return
    fi

    log_step "Checking configuration and cache..."

    # Check what exists
    local has_config=false
    local has_cache=false
    local cache_runs_count=0

    if [[ -f "$CONFIG_DIR/connection.yml" ]] || [[ -f "$CONFIG_DIR/metadata.json" ]]; then
        has_config=true
    fi

    if [[ -d "$CONFIG_DIR/runs" ]]; then
        has_cache=true
        cache_runs_count=$(find "$CONFIG_DIR/runs" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l)
    fi

    log_info "Configuration directory: $CONFIG_DIR"
    if [[ "$has_config" == true ]]; then
        log_info "  - Configuration files found"
    fi
    if [[ "$has_cache" == true ]]; then
        log_info "  - Cache contains $cache_runs_count run(s)"
    fi

    # Determine cleanup strategy
    if [[ "$COMPLETE_REMOVAL" == true ]]; then
        # Remove everything
        if [[ "$has_cache" == true ]] && [[ "$cache_runs_count" -gt 0 ]]; then
            log_warning "This will delete $cache_runs_count cached run(s)!"
            log_warning "Any unuploaded test data will be lost!"

            if [[ "$DRY_RUN" == false ]]; then
                read -rp "Are you SURE you want to delete all cached data? [y/N] " response
                case "$response" in
                    [yY][eE][sS]|[yY])
                        rm -rf "$CONFIG_DIR"
                        REMOVED_ITEMS+=("Complete removal: $CONFIG_DIR")
                        log_success "Removed configuration and cache"
                        return
                        ;;
                    *)
                        log_info "Keeping configuration and cache"
                        return
                        ;;
                esac
            else
                log_info "[DRY RUN] Would remove entire directory: $CONFIG_DIR"
                return
            fi
        else
            if [[ "$DRY_RUN" == true ]]; then
                log_info "[DRY RUN] Would remove: $CONFIG_DIR"
            else
                rm -rf "$CONFIG_DIR"
                REMOVED_ITEMS+=("Configuration directory: $CONFIG_DIR")
                log_success "Removed configuration directory"
            fi
        fi
    elif [[ "$KEEP_CACHE" == true ]]; then
        # Remove configs but keep cache
        if [[ "$DRY_RUN" == true ]]; then
            log_info "[DRY RUN] Would remove config files, keep cache"
        else
            rm -f "$CONFIG_DIR/connection.yml"
            rm -f "$CONFIG_DIR/metadata.json"
            REMOVED_ITEMS+=("Config files (cache preserved)")
            log_success "Removed config files, kept cache"
        fi
    else
        # Interactive mode - ask user
        echo
        log_info "What would you like to do with configuration and cache?"
        echo
        echo "  1) Keep everything (configs + cache)"
        echo "  2) Remove configs only, keep cache"
        echo "  3) Remove everything (configs + cache) ${RED}[WARNING: data loss!]${RESET}"
        echo

        if [[ "$DRY_RUN" == true ]]; then
            log_info "[DRY RUN] Would prompt for cleanup option"
            return
        fi

        read -rp "Choose [1/2/3]: " choice

        case "$choice" in
            1)
                log_info "Keeping configuration and cache"
                ;;
            2)
                rm -f "$CONFIG_DIR/connection.yml"
                rm -f "$CONFIG_DIR/metadata.json"
                REMOVED_ITEMS+=("Config files (cache preserved)")
                log_success "Removed config files, kept cache"
                ;;
            3)
                if [[ "$has_cache" == true ]] && [[ "$cache_runs_count" -gt 0 ]]; then
                    log_warning "This will delete $cache_runs_count cached run(s)!"
                    read -rp "Are you SURE? Type 'delete' to confirm: " confirmation
                    if [[ "$confirmation" == "delete" ]]; then
                        rm -rf "$CONFIG_DIR"
                        REMOVED_ITEMS+=("Complete removal: $CONFIG_DIR")
                        log_success "Removed configuration and cache"
                    else
                        log_info "Cancelled - keeping configuration and cache"
                    fi
                else
                    rm -rf "$CONFIG_DIR"
                    REMOVED_ITEMS+=("Configuration directory: $CONFIG_DIR")
                    log_success "Removed configuration directory"
                fi
                ;;
            *)
                log_info "Invalid choice - keeping configuration and cache"
                ;;
        esac
    fi
}

# Show summary of what was removed
show_summary() {
    log_header "Uninstallation Summary"

    if [[ ${#REMOVED_ITEMS[@]} -eq 0 ]]; then
        log_info "No items were removed"
        return
    fi

    echo "${BOLD}Removed items:${RESET}"
    echo
    for item in "${REMOVED_ITEMS[@]}"; do
        echo "  ${GREEN}✓${RESET} $item"
    done
    echo

    if [[ "$DRY_RUN" == true ]]; then
        log_info "This was a dry run - no changes were made"
        log_info "Run without --dry-run to perform actual uninstallation"
    else
        log_success "Astrolabe has been uninstalled"

        # Check if binary is still in PATH
        if command -v $BINARY_NAME >/dev/null 2>&1; then
            log_warning "Binary is still accessible via PATH"
            log_info "You may need to restart your shell or manually remove from PATH"
        fi
    fi
}

# Main uninstallation flow
main() {
    log_header "Astrolabe Uninstaller"

    parse_args "$@"

    if [[ "$DRY_RUN" == true ]]; then
        log_warning "DRY RUN MODE - No changes will be made"
        echo
    fi

    find_binary
    remove_binary
    remove_completions
    cleanup_path_modifications
    cleanup_config_and_cache
    show_summary

    echo
    log_info "Thank you for using Astrolabe!"
}

# Run main function
main "$@"
