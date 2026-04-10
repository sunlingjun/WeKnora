import { fileURLToPath, URL } from 'node:url'
import { readFileSync } from 'node:fs'
import { join, resolve, dirname } from 'node:path'
import { existsSync } from 'node:fs'
import { createRequire } from 'node:module'
import { defineConfig } from 'vite'
import type { ServerOptions } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueJsx from '@vitejs/plugin-vue-jsx'
// 获取当前文件所在目录（ESM 模块方式）
const __dirname = dirname(fileURLToPath(import.meta.url))
const require = createRequire(import.meta.url)

function resolveVueOfficePptxEntry(): string {
  try {
    const pkgDir = dirname(require.resolve('@vue-office/pptx/package.json'))
    const candidates = [
      resolve(pkgDir, 'lib/v3/index.js'),
      resolve(pkgDir, 'lib/index.js'),
      resolve(pkgDir, 'lib/v3/vue-office-pptx.mjs'),
    ]
    const matched = candidates.find((candidate) => existsSync(candidate))
    return matched ?? '@vue-office/pptx'
  } catch {
    return '@vue-office/pptx'
  }
}

// 获取 HTTPS 配置
function getHttpsConfig(): ServerOptions['https'] {
  // 尝试从环境变量读取证书路径，否则使用默认路径
  // 注意：__dirname 是 vite.config.ts 所在目录（frontend/），ssl 在项目根目录
  const keyPath = process.env.SSL_KEY_PATH || join(__dirname, '../ssl/key.pem')
  const certPath = process.env.SSL_CERT_PATH || join(__dirname, '../ssl/cert.pem')

  // 输出调试信息
  console.log(`[HTTPS Config] Looking for certificates:`)
  console.log(`  Key: ${keyPath}`)
  console.log(`  Cert: ${certPath}`)

  try {
    // 读取证书文件
    const key = readFileSync(keyPath)
    const cert = readFileSync(certPath)
    console.log(`✓ Using SSL certificates: ${certPath}`)
    console.log(`  Key size: ${key.length} bytes`)
    console.log(`  Cert size: ${cert.length} bytes`)
    return { key, cert }
  } catch (error: any) {
    // 如果证书文件不存在，Vite 会自动生成自签名证书
    console.log('⚠ SSL certificates not found, Vite will generate self-signed certificate')
    console.log(`  Expected paths: ${keyPath}, ${certPath}`)
    if (error?.message) {
      console.log(`  Error: ${error.message}`)
    }
    console.log(`  Current working directory: ${process.cwd()}`)
    console.log(`  Config file directory: ${__dirname}`)
    // 返回 true 让 Vite 自动生成自签名证书
    return true as any // 类型断言，Vite 7 支持 boolean
  }
}

export default defineConfig({
  plugins: [
    vue(),
    vueJsx(),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '@vue-office/pptx': resolveVueOfficePptxEntry(),
    },
  },
  server: {
    port: 443, // 使用标准 HTTP/HTTPS 端口（需要管理员权限）
    // 如果 80 端口有问题，可以改为 8081（无需管理员权限）
    // port: 8081,
    host: '0.0.0.0', // 监听所有网络接口，允许外部访问
    strictPort: false, // 如果端口被占用，自动尝试下一个端口
    https: getHttpsConfig(),
    open: false, // 不自动打开浏览器
    // HMR 配置
    hmr: {
      overlay: true, // 显示错误覆盖层
      // 不设置 host，让 Vite 自动使用当前访问的域名
      // 这样无论通过 localhost 还是 zsk.t.nxin.com 访问，WebSocket 都会使用相同的域名
      port: 80, // 使用相同的端口（如果改为 8081，这里也要改）
      protocol: 'wss', // 使用 WSS（HTTPS 环境）
    },
    // 允许的主机名（用于 CAS 单点登录开发环境）
    allowedHosts: [
      'zsk.t.nxin.com',      // 测试环境
      'zsk.nxin.com',        // 生产环境
      'localhost',            // 本地开发
      '.nxin.com',            // 所有 nxin.com 子域名
    ],
    // 代理配置，用于开发环境
    proxy: {
      '/api': {
        target: process.env.BACKEND_URL || 'https://zsk.t.nxin.com:8080', // 本地开发使用 HTTP
        changeOrigin: true,
        secure: false, // 本地开发使用 secure: false（允许自签名证书）
        // 如果需要重写路径，可以取消注释下面的配置
        // rewrite: (path) => path.replace(/^\/api/, '')
      },
      '/files': {
          target: process.env.BACKEND_URL || 'https://zsk.t.nxin.com:8080', // 本地开发使用 HTTP
        changeOrigin: true,
        secure: false,
      }
    }
  }
})
