import { fileURLToPath, URL } from 'node:url'
import { readFileSync } from 'node:fs'
import { join, resolve, dirname } from 'node:path'
import { existsSync } from 'node:fs'
import { execSync } from 'node:child_process'
import { createRequire } from 'node:module'
import { defineConfig, type Plugin, type ServerOptions } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueJsx from '@vitejs/plugin-vue-jsx'
// 获取当前文件所在目录（ESM 模块方式）
const __dirname = dirname(fileURLToPath(import.meta.url))
const require = createRequire(import.meta.url)

const pkg = require('./package.json') as { version?: string }
const FRONTEND_VERSION = pkg.version ?? 'unknown'

function resolveFrontendCommit(): string {
  const fromEnv = process.env.VITE_FRONTEND_COMMIT || process.env.GITHUB_SHA
  if (fromEnv) {
    return fromEnv.slice(0, 7)
  }
  try {
    return execSync('git rev-parse --short HEAD', { stdio: ['ignore', 'pipe', 'ignore'] })
      .toString()
      .trim()
  } catch {
    return 'unknown'
  }
}

const FRONTEND_COMMIT = resolveFrontendCommit()

/** Dev parity with nginx: serve embed.html for /embed/:channelId (not the main SPA). */
function embedHtmlDevFallback(): Plugin {
  return {
    name: 'embed-html-dev-fallback',
    configureServer(server) {
      server.middlewares.use((req, _res, next) => {
        const raw = req.url ?? ''
        const qIdx = raw.indexOf('?')
        const path = qIdx >= 0 ? raw.slice(0, qIdx) : raw
        const qs = qIdx >= 0 ? raw.slice(qIdx) : ''
        if (path.startsWith('/embed/') && path !== '/embed.html' && !path.includes('.')) {
          req.url = `/embed.html${qs}`
        }
        next()
      })
    },
  }
}
// 与 config.yaml server.https.enabled 对齐：后端 8080 为进程内 HTTPS，代理须用 https://
const DEV_PROXY_TARGET =
  process.env.VITE_DEV_PROXY_TARGET ||
  process.env.FRONTEND_BACKEND_URL ||
  process.env.BACKEND_URL ||
  'https://127.0.0.1:8080'

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
  define: {
    __FRONTEND_VERSION__: JSON.stringify(FRONTEND_VERSION),
    __FRONTEND_COMMIT__: JSON.stringify(FRONTEND_COMMIT),
  },
  build: {
    modulePreload: {
      resolveDependencies(_filename, deps, { hostId }) {
        // Embed iframe bootstraps with token exchange only; defer heavy chat chunks.
        if (hostId?.includes('embed')) {
          return deps.filter((dep) => !(
            dep.includes('vendor-mermaid')
            || dep.includes('vendor-highlight')
            || dep.includes('vendor-markdown')
            || dep.includes('vendor-tdesign')
            || dep.includes('botmsg')
            || dep.includes('usermsg')
            || dep.includes('EmbedBotMessage')
            || dep.includes('EmbedUserMessage')
            || dep.includes('AgentStreamDisplay')
            || dep.includes('EmbedChatCore')
            || dep.includes('vendor-markdown')
            || dep.includes('fonts-')
          ))
        }
        return deps
      },
    },
    rollupOptions: {
      input: {
        main: resolve(__dirname, 'index.html'),
        embed: resolve(__dirname, 'embed.html'),
      },
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return
          if (id.includes('mermaid') || id.includes('/dagre') || id.includes('cytoscape')) {
            return 'vendor-mermaid'
          }
          if (id.includes('marked') || id.includes('katex')) {
            return 'vendor-markdown'
          }
          if (id.includes('highlight.js')) {
            return 'vendor-highlight'
          }
        },
      },
    },
  },
  plugins: [
    vue(),
    vueJsx(),
    embedHtmlDevFallback(),
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
        target: DEV_PROXY_TARGET,
        changeOrigin: true,
        secure: false, // 本地开发使用 secure: false（允许自签名证书）
        // 如果需要重写路径，可以取消注释下面的配置
        // rewrite: (path) => path.replace(/^\/api/, '')
      },
      '/files': {
        target: DEV_PROXY_TARGET,
        changeOrigin: true,
        secure: false,
      }
    }
  },
  // `vite preview` 用生产构建产物(dist)本地起服务，是最接近 release 镜像的环境：
  // 同样的压缩 / 拆包 / CSS 加载顺序，可提前暴露只在生产构建出现的问题
  // （如主题变量被打包顺序覆盖）。用法：npm run build && npm run preview
  preview: {
    port: 4173,
    host: true,
    proxy: {
      '/api': {
        target: DEV_PROXY_TARGET,
        changeOrigin: true,
        secure: false,
      },
      '/files': {
        target: DEV_PROXY_TARGET,
        changeOrigin: true,
        secure: false,
      }
    }
  }
})
