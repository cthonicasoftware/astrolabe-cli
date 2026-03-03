<#
.SYNOPSIS
    Astrolabe Installation Script for Windows

.DESCRIPTION
    Installs the Astrolabe CLI/TUI tool on Windows systems.
    Supports user-local and system-wide installation.

.PARAMETER SystemWide
    Install to C:\Program Files (requires Administrator)

.PARAMETER NoCompletion
    Skip PowerShell completion installation

.PARAMETER DryRun
    Preview installation without making changes

.PARAMETER Help
    Show help message

.EXAMPLE
    .\install.ps1
    Install to user-local directory (default)

.EXAMPLE
    .\install.ps1 -SystemWide
    Install to Program Files (requires admin)

.EXAMPLE
    .\install.ps1 -DryRun
    Preview installation steps

.NOTES
    Version: 0.1.0
    Requires: PowerShell 5.1 or later
#>

[CmdletBinding(SupportsShouldProcess)]
param(
    [switch]$SystemWide,
    [switch]$NoCompletion,
    [switch]$DryRun,
    [switch]$Help
)

# Configuration
$Script:BinaryName = "astrolabe.exe"
$Script:Version = "0.1.0"
$Script:ConfigDir = Join-Path $env:USERPROFILE ".astrolabe"
$Script:UserInstallDir = Join-Path $env:LOCALAPPDATA "Programs\Astrolabe"
$Script:SystemInstallDir = Join-Path ${env:ProgramFiles} "Astrolabe"
$Script:InstallDir = ""
$Script:CompletionScript = Join-Path $env:USERPROFILE ".astrolabe\astrolabe-completion.ps1"

# Color scheme
$Script:Colors = @{
    Info    = "Cyan"
    Success = "Green"
    Warning = "Yellow"
    Error   = "Red"
    Header  = "White"
}

#region Helper Functions

function Write-ColorOutput {
    param(
        [Parameter(Mandatory)]
        [string]$Message,

        [Parameter(Mandatory)]
        [ValidateSet("Info", "Success", "Warning", "Error", "Header", "Step")]
        [string]$Level
    )

    # Use simple ASCII characters that work across all terminals
    $icon = switch ($Level) {
        "Info"    { "[i]" }
        "Success" { "[+]" }
        "Warning" { "[!]" }
        "Error"   { "[x]" }
        "Step"    { ">>>" }
        "Header"  { "" }
    }

    $color = switch ($Level) {
        "Info"    { $Colors.Info }
        "Success" { $Colors.Success }
        "Warning" { $Colors.Warning }
        "Error"   { $Colors.Error }
        "Step"    { $Colors.Info }
        "Header"  { $Colors.Header }
    }

    if ($Level -eq "Header") {
        Write-Host "`n$Message`n" -ForegroundColor $color
    } else {
        Write-Host "$icon $Message" -ForegroundColor $color
    }
}

function Test-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Add-ToPath {
    param(
        [string]$Directory,
        [ValidateSet("User", "Machine")]
        [string]$Scope = "User"
    )

    if ($DryRun) {
        Write-ColorOutput "[DRY RUN] Would add to $Scope PATH: $Directory" -Level Info
        return
    }

    $currentPath = [Environment]::GetEnvironmentVariable("Path", $Scope)

    # Check if already in PATH
    if ($currentPath -split ';' | Where-Object { $_ -eq $Directory }) {
        Write-ColorOutput "Already in PATH: $Directory" -Level Info
        return
    }

    $newPath = if ($currentPath) {
        "$currentPath;$Directory"
    } else {
        $Directory
    }

    [Environment]::SetEnvironmentVariable("Path", $newPath, $Scope)

    # Update current session
    $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" +
                [System.Environment]::GetEnvironmentVariable("Path", "User")
}

function Test-PathContains {
    param([string]$Directory)

    $machinePath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $allPaths = ($machinePath + ";" + $userPath) -split ';'

    return $allPaths -contains $Directory
}

#endregion

#region Main Functions

function Show-Help {
    Get-Help $PSCommandPath -Detailed
}

function Test-Prerequisites {
    Write-ColorOutput "Checking prerequisites..." -Level Step

    # Check PowerShell version
    $psVersion = $PSVersionTable.PSVersion
    if ($psVersion.Major -lt 5) {
        Write-ColorOutput "PowerShell 5.1 or later required (found $psVersion)" -Level Error
        exit 1
    }
    Write-ColorOutput "PowerShell version: $psVersion" -Level Info

    # Check Windows version
    $os = Get-CimInstance Win32_OperatingSystem
    Write-ColorOutput "Windows version: $($os.Caption) (Build $($os.BuildNumber))" -Level Info

    # Check architecture
    $arch = $env:PROCESSOR_ARCHITECTURE
    Write-ColorOutput "Architecture: $arch" -Level Info

    if ($arch -ne "AMD64" -and $arch -ne "x86_64") {
        Write-ColorOutput "Warning: Unusual architecture detected. Expected AMD64/x86_64." -Level Warning
    }

    Write-ColorOutput "Prerequisites satisfied" -Level Success
}

function Test-Binary {
    param([string]$BinaryPath)

    Write-ColorOutput "Checking for binary..." -Level Step

    if (-not (Test-Path $BinaryPath)) {
        Write-ColorOutput "Binary not found: $BinaryPath" -Level Error
        Write-ColorOutput "Please build the binary first:" -Level Info
        Write-Host "  mise run build" -ForegroundColor Cyan
        Write-Host "  OR" -ForegroundColor Cyan
        Write-Host "  go build -o astrolabe.exe ./cmd/astrolabe" -ForegroundColor Cyan
        exit 1
    }

    Write-ColorOutput "Found binary: $BinaryPath" -Level Success
}

function Test-AdminRights {
    if ($SystemWide) {
        if (-not (Test-Administrator)) {
            Write-ColorOutput "System-wide installation requires Administrator rights" -Level Error
            Write-ColorOutput "Please run PowerShell as Administrator, or omit -SystemWide flag" -Level Info
            exit 1
        }
        Write-ColorOutput "Running as Administrator" -Level Success
    }
}

function Test-ExistingInstallation {
    $target = Join-Path $InstallDir $BinaryName

    if (Test-Path $target) {
        Write-ColorOutput "Astrolabe is already installed at: $target" -Level Warning

        if (-not $DryRun) {
            $response = Read-Host "Overwrite existing installation? [y/N]"
            if ($response -notmatch '^[yY]') {
                Write-ColorOutput "Installation cancelled" -Level Info
                exit 0
            }
        }
    }
}

function Install-Binary {
    Write-ColorOutput "Installing binary to: $InstallDir" -Level Step

    $source = Join-Path $PSScriptRoot "..\$BinaryName"
    if (-not (Test-Path $source)) {
        $source = Join-Path (Get-Location) $BinaryName
    }

    $target = Join-Path $InstallDir $BinaryName

    if ($DryRun) {
        Write-ColorOutput "[DRY RUN] Would create directory: $InstallDir" -Level Info
        Write-ColorOutput "[DRY RUN] Would copy: $source -> $target" -Level Info
        return
    }

    # Create installation directory
    if (-not (Test-Path $InstallDir)) {
        New-Item -Path $InstallDir -ItemType Directory -Force | Out-Null
    }

    # Copy binary
    Copy-Item -Path $source -Destination $target -Force

    Write-ColorOutput "Binary installed successfully" -Level Success
}

function Update-PathVariable {
    Write-ColorOutput "Updating PATH..." -Level Step

    if (Test-PathContains $InstallDir) {
        Write-ColorOutput "$InstallDir is already in PATH" -Level Success
        return
    }

    $scope = if ($SystemWide) { "Machine" } else { "User" }
    Add-ToPath -Directory $InstallDir -Scope $scope

    if (-not $DryRun) {
        Write-ColorOutput "Added to PATH ($scope)" -Level Success
        Write-ColorOutput "Restart your terminal for PATH changes to take effect" -Level Warning
    }
}

function Install-Completion {
    if ($NoCompletion) {
        Write-ColorOutput "Skipping PowerShell completion installation (-NoCompletion)" -Level Info
        return
    }

    Write-ColorOutput "Configuring PowerShell completion..." -Level Step

    if ($DryRun) {
        Write-ColorOutput "[DRY RUN] Would generate completion script to: $CompletionScript" -Level Info
        Write-ColorOutput "[DRY RUN] Would modify PowerShell profile: $PROFILE" -Level Info
        return
    }

    $response = Read-Host "Install PowerShell completion? [Y/n]"
    if ($response -match '^[nN]') {
        Write-ColorOutput "Skipped completion installation" -Level Info
        return
    }

    try {
        # Generate completion script
        $binaryPath = Join-Path $InstallDir $BinaryName
        & $binaryPath completion powershell | Out-File -FilePath $CompletionScript -Encoding UTF8

        # Create profile if it doesn't exist
        if (-not (Test-Path $PROFILE)) {
            New-Item -Path $PROFILE -ItemType File -Force | Out-Null
        }

        # Check if already added
        $profileContent = Get-Content $PROFILE -Raw -ErrorAction SilentlyContinue
        if ($profileContent -notmatch "astrolabe-completion\.ps1") {
            # Add to profile
            Add-Content -Path $PROFILE -Value @"

# Astrolabe completion (added by installer)
if (Test-Path "$CompletionScript") {
    . "$CompletionScript"
}
"@
            Write-ColorOutput "PowerShell completion installed" -Level Success
            Write-ColorOutput "Restart PowerShell for completion to take effect" -Level Info
        } else {
            Write-ColorOutput "Completion already configured in profile" -Level Info
        }
    }
    catch {
        Write-ColorOutput "Failed to install completion: $_" -Level Warning
        Write-ColorOutput "You can add completion manually later" -Level Info
    }
}

function New-ConfigDirectory {
    Write-ColorOutput "Setting up configuration directory..." -Level Step

    if (Test-Path $ConfigDir) {
        Write-ColorOutput "Configuration directory already exists: $ConfigDir" -Level Info
        return
    }

    if ($DryRun) {
        Write-ColorOutput "[DRY RUN] Would create directory: $ConfigDir" -Level Info
        return
    }

    New-Item -Path $ConfigDir -ItemType Directory -Force | Out-Null
    Write-ColorOutput "Created configuration directory: $ConfigDir" -Level Success
}

function Show-SerialPortInfo {
    Write-ColorOutput "Checking serial ports..." -Level Step

    try {
        $ports = Get-CimInstance -ClassName Win32_PnPEntity |
                 Where-Object { $_.Caption -match "COM\d+" } |
                 Select-Object Caption, DeviceID

        if ($ports) {
            Write-ColorOutput "Available serial ports:" -Level Info
            foreach ($port in $ports) {
                Write-Host "  - $($port.Caption)" -ForegroundColor Gray
            }
        } else {
            Write-ColorOutput "No serial ports detected" -Level Info
            Write-ColorOutput "If you need serial port access, ensure drivers are installed" -Level Info
        }
    }
    catch {
        Write-ColorOutput "Could not enumerate serial ports" -Level Warning
    }
}

function Test-Installation {
    Write-ColorOutput "Validating installation..." -Level Step

    if ($DryRun) {
        Write-ColorOutput "[DRY RUN] Would run: $BinaryName version" -Level Info
        Write-ColorOutput "[DRY RUN] Installation validation would succeed" -Level Success
        return
    }

    $binaryPath = Join-Path $InstallDir $BinaryName

    try {
        # Test execution
        $output = & $binaryPath version 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-ColorOutput "Installation validated successfully" -Level Success
        } else {
            Write-ColorOutput "Binary exists but execution failed" -Level Error
            Write-ColorOutput "Output: $output" -Level Info
            exit 1
        }
    }
    catch {
        Write-ColorOutput "Installation validation failed: $_" -Level Error
        exit 1
    }
}

function Show-Summary {
    Write-ColorOutput "Installation Complete!" -Level Header

    Write-Host "[+] Astrolabe v$Version installed to: " -ForegroundColor Green -NoNewline
    Write-Host $InstallDir -ForegroundColor Cyan

    Write-Host "[+] Configuration directory: " -ForegroundColor Green -NoNewline
    Write-Host $ConfigDir -ForegroundColor Cyan

    Write-Host ""
    Write-ColorOutput "Next steps:" -Level Info
    Write-Host ""
    Write-Host "  1. Restart your terminal to update PATH" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "  2. Verify installation:" -ForegroundColor White
    Write-Host "     astrolabe version" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  3. Launch the TUI (recommended for operators):" -ForegroundColor White
    Write-Host "     astrolabe" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  4. Or use CLI commands directly:" -ForegroundColor White
    Write-Host "     astrolabe capture serial COM1" -ForegroundColor Cyan
    Write-Host "     astrolabe capture file data.csv" -ForegroundColor Cyan
    Write-Host "     astrolabe upload" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  5. Get help:" -ForegroundColor White
    Write-Host "     astrolabe --help" -ForegroundColor Cyan
    Write-Host ""
    Write-ColorOutput "Documentation: https://github.com/cthonicasoftware/astrolabe-cli" -Level Info
}

#endregion

#region Main Execution

function Main {
    # Parse arguments
    if ($Help) {
        Show-Help
        exit 0
    }

    # Set installation directory
    $Script:InstallDir = if ($SystemWide) { $SystemInstallDir } else { $UserInstallDir }

    # Show header
    Write-ColorOutput "Astrolabe Installer v$Version" -Level Header

    if ($DryRun) {
        Write-ColorOutput "DRY RUN MODE - No changes will be made" -Level Warning
        Write-Host ""
    }

    # Run installation steps
    Test-Prerequisites
    Test-Binary (Join-Path (Get-Location) $BinaryName)
    Test-AdminRights
    Test-ExistingInstallation
    Install-Binary
    Update-PathVariable
    New-ConfigDirectory
    Install-Completion
    Show-SerialPortInfo
    Test-Installation
    Show-Summary

    if ($DryRun) {
        Write-Host ""
        Write-ColorOutput "Dry run complete. Run without -DryRun to perform actual installation." -Level Info
    }
}

# Run main function
try {
    Main
}
catch {
    Write-ColorOutput "Installation failed: $_" -Level Error
    Write-ColorOutput $_.ScriptStackTrace -Level Error
    exit 1
}

#endregion

