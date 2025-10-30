# Windows Testing Guide for Astrolabe Installation Scripts

This guide will help you test the PowerShell installation scripts on your Windows system.

## Prerequisites

Before testing, ensure you have:

1. **Windows 10 or Windows 11** (or Windows Server 2019+)
2. **PowerShell 5.1 or later** (check with `$PSVersionTable.PSVersion`)
3. **Go installed** (to build the binary)
4. **Git** (to clone/pull the repository)

## Step 1: Build the Windows Binary

First, you need to build `astrolabe.exe` on your Windows system.

```powershell
# Navigate to project directory
cd C:\path\to\qa_cli_agent

# Build the binary
go build -o astrolabe.exe .\cmd\astrolabe

# Verify binary was created
dir astrolabe.exe

# Test binary execution
.\astrolabe.exe version
```

**Expected output:**
```
Agent Version: 0.1.0
Schema Version: 1.0.0
```

## Step 2: Check PowerShell Execution Policy

Before running the installation scripts, check your execution policy:

```powershell
# Check current policy
Get-ExecutionPolicy -List

# If scripts are blocked, enable for current session
Set-ExecutionPolicy Bypass -Scope Process

# Or enable for current user (persistent)
Set-ExecutionPolicy RemoteSigned -Scope CurrentUser
```

## Step 3: Test Installation Script (Dry Run)

Test the installation script without making any changes:

```powershell
# Dry run - see what would happen
.\scripts\install.ps1 -DryRun
```

**What to verify:**
- ✓ PowerShell version detected correctly
- ✓ Windows version and architecture displayed
- ✓ Binary found at `.\astrolabe.exe`
- ✓ Installation directory would be created
- ✓ PATH would be updated
- ✓ Config directory would be created
- ✓ COM ports detected (if any USB-Serial adapters)
- ✓ Installation would validate successfully

**Expected output should include:**
```
Astrolabe Installer v0.1.0

⚠ DRY RUN MODE - No changes will be made

ℹ Detected platform: ...
✓ Found binary: ...
→ Installing binary to: C:\Users\<YourName>\AppData\Local\Programs\Astrolabe
...
✓ [DRY RUN] Installation validation would succeed
```

## Step 4: Test Actual Installation (User-Local)

Perform the actual installation (user-local, no admin required):

```powershell
# User-local installation
.\scripts\install.ps1
```

**During installation:**
- Respond `y` when asked to overwrite (if already installed)
- Respond `Y` when asked about PowerShell completion (or `n` to skip)

**What to verify:**
- ✓ Binary copied to `%LOCALAPPDATA%\Programs\Astrolabe\`
- ✓ Directory added to User PATH
- ✓ Config directory created: `%USERPROFILE%\.astrolabe\`
- ✓ PowerShell completion installed (if accepted)
- ✓ Installation validates successfully

**After installation, verify:**

```powershell
# Restart PowerShell (to pick up PATH changes)
# Then verify:

# 1. Check if binary is in PATH
where.exe astrolabe

# 2. Run astrolabe
astrolabe version

# 3. Check installation directory
dir "$env:LOCALAPPDATA\Programs\Astrolabe"

# 4. Check config directory
dir "$env:USERPROFILE\.astrolabe"

# 5. Test completion (if installed)
astrolabe [Tab]  # Should show completions
```

## Step 5: Test PowerShell Completion

If you installed PowerShell completion, test it:

```powershell
# Restart PowerShell first
exit
# Open new PowerShell window

# Test completion
astrolabe [Tab]           # Should list subcommands
astrolabe capture [Tab]   # Should list capture sources
```

## Step 6: Test Uninstallation Script (Dry Run)

Test the uninstallation script without making changes:

```powershell
.\scripts\uninstall.ps1 -DryRun
```

**What to verify:**
- ✓ Found binary location
- ✓ Would remove binary
- ✓ Would remove PATH entry
- ✓ Would prompt for completion removal
- ✓ Would prompt for config/cache cleanup

## Step 7: Test Actual Uninstallation

Test the uninstallation process:

```powershell
# Interactive uninstallation
.\scripts\uninstall.ps1
```

**During uninstallation:**
- Choose option 1, 2, or 3 for config/cache cleanup
  - **Option 1**: Keep everything (safest for testing)
  - **Option 2**: Remove configs only
  - **Option 3**: Complete removal (WARNING: deletes everything)

**After uninstallation, verify:**

```powershell
# 1. Binary should be gone
where.exe astrolabe  # Should return error

# 2. Check installation directory removed
Test-Path "$env:LOCALAPPDATA\Programs\Astrolabe"  # Should be False

# 3. Check PATH
$env:Path -split ';' | Select-String "Astrolabe"  # Should be empty

# 4. Check config directory (depends on cleanup option)
Test-Path "$env:USERPROFILE\.astrolabe"
```

## Step 8: Test System-Wide Installation (Optional)

If you have Administrator rights, test system-wide installation:

```powershell
# Open PowerShell as Administrator
# Right-click PowerShell → "Run as Administrator"

# Navigate to project
cd C:\path\to\qa_cli_agent

# Test dry run
.\scripts\install.ps1 -SystemWide -DryRun

# Actual installation
.\scripts\install.ps1 -SystemWide
```

**What to verify:**
- ✓ Binary copied to `C:\Program Files\Astrolabe\`
- ✓ Added to System PATH (Machine scope)
- ✓ Works for all users on the system

**After system-wide installation:**

```powershell
# Verify installation
where.exe astrolabe  # Should show C:\Program Files\Astrolabe\astrolabe.exe

# Test uninstallation (requires admin)
.\scripts\uninstall.ps1
```

## Test Matrix

Use this checklist to ensure comprehensive testing:

### Installation Tests
- [ ] Dry run succeeds
- [ ] User-local installation succeeds
- [ ] Binary executes after installation (`astrolabe version`)
- [ ] PATH updated correctly (restart PowerShell and test)
- [ ] Config directory created
- [ ] PowerShell completion works (if installed)
- [ ] Can install over existing installation
- [ ] System-wide installation succeeds (if testing with admin)

### Uninstallation Tests
- [ ] Dry run shows what would be removed
- [ ] Interactive uninstallation works
- [ ] Binary removed from file system
- [ ] PATH cleaned up correctly
- [ ] Completion removed from profile (if applicable)
- [ ] Config cleanup options work (keep all/configs only/complete)
- [ ] System-wide uninstallation succeeds (if testing with admin)

### Edge Cases
- [ ] Installing without binary present shows clear error
- [ ] Installing without execution policy set shows clear error
- [ ] System-wide without admin shows clear error
- [ ] Uninstalling when not installed handles gracefully
- [ ] Scripts handle spaces in paths correctly
- [ ] Scripts work in Windows Terminal
- [ ] Scripts work in PowerShell 5.1 (built-in)
- [ ] Scripts work in PowerShell 7+ (if available)

## Common Issues and Solutions

### Issue: "Execution Policy" Error
```
.\install.ps1 : File cannot be loaded because running scripts is disabled
```

**Solution:**
```powershell
Set-ExecutionPolicy Bypass -Scope Process
```

### Issue: "Access Denied" for System-Wide
```
Access to the path 'C:\Program Files' is denied.
```

**Solution:**
Run PowerShell as Administrator or use user-local installation:
```powershell
.\scripts\install.ps1  # User-local, no admin needed
```

### Issue: Binary Not Found After Installation
```
'astrolabe' is not recognized as an internal or external command
```

**Solution:**
Restart PowerShell to pick up PATH changes:
```powershell
exit
# Open new PowerShell window
astrolabe version
```

### Issue: Colors Not Displaying
If emoji and colors don't show correctly:

**Solution:**
Use Windows Terminal (recommended):
```powershell
# Download from Microsoft Store
# Or disable colors:
$env:NO_COLOR = "1"
.\scripts\install.ps1
```

### Issue: COM Ports Not Detected
```
No serial ports detected
```

**Solution:**
Check Device Manager and install USB-Serial drivers:
```powershell
# Open Device Manager
devmgmt.msc

# Look for "Ports (COM & LPT)"
# Install FTDI, CH340, or other drivers if needed
```

## Reporting Issues

When reporting issues, please include:

1. **PowerShell version**: `$PSVersionTable.PSVersion`
2. **Windows version**: `[System.Environment]::OSVersion`
3. **Execution policy**: `Get-ExecutionPolicy -List`
4. **Full error message** (copy from PowerShell)
5. **Script output** (complete output from script execution)
6. **Test scenario** (dry-run, user-local, system-wide, etc.)

## Success Criteria

Installation is successful when:
- ✓ `astrolabe version` executes from any directory
- ✓ Config directory exists at `%USERPROFILE%\.astrolabe\`
- ✓ PATH contains installation directory
- ✓ PowerShell completion works (if enabled)
- ✓ Uninstallation removes all traces (except configs if chosen)

## Next Steps After Testing

Once testing is complete and issues are resolved:

1. Update [TODO.md](../TODO.md) to mark Windows testing complete
2. Document any Windows-specific quirks discovered
3. Update [scripts/README.md](README.md) with additional troubleshooting if needed
4. Consider creating GitHub release with pre-built binaries for Windows

## Questions?

If you encounter any issues or have questions during testing:
1. Check [scripts/README.md](README.md) for troubleshooting
2. Review this testing guide for common solutions
3. Report issues with full details (see "Reporting Issues" above)
