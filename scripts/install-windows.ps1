# Lorag Installer for Windows
# Run as Administrator: powershell -ExecutionPolicy Bypass -File install-windows.ps1

$ErrorActionPreference = "Stop"

$APP_NAME = "Lorag"
$INSTALL_DIR = "$env:ProgramFiles\Lorag"
$BIN_DIR = "$env:ProgramFiles\Lorag\bin"
$DATA_DIR = "$env:LOCALAPPDATA\Lorag"

Write-Host "============================================" -ForegroundColor Cyan
Write-Host "     Installation de Lorag pour Windows" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""

# Check architecture
$ARCH = $env:PROCESSOR_ARCHITECTURE
if ($ARCH -eq "AMD64") {
    $DOWNLOAD_URL = "https://github.com/lorag/lorag/releases/latest/download/lorag-windows-amd64.zip"
    Write-Host "Architecture détectée: x64" -ForegroundColor Green
} elseif ($ARCH -eq "ARM64") {
    $DOWNLOAD_URL = "https://github.com/lorag/lorag/releases/latest/download/lorag-windows-arm64.zip"
    Write-Host "Architecture détectée: ARM64" -ForegroundColor Green
} else {
    Write-Host "Architecture non supportée: $ARCH" -ForegroundColor Red
    exit 1
}

# Download
Write-Host ""
Write-Host "Téléchargement de Lorag..." -ForegroundColor Yellow
$TEMP_FILE = "$env:TEMP\lorag.zip"
Invoke-WebRequest -Uri $DOWNLOAD_URL -OutFile $TEMP_FILE

# Extract
Write-Host "Extraction..." -ForegroundColor Yellow
Expand-Archive -Path $TEMP_FILE -DestinationPath "$env:TEMP\lorag" -Force

# Create directories
Write-Host "Création des répertoires..." -ForegroundColor Yellow
New-Item -ItemType Directory -Force -Path $INSTALL_DIR | Out-Null
New-Item -ItemType Directory -Force -Path $BIN_DIR | Out-Null
New-Item -ItemType Directory -Force -Path $DATA_DIR | Out-Null

# Install binaries
Write-Host "Installation des binaires..." -ForegroundColor Yellow
Copy-Item "$env:TEMP\lorag\lorag.exe" -Destination $BIN_DIR -Force
Copy-Item "$env:TEMP\lorag\loragctl.exe" -Destination $BIN_DIR -Force

# Add to PATH
$CurrentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
if ($CurrentPath -notlike "*$BIN_DIR*") {
    Write-Host "Ajout au PATH..." -ForegroundColor Yellow
    [Environment]::SetEnvironmentVariable("Path", "$CurrentPath;$BIN_DIR", "Machine")
}

# Create Start Menu shortcut
Write-Host "Création du raccourci..." -ForegroundColor Yellow
$WshShell = New-Object -comObject WScript.Shell
$Shortcut = $WshShell.CreateShortcut("$env:ProgramData\Microsoft\Windows\Start Menu\Programs\Lorag.lnk")
$Shortcut.TargetPath = "$BIN_DIR\lorag.exe"
$Shortcut.Arguments = "server"
$Shortcut.WorkingDirectory = $DATA_DIR
$Shortcut.IconLocation = "$BIN_DIR\lorag.exe,0"
$Shortcut.Save()

# Create Desktop shortcut
$DesktopShortcut = $WshShell.CreateShortcut("$env:PUBLIC\Desktop\Lorag.lnk")
$DesktopShortcut.TargetPath = "$BIN_DIR\lorag.exe"
$DesktopShortcut.Arguments = "server"
$DesktopShortcut.WorkingDirectory = $DATA_DIR
$DesktopShortcut.IconLocation = "$BIN_DIR\lorag.exe,0"
$DesktopShortcut.Save()

# Cleanup
Remove-Item $TEMP_FILE -Force
Remove-Item "$env:TEMP\lorag" -Recurse -Force

Write-Host ""
Write-Host "============================================" -ForegroundColor Green
Write-Host "     Installation terminée !" -ForegroundColor Green
Write-Host "============================================" -ForegroundColor Green
Write-Host ""
Write-Host "Lorag est installé dans :"
Write-Host "  $INSTALL_DIR"
Write-Host ""
Write-Host "Pour démarrer :"
Write-Host "  - Double-cliquez sur l'icône Lorag sur le bureau"
Write-Host "  - Ou cherchez 'Lorag' dans le menu Démarrer"
Write-Host "  - Ou ouvrez un terminal et tapez: lorag server"
Write-Host ""
Write-Host "Données stockées dans :"
Write-Host "  $DATA_DIR"
Write-Host ""
Write-Host "Documentation : https://github.com/lorag/lorag" -ForegroundColor Cyan
Write-Host ""

# Refresh PATH
$env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine")
