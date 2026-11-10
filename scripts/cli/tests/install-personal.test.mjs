import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";

const root = new URL("../../../", import.meta.url);
const installer = fs.readFileSync(new URL("scripts/install.ps1", root), "utf8");
const personal = fs.readFileSync(new URL("scripts/install-personal.ps1", root), "utf8");
const powershell = process.env.JAVBOSS_TEST_POWERSHELL || "pwsh";
const available = spawnSync(powershell, ["-NoProfile", "-Command", "exit 0"]).status === 0;
const repo = "beichenl01033-sudo/JavBoss";

function fixture(t) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "javboss-personal-install-test-"));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  return dir;
}

// A BOM also lets Windows PowerShell 5.1 read the existing Chinese strings.
function writeScript(file, source) {
  fs.writeFileSync(file, "\ufeff" + source, "utf8");
}

const cases = [
  ["same-base official to personal", "v2.1.5", "v2.1.5-personal.1", true],
  ["official build metadata", "v2.1.5+build.1", "v2.1.5-personal.1", true],
  ["older official base", "v2.1.4", "v2.1.5-personal.1", true],
  ["older personal release", "v2.1.5-personal.1", "v2.1.5-personal.2", true],
  ["new personal base", "v2.1.4-personal.9", "v2.1.5-personal.1", true],
  ["fresh installation", null, "v2.1.5-personal.1", true],
  ["same personal release", "v2.1.5-personal.1", "v2.1.5-personal.1", false],
  ["same personal with metadata", "v2.1.5-personal.1+build.2", "v2.1.5-personal.1", false],
  ["newer personal release", "v2.1.5-personal.2", "v2.1.5-personal.1", false],
  ["newer official base", "v2.1.6", "v2.1.5-personal.1", false],
  ["newer official major", "v3.0.0", "v2.1.5-personal.1", false],
  ["newer personal base", "v2.1.6-personal.1", "v2.1.5-personal.1", false],
];

function runPersonalInstaller(t, { installed = "v2.1.5", target = "v2.1.5-personal.1", running = false, downloadFails = false, missingVersion = false, missingAsset = false } = {}) {
  const dir = fixture(t);
  const installDir = path.join(dir, "JavBoss");
  fs.mkdirSync(path.join(installDir, "data"), { recursive: true });
  fs.mkdirSync(path.join(dir, "temp"));
  fs.mkdirSync(path.join(dir, "package", "JavBoss"), { recursive: true });
  fs.writeFileSync(path.join(installDir, "data", "keep.db"), "user database");
  fs.writeFileSync(path.join(installDir, "config.toml"), "user config");
  if (installed !== null) {
    fs.writeFileSync(path.join(installDir, "javboss.exe"), "old binary");
    if (!missingVersion) fs.writeFileSync(path.join(installDir, ".version"), installed + "\n");
  }
  fs.writeFileSync(path.join(dir, "package", "JavBoss", "javboss.exe"), "personal binary");
  fs.writeFileSync(path.join(dir, "package", "JavBoss", "config.toml"), "default config");
  const start = installer.indexOf("\n$baseDir =");
  const mockedInstaller = installer.slice(0, start) + `
function Get-PlatformLabel { return 'windows-x86_64' }
function Get-JavBossProcessInInstallDir {
  if ($env:TEST_RUNNING -eq '1') { return [PSCustomObject]@{ ProcessId = 123 } }
  return @()
}
function New-JavBossShortcuts {}
function Start-JavBoss {
  param($InstallDir)
  Set-Content -LiteralPath (Join-Path $InstallDir 'started') -Value 'yes'
}
function Invoke-WebRequest {
  param($Uri, $OutFile, $Headers)
  $expected = 'https://github.com/${repo}/releases/download/' + $env:TARGET_TAG + '/javboss-' + $env:TARGET_TAG + '-windows-x86_64.zip'
  if ($Uri -ne $expected) { throw "wrong download repository or asset: $Uri" }
  Set-Content -LiteralPath (Join-Path $env:TEST_ROOT 'download-attempted') -Value 'yes'
  if ($env:TEST_DOWNLOAD_FAILS -eq '1') { throw 'simulated download failure' }
  Compress-Archive -LiteralPath (Join-Path $env:TEST_ROOT 'package/JavBoss') -DestinationPath $OutFile
}
` + installer.slice(start);
  writeScript(path.join(dir, "installer.ps1"), mockedInstaller);
  const harness = `
$ErrorActionPreference = 'Stop'
$env:JAVBOSS_INSTALL_REPO = 'original/repository'
$env:JAVBOSS_INSTALL_VERSION = 'v0.1.0'
function Invoke-RestMethod {
  param($Uri, $Headers)
  if ($Uri -eq 'https://api.github.com/repos/${repo}/releases/latest') {
    $assets = @()
    if ($env:TEST_MISSING_ASSET -ne '1') {
      $assets = @([PSCustomObject]@{ name = 'javboss-' + $env:TARGET_TAG + '-windows-x86_64.zip' })
    }
    return [PSCustomObject]@{ tag_name = $env:TARGET_TAG; assets = $assets }
  }
  if ($Uri -eq 'https://raw.githubusercontent.com/${repo}/personal/web/scripts/install.ps1') {
    return Get-Content -Raw -LiteralPath (Join-Path $env:TEST_ROOT 'installer.ps1')
  }
  throw "unexpected request: $Uri"
}
$failure = $null
try {
${personal}
} catch { $failure = $_.Exception.Message }
if ($env:JAVBOSS_INSTALL_REPO -ne 'original/repository' -or $env:JAVBOSS_INSTALL_VERSION -ne 'v0.1.0') { throw 'installer leaked process environment overrides' }
if ($failure) { throw $failure }
`;
  const script = path.join(dir, "test.ps1");
  writeScript(script, harness);
  const result = spawnSync(powershell, ["-NoProfile", "-File", script], {
    encoding: "utf8", timeout: 45000,
    env: {
      ...process.env, TEST_ROOT: dir, LOCALAPPDATA: dir, TARGET_TAG: target,
      TEMP: path.join(dir, "temp"), TMP: path.join(dir, "temp"), TMPDIR: path.join(dir, "temp"),
      TEST_RUNNING: running ? "1" : "0", TEST_DOWNLOAD_FAILS: downloadFails ? "1" : "0",
      TEST_MISSING_ASSET: missingAsset ? "1" : "0",
    },
  });
  return { ...result, dir, read: name => fs.readFileSync(path.join(installDir, name), "utf8").replace(/^\ufeff/, "").trim() };
}

for (const [name, installed, target, shouldInstall] of cases) {
  test(`personal installer: ${name}`, { skip: !available }, t => {
    const result = runPersonalInstaller(t, { installed, target });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(fs.existsSync(path.join(result.dir, "download-attempted")), shouldInstall);
    assert.equal(result.read("javboss.exe"), shouldInstall ? "personal binary" : "old binary");
    assert.equal(result.read(".version"), shouldInstall ? target : installed);
    assert.equal(result.read("data/keep.db"), "user database");
    assert.equal(result.read("config.toml"), "user config");
    assert.equal(fs.existsSync(path.join(result.dir, "JavBoss", "started")), shouldInstall);
    assert.match(result.stdout, /personal release selected: beichenl01033-sudo\/JavBoss/);
    assert.deepEqual(fs.readdirSync(path.join(result.dir, "temp")), []);
  });
}

for (const [name, options] of [
  ["running application", { running: true }],
  ["failed download", { downloadFails: true }],
  ["invalid installed version", { installed: "unknown" }],
  ["missing installed version", { missingVersion: true }],
  ["invalid target version", { target: "v02.1.5-personal.1" }],
  ["non-personal release", { target: "v2.1.5" }],
  ["missing Windows asset", { missingAsset: true }],
]) {
  test(`personal installer preserves files on ${name}`, { skip: !available }, t => {
    const result = runPersonalInstaller(t, options);
    assert.notEqual(result.status, 0);
    assert.equal(result.read("javboss.exe"), "old binary");
    assert.equal(result.read("config.toml"), "user config");
    assert.equal(result.read("data/keep.db"), "user database");
    if (!options.missingVersion) assert.equal(result.read(".version"), options.installed || "v2.1.5");
    assert(!fs.existsSync(path.join(result.dir, "JavBoss", "started")));
    if (!options.downloadFails) assert(!fs.existsSync(path.join(result.dir, "download-attempted")));
    assert.deepEqual(fs.readdirSync(path.join(result.dir, "temp")), []);
    assert.doesNotMatch(result.stderr, /leaked process environment overrides/);
  });
}

test("channel switch requires the personal repository and an exactly pinned target", { skip: !available }, t => {
  const dir = fixture(t);
  const script = path.join(dir, "switch.ps1");
  writeScript(script, installer.split("\n$baseDir =")[0] + `
foreach ($repository in @('Solr159/JavBoss', 'other/repository', '${repo}')) {
  foreach ($pinned in @('', 'v2.1.5-personal.2', 'v2.1.5-personal.1')) {
    $Repo = $repository
    $PinnedVersion = $pinned
    $expected = $repository -eq '${repo}' -and $pinned -eq 'v2.1.5-personal.1'
    $actual = Test-PersonalReleaseSwitch -InstalledVersion 'v2.1.5' -TargetVersion 'v2.1.5-personal.1'
    if ($actual -ne $expected) { throw "unexpected channel switch for $repository / $pinned" }
  }
}
`);
  const result = spawnSync(powershell, ["-NoProfile", "-File", script], { encoding: "utf8", timeout: 15000 });
  assert.equal(result.status, 0, result.stderr);
});
