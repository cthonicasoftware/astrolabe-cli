<#
.SYNOPSIS
    Astrolabe Uninstallation Script for Windows

.DESCRIPTION
    Uninstalls the Astrolabe CLI/TUI tool from Windows systems.
    Provides options for cache and configuration cleanup.

.PARAMETER Complete
    Remove everything including cache and configs

.PARAMETER KeepCache
    Remove binary and configs but keep cached run data

.PARAMETER DryRun
    Preview uninstallation without making changes

.PARAMETER Help
    Show help message

.EXAMPLE
    .\uninstall.ps1
    Interactive mode (asks about each item)

.EXAMPLE
    .\uninstall.ps1 -Complete
    Complete removal including cache

.EXAMPLE
    .\uninstall.ps1 -KeepCache
    Remove binary and configs, keep cached data

.EXAMPLE
    .\uninstall.ps1 -DryRun
    Preview what would be removed

.NOTES
    Version: 0.1.0
    Requires: PowerShell 5.1 or later
#>

[CmdletBinding(SupportsShouldProcess)]
param(
    [switch]$Complete,
    [switch]$KeepCache,
    [switch]$DryRun,
    [switch]$Help
)

# Configuration
$Script:BinaryName = "astrolabe.exe"
$Script:ConfigDir = Join-Path $env:USERPROFILE ".astrolabe"
$Script:UserInstallDir = Join-Path $env:LOCALAPPDATA "Programs\Astrolabe"
$Script:SystemInstallDir = Join-Path ${env:ProgramFiles} "Astrolabe"
$Script:CompletionScript = Join-Path $ConfigDir "astrolabe-completion.ps1"
$Script:FoundBinary = $null
$Script:RemovedItems = @()

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

    $icon = switch ($Level) {
        "Info"    { "ℹ" }
        "Success" { "✓" }
        "Warning" { "⚠" }
        "Error"   { "✗" }
        "Step"    { "→" }
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

function Remove-FromPath {
    param(
        [string]$Directory,
        [ValidateSet("User", "Machine")]
        [string]$Scope = "User"
    )

    if ($DryRun) {
        Write-ColorOutput "[DRY RUN] Would remove from $Scope PATH: $Directory" -Level Info
        return $false
    }

    $currentPath = [Environment]::GetEnvironmentVariable("Path", $Scope)
    $pathArray = $currentPath -split ';' | Where-Object { $_ -ne $Directory -and $_ -ne "" }
    $newPath = $pathArray -join ';'

    if ($currentPath -ne $newPath) {
        [Environment]::SetEnvironmentVariable("Path", $newPath, $Scope)

        # Update current session
        $env:Path = [System.Environment]::GetEnvironmentVariable("Path", "Machine") + ";" +
                    [System.Environment]::GetEnvironmentVariable("Path", "User")
        return $true
    }

    return $false
}

#endregion

#region Main Functions

function Show-Help {
    Get-Help $PSCommandPath -Detailed
}

function Find-Installation {
    Write-ColorOutput "Locating installed binary..." -Level Step

    $locations = @(
        @{ Path = $UserInstallDir; Type = "user" },
        @{ Path = $SystemInstallDir; Type = "system" }
    )

    foreach ($location in $locations) {
        $binaryPath = Join-Path $location.Path $BinaryName
        if (Test-Path $binaryPath) {
            $Script:FoundBinary = @{
                Path = $binaryPath
                Dir  = $location.Path
                Type = $location.Type
            }
            Write-ColorOutput "Found binary: $binaryPath ($($location.Type) installation)" -Level Success
            return
        }
    }

    # Check if it's in PATH but elsewhere
    $inPath = Get-Command $BinaryName -ErrorAction SilentlyContinue
    if ($inPath) {
        $Script:FoundBinary = @{
            Path = $inPath.Source
            Dir  = Split-Path $inPath.Source -Parent
            Type = "custom"
        }
        Write-ColorOutput "Found binary in PATH: $($inPath.Source)" -Level Warning
        return
    }

    Write-ColorOutput "Binary not found in standard locations" -Level Warning
    Write-ColorOutput "Checked: $UserInstallDir and $SystemInstallDir" -Level Info
}

function Remove-Binary {
    if (-not $FoundBinary) {
        Write-ColorOutput "No binary to remove" -Level Warning
        return
    }

    Write-ColorOutput "Removing binary: $($FoundBinary.Path)" -Level Step

    # Check if we need admin rights
    if ($FoundBinary.Type -eq "system" -and -not (Test-Administrator)) {
        Write-ColorOutput "Removing system installation requires Administrator rights" -Level Error
        Write-ColorOutput "Please run PowerShell as Administrator" -Level Info
        exit 1
    }

    if ($DryRun) {
        Write-ColorOutput "[DRY RUN] Would remove: $($FoundBinary.Path)" -Level Info
        Write-ColorOutput "[DRY RUN] Would remove directory: $($FoundBinary.Dir)" -Level Info
        return
    }

    try {
        # Remove binary
        Remove-Item -Path $FoundBinary.Path -Force -ErrorAction Stop

        # Remove directory if empty or only contains completion script
        $dirContents = Get-ChildItem -Path $FoundBinary.Dir -ErrorAction SilentlyContinue
        if (-not $dirContents -or $dirContents.Count -eq 0) {
            Remove-Item -Path $FoundBinary.Dir -Force -Recurse -ErrorAction Stop
        }

        $Script:RemovedItems += "Binary: $($FoundBinary.Path)"
        Write-ColorOutput "Binary removed" -Level Success
    }
    catch {
        Write-ColorOutput "Failed to remove binary: $_" -Level Error
    }
}

function Remove-PathEntries {
    Write-ColorOutput "Checking PATH entries..." -Level Step

    $pathsToRemove = @($UserInstallDir, $SystemInstallDir)
    $removedAny = $false

    foreach ($path in $pathsToRemove) {
        # Check User PATH
        $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if ($userPath -split ';' | Where-Object { $_ -eq $path }) {
            if ($DryRun) {
                Write-ColorOutput "[DRY RUN] Would remove from User PATH: $path" -Level Info
            } else {
                if (Remove-FromPath -Directory $path -Scope "User") {
                    $Script:RemovedItems += "User PATH entry: $path"
                    Write-ColorOutput "Removed from User PATH: $path" -Level Success
                    $removedAny = $true
                }
            }
        }

        # Check Machine PATH (requires admin)
        $machinePath = [Environment]::GetEnvironmentVariable("Path", "Machine")
        if ($machinePath -split ';' | Where-Object { $_ -eq $path }) {
            if (-not (Test-Administrator)) {
                Write-ColorOutput "Cannot remove from System PATH (requires Administrator)" -Level Warning
                Write-ColorOutput "Run as Administrator to complete removal" -Level Info
            } else {
                if ($DryRun) {
                    Write-ColorOutput "[DRY RUN] Would remove from System PATH: $path" -Level Info
                } else {
                    if (Remove-FromPath -Directory $path -Scope "Machine") {
                        $Script:RemovedItems += "System PATH entry: $path"
                        Write-ColorOutput "Removed from System PATH: $path" -Level Success
                        $removedAny = $true
                    }
                }
            }
        }
    }

    if (-not $removedAny -and -not $DryRun) {
        Write-ColorOutput "No PATH entries found to remove" -Level Info
    }
}

function Remove-Completion {
    Write-ColorOutput "Checking PowerShell completion..." -Level Step

    $completionRemoved = $false

    # Remove completion script
    if (Test-Path $CompletionScript) {
        if ($DryRun) {
            Write-ColorOutput "[DRY RUN] Would remove completion script: $CompletionScript" -Level Info
        } else {
            Remove-Item -Path $CompletionScript -Force -ErrorAction SilentlyContinue
            $Script:RemovedItems += "Completion script: $CompletionScript"
            Write-ColorOutput "Removed completion script" -Level Success
            $completionRemoved = $true
        }
    }

    # Remove from PowerShell profile
    if (Test-Path $PROFILE) {
        $profileContent = Get-Content $PROFILE -Raw -ErrorAction SilentlyContinue

        if ($profileContent -match "astrolabe-completion\.ps1|Astrolabe completion") {
            if ($DryRun) {
                Write-ColorOutput "[DRY RUN] Would remove completion from profile: $PROFILE" -Level Info
            } else {
                $response = Read-Host "Remove completion from PowerShell profile? [y/N]"
                if ($response -match '^[yY]') {
                    # Backup profile
                    $backupPath = "$PROFILE.bak.$(Get-Date -Format 'yyyyMMdd_HHmmss')"
                    Copy-Item -Path $PROFILE -Destination $backupPath -Force

                    # Remove completion lines
                    $lines = Get-Content $PROFILE
                    $newLines = @()
                    $skipNext = 0

                    for ($i = 0; $i -lt $lines.Count; $i++) {
                        if ($skipNext -gt 0) {
                            $skipNext--
                            continue
                        }

                        if ($lines[$i] -match "# Astrolabe completion") {
                            # Skip this line and the next 3 lines (comment + if statement + . source + closing brace)
                            $skipNext = 3
                            continue
                        }

                        $newLines += $lines[$i]
                    }

                    $newLines | Out-File -FilePath $PROFILE -Encoding UTF8
                    $Script:RemovedItems += "PowerShell profile completion (backup: $backupPath)"
                    Write-ColorOutput "Removed completion from profile (backup created)" -Level Success
                    $completionRemoved = $true
                } else {
                    Write-ColorOutput "Kept completion in profile" -Level Info
                }
            }
        }
    }

    if (-not $completionRemoved -and -not $DryRun) {
        Write-ColorOutput "No completion configuration found" -Level Info
    }
}

function Remove-ConfigAndCache {
    if (-not (Test-Path $ConfigDir)) {
        Write-ColorOutput "Configuration directory not found: $ConfigDir" -Level Info
        return
    }

    Write-ColorOutput "Checking configuration and cache..." -Level Step

    # Analyze what exists
    $hasConfig = $false
    $hasCache = $false
    $cacheRunsCount = 0

    $configFiles = @(
        (Join-Path $ConfigDir "connection.yml"),
        (Join-Path $ConfigDir "metadata.json")
    )

    foreach ($file in $configFiles) {
        if (Test-Path $file) {
            $hasConfig = $true
            break
        }
    }

    $cacheDir = Join-Path $ConfigDir "runs"
    if (Test-Path $cacheDir) {
        $hasCache = $true
        $cacheRunsCount = (Get-ChildItem -Path $cacheDir -Directory -ErrorAction SilentlyContinue).Count
    }

    Write-ColorOutput "Configuration directory: $ConfigDir" -Level Info
    if ($hasConfig) {
        Write-Host "  - Configuration files found" -ForegroundColor Gray
    }
    if ($hasCache) {
        Write-Host "  - Cache contains $cacheRunsCount run(s)" -ForegroundColor Gray
    }

    # Determine cleanup strategy
    if ($Complete) {
        # Complete removal
        if ($hasCache -and $cacheRunsCount -gt 0) {
            Write-ColorOutput "This will delete $cacheRunsCount cached run(s)!" -Level Warning
            Write-ColorOutput "Any unuploaded test data will be lost!" -Level Warning

            if ($DryRun) {
                Write-ColorOutput "[DRY RUN] Would remove entire directory: $ConfigDir" -Level Info
            } else {
                $response = Read-Host "Are you SURE you want to delete all cached data? [y/N]"
                if ($response -match '^[yY]') {
                    Remove-Item -Path $ConfigDir -Recurse -Force -ErrorAction Stop
                    $Script:RemovedItems += "Complete removal: $ConfigDir"
                    Write-ColorOutput "Removed configuration and cache" -Level Success
                } else {
                    Write-ColorOutput "Keeping configuration and cache" -Level Info
                }
            }
        } else {
            if ($DryRun) {
                Write-ColorOutput "[DRY RUN] Would remove: $ConfigDir" -Level Info
            } else {
                Remove-Item -Path $ConfigDir -Recurse -Force -ErrorAction Stop
                $Script:RemovedItems += "Configuration directory: $ConfigDir"
                Write-ColorOutput "Removed configuration directory" -Level Success
            }
        }
    }
    elseif ($KeepCache) {
        # Remove configs but keep cache
        if ($DryRun) {
            Write-ColorOutput "[DRY RUN] Would remove config files, keep cache" -Level Info
        } else {
            foreach ($file in $configFiles) {
                if (Test-Path $file) {
                    Remove-Item -Path $file -Force -ErrorAction SilentlyContinue
                }
            }
            $Script:RemovedItems += "Config files (cache preserved)"
            Write-ColorOutput "Removed config files, kept cache" -Level Success
        }
    }
    else {
        # Interactive mode
        Write-Host ""
        Write-ColorOutput "What would you like to do with configuration and cache?" -Level Info
        Write-Host ""
        Write-Host "  1) Keep everything (configs + cache)" -ForegroundColor White
        Write-Host "  2) Remove configs only, keep cache" -ForegroundColor White
        Write-Host "  3) Remove everything (configs + cache) " -ForegroundColor White -NoNewline
        Write-Host "[WARNING: data loss!]" -ForegroundColor Red
        Write-Host ""

        if ($DryRun) {
            Write-ColorOutput "[DRY RUN] Would prompt for cleanup option" -Level Info
            return
        }

        $choice = Read-Host "Choose [1/2/3]"

        switch ($choice) {
            "1" {
                Write-ColorOutput "Keeping configuration and cache" -Level Info
            }
            "2" {
                foreach ($file in $configFiles) {
                    if (Test-Path $file) {
                        Remove-Item -Path $file -Force -ErrorAction SilentlyContinue
                    }
                }
                $Script:RemovedItems += "Config files (cache preserved)"
                Write-ColorOutput "Removed config files, kept cache" -Level Success
            }
            "3" {
                if ($hasCache -and $cacheRunsCount -gt 0) {
                    Write-ColorOutput "This will delete $cacheRunsCount cached run(s)!" -Level Warning
                    $confirmation = Read-Host "Are you SURE? Type 'delete' to confirm"
                    if ($confirmation -eq "delete") {
                        Remove-Item -Path $ConfigDir -Recurse -Force -ErrorAction Stop
                        $Script:RemovedItems += "Complete removal: $ConfigDir"
                        Write-ColorOutput "Removed configuration and cache" -Level Success
                    } else {
                        Write-ColorOutput "Cancelled - keeping configuration and cache" -Level Info
                    }
                } else {
                    Remove-Item -Path $ConfigDir -Recurse -Force -ErrorAction Stop
                    $Script:RemovedItems += "Configuration directory: $ConfigDir"
                    Write-ColorOutput "Removed configuration directory" -Level Success
                }
            }
            default {
                Write-ColorOutput "Invalid choice - keeping configuration and cache" -Level Info
            }
        }
    }
}

function Show-Summary {
    Write-ColorOutput "Uninstallation Summary" -Level Header

    if ($RemovedItems.Count -eq 0) {
        Write-ColorOutput "No items were removed" -Level Info
        return
    }

    Write-Host "Removed items:" -ForegroundColor White
    Write-Host ""
    foreach ($item in $RemovedItems) {
        Write-Host "  ✓ $item" -ForegroundColor Green
    }
    Write-Host ""

    if ($DryRun) {
        Write-ColorOutput "This was a dry run - no changes were made" -Level Info
        Write-ColorOutput "Run without -DryRun to perform actual uninstallation" -Level Info
    } else {
        Write-ColorOutput "Astrolabe has been uninstalled" -Level Success

        # Check if binary is still accessible
        $stillInPath = Get-Command $BinaryName -ErrorAction SilentlyContinue
        if ($stillInPath) {
            Write-ColorOutput "Binary is still accessible via PATH" -Level Warning
            Write-ColorOutput "Restart your terminal for PATH changes to take effect" -Level Info
        }
    }

    Write-Host ""
    Write-ColorOutput "Thank you for using Astrolabe!" -Level Info
}

#endregion

#region Main Execution

function Main {
    # Parse arguments
    if ($Help) {
        Show-Help
        exit 0
    }

    # Validate parameter combinations
    if ($Complete -and $KeepCache) {
        Write-ColorOutput "Cannot use -Complete and -KeepCache together" -Level Error
        exit 1
    }

    # Show header
    Write-ColorOutput "Astrolabe Uninstaller" -Level Header

    if ($DryRun) {
        Write-ColorOutput "DRY RUN MODE - No changes will be made" -Level Warning
        Write-Host ""
    }

    # Run uninstallation steps
    Find-Installation
    Remove-Binary
    Remove-PathEntries
    Remove-Completion
    Remove-ConfigAndCache
    Show-Summary
}

# Run main function
try {
    Main
}
catch {
    Write-ColorOutput "Uninstallation failed: $_" -Level Error
    Write-ColorOutput $_.ScriptStackTrace -Level Error
    exit 1
}

#endregion
