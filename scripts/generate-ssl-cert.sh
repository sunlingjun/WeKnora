#!/bin/bash

# 生成自签名 SSL 证书脚本
# 用于本地开发环境

set -e

# 配置
CERT_DIR="./ssl"
CERT_FILE="$CERT_DIR/cert.pem"
KEY_FILE="$CERT_DIR/key.pem"
DAYS=365
DOMAIN="localhost"

# 解析命令行参数
while [[ $# -gt 0 ]]; do
  case $1 in
    -d|--domain)
      DOMAIN="$2"
      shift 2
      ;;
    -o|--output)
      CERT_DIR="$2"
      CERT_FILE="$CERT_DIR/cert.pem"
      KEY_FILE="$CERT_DIR/key.pem"
      shift 2
      ;;
    -D|--days)
      DAYS="$2"
      shift 2
      ;;
    -h|--help)
      echo "Usage: $0 [OPTIONS]"
      echo "Options:"
      echo "  -d, --domain DOMAIN   证书域名 (默认: localhost)"
      echo "  -o, --output DIR      输出目录 (默认: ./ssl)"
      echo "  -D, --days DAYS        证书有效期天数 (默认: 365)"
      echo "  -h, --help            显示帮助信息"
      echo ""
      echo "示例:"
      echo "  $0 -d zsk.t.nxin.com -o ./ssl"
      exit 0
      ;;
    *)
      echo "未知参数: $1"
      echo "使用 -h 或 --help 查看帮助信息"
      exit 1
      ;;
  esac
done

# 创建证书目录
mkdir -p "$CERT_DIR"

# 检查是否已存在证书
if [ -f "$CERT_FILE" ] && [ -f "$KEY_FILE" ]; then
  echo "证书文件已存在: $CERT_FILE"
  read -p "是否覆盖? (y/N): " -n 1 -r
  echo
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "已取消"
    exit 0
  fi
fi

# 生成自签名证书
echo "正在生成 SSL 证书..."
echo "域名: $DOMAIN"
echo "输出目录: $CERT_DIR"
echo "有效期: $DAYS 天"

openssl req -x509 -nodes -days "$DAYS" -newkey rsa:2048 \
  -keyout "$KEY_FILE" \
  -out "$CERT_FILE" \
  -subj "/C=CN/ST=Beijing/L=Beijing/O=WeKnora/OU=Development/CN=$DOMAIN" \
  -addext "subjectAltName=DNS:$DOMAIN,DNS:*.$DOMAIN,DNS:localhost,IP:127.0.0.1,IP:::1"

# 设置权限
chmod 600 "$KEY_FILE"
chmod 644 "$CERT_FILE"

echo ""
echo "✅ SSL 证书生成成功!"
echo ""
echo "证书文件:"
echo "  - 证书: $CERT_FILE"
echo "  - 私钥: $KEY_FILE"
echo ""
echo "使用方法:"
echo "  export SSL_CERT_PATH=$CERT_FILE"
echo "  export SSL_KEY_PATH=$KEY_FILE"
echo "  go run cmd/server/main.go"
echo ""
echo "或者添加到 .env 文件:"
echo "  SSL_CERT_PATH=$CERT_FILE"
echo "  SSL_KEY_PATH=$KEY_FILE"
