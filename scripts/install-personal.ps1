$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$repo = "beichenl01033-sudo/JavBoss"
$platform = "windows-x86_64"
$releaseApi = "https://api.github.com/repos/$repo/releases/latest"
$installerUrl = "https://raw.githubusercontent.com/beichenl01033-sudo/JavBoss/personal/web/scripts/install.ps1"
$previousRepo = [Environment]::GetEnvironmentVariable("JAVBOSS_INSTALL_REPO", "Process")
$previousVersion = [Environment]::GetEnvironmentVariable("JAVBOSS_INSTALL_VERSION", "Process")

try {
  $release = Invoke-RestMethod -Uri $releaseApi -Headers @{ "User-Agent" = "JavBoss-Personal-Installer" }
  $version = "$($release.tag_name)".Trim()
  if (-not $version -or $version -notmatch "-personal\.") {
    throw "No valid personal JavBoss release was found"
  }

  $assetName = "javboss-$version-$platform.zip"
  $asset = @($release.assets | Where-Object { $_.name -eq $assetName }) | Select-Object -First 1
  if (-not $asset) {
    throw "The latest personal release does not contain $assetName"
  }

  $env:JAVBOSS_INSTALL_REPO = $repo
  $env:JAVBOSS_INSTALL_VERSION = $version
  $installer = Invoke-RestMethod -Uri $installerUrl -Headers @{ "User-Agent" = "JavBoss-Personal-Installer" }
  Invoke-Expression $installer
} finally {
  [Environment]::SetEnvironmentVariable("JAVBOSS_INSTALL_REPO", $previousRepo, "Process")
  [Environment]::SetEnvironmentVariable("JAVBOSS_INSTALL_VERSION", $previousVersion, "Process")
}
