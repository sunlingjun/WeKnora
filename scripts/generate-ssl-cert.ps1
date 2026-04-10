# PowerShell 脚本：生成自签名 SSL 证书
# 用于 Windows 本地开发环境

param(
    [string]$Domain = "localhost",
    [string]$OutputDir = "./ssl",
    [int]$Days = 365
)

# 创建证书目录
if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir | Out-Null
}

$CertFile = Join-Path $OutputDir "cert.pem"
$KeyFile = Join-Path $OutputDir "key.pem"

# 检查是否已存在证书
if ((Test-Path $CertFile) -and (Test-Path $KeyFile)) {
    $overwrite = Read-Host "证书文件已存在，是否覆盖? (y/N)"
    if ($overwrite -ne 'y' -and $overwrite -ne 'Y') {
        Write-Host "已取消" -ForegroundColor Yellow
        exit 0
    }
}

Write-Host "正在生成 SSL 证书..." -ForegroundColor Green
Write-Host "域名: $Domain"
Write-Host "输出目录: $OutputDir"
Write-Host "有效期: $Days 天"

# 生成自签名证书
# 注意：需要安装 OpenSSL for Windows
# 下载地址: https://slproweb.com/products/Win32OpenSSL.html

$opensslPath = "openssl"
if (-not (Get-Command $opensslPath -ErrorAction SilentlyContinue)) {
    Write-Host "错误: 未找到 openssl 命令" -ForegroundColor Red
    Write-Host "请安装 OpenSSL for Windows:" -ForegroundColor Yellow
    Write-Host "  https://slproweb.com/products/Win32OpenSSL.html" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "或者使用 Git Bash 运行 generate-ssl-cert.sh 脚本" -ForegroundColor Yellow
    exit 1
}

$subject = "/C=CN/ST=Beijing/L=Beijing/O=WeKnora/OU=Development/CN=$Domain"
$san = "subjectAltName=DNS:$Domain,DNS:*.$Domain,DNS:localhost,IP:127.0.0.1"

& $opensslPath req -x509 -nodes -days $Days -newkey rsa:2048 `
    -keyout $KeyFile `
    -out $CertFile `
    -subj $subject `
    -addext $san

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "✅ SSL 证书生成成功!" -ForegroundColor Green
    Write-Host ""
    Write-Host "证书文件:"
    Write-Host "  - 证书: $CertFile"
    Write-Host "  - 私钥: $KeyFile"
    Write-Host ""
    Write-Host "使用方法:"
    Write-Host "  `$env:SSL_CERT_PATH='$CertFile'"
    Write-Host "  `$env:SSL_KEY_PATH='$KeyFile'"
    Write-Host "  go run cmd/server/main.go"
    Write-Host ""
    Write-Host "或者添加到 .env 文件:"
    Write-Host "  SSL_CERT_PATH=$CertFile"
    Write-Host "  SSL_KEY_PATH=$KeyFile"
} else {
    Write-Host "错误: 证书生成失败" -ForegroundColor Red
    exit 1
}
