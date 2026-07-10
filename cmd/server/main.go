// Package main is the main package for the WeKnora server
// It contains the main function and the entry point for the server
//
// @title           WeKnora API
// @version         1.0
// @description     WeKnora 知识库管理系统 API 文档
// @termsOfService  http://swagger.io/terms/
//
// @contact.name   WeKnora Github
// @contact.url    https://github.com/Tencent/WeKnora
//
// @BasePath  /api/v1
//
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description 用户登录认证：输入 Bearer {token} 格式的 JWT 令牌

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key
// @description 租户身份认证：输入 sk- 开头的 API Key
package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/container"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/runtime"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// resolveInProcessTLSFromConfig returns cert paths when server.https is enabled.
// Paths must be set in YAML (validated in config.ValidateConfig); there is no default file path.
func resolveInProcessTLSFromConfig(cfg *config.Config) (certFile, keyFile string, useTLS bool) {
	if cfg == nil || cfg.Server == nil {
		return "", "", false
	}
	httpsCfg := cfg.Server.HTTPS
	if httpsCfg == nil || !httpsCfg.Enabled {
		return "", "", false
	}
	return strings.TrimSpace(httpsCfg.CertFile), strings.TrimSpace(httpsCfg.KeyFile), true
}

func main() {
	// Set Gin mode
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}
	// Mute Gin's per-route registration spam (one line per route × ~150
	// routes) — replaced by a single summary printed after router build.
	runtime.SilenceGinRouteSpam()
	// Print the env banner before container build so operators see what
	// config landed even when DB / storage init fails.
	runtime.LogStartupEnv(context.Background())
	runtime.MarkServerStarted()

	// Build dependency injection container
	c := container.BuildContainer(runtime.GetContainer())

	// One-shot bootstrap hooks (e.g. promote env-named user to system
	// admin). Best-effort: never aborts startup — see bootstrap.go.
	runStartupBootstrap(c)

	// Run application
	err := c.Invoke(func(
		cfg *config.Config,
		router *gin.Engine,
		resourceCleaner interfaces.ResourceCleaner,
		systemSettingSvc interfaces.SystemSettingService,
	) error {
		// Create HTTP server
		server := &http.Server{
			Handler: router,
		}

		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		tcpListener, err := listenWithRetry(addr, 10, 300*time.Millisecond)
		if err != nil {
			return fmt.Errorf("failed to start server: %v", err)
		}

		certFile, keyFile, tlsOK := resolveInProcessTLSFromConfig(cfg)
		serveListener := tcpListener
		if tlsOK {
			cert, tlsErr := tls.LoadX509KeyPair(certFile, keyFile)
			if tlsErr != nil {
				_ = tcpListener.Close()
				return fmt.Errorf("load TLS certificate: %w", tlsErr)
			}
			tlsCfg := &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			}
			serveListener = tls.NewListener(tcpListener, tlsCfg)
			logger.Infof(context.Background(), "in-process HTTPS enabled, cert=%s key=%s", certFile, keyFile)
		}

		ctx, done := context.WithCancel(context.Background())

		// Start the system_settings pubsub subscriber. Runs in its own
		// goroutine and exits when ctx is cancelled at shutdown. Best-
		// effort: an error here only warns (Redis may legitimately be
		// disabled in lite-mode deployments — the service no-ops in
		// that case anyway).
		if err := systemSettingSvc.SubscribeRedis(ctx); err != nil {
			logger.Warnf(ctx, "[system_settings] subscribe failed: %v", err)
		}

		signals := make(chan os.Signal, 1)
		signal.Notify(signals, shutdownSignals...)
		go func() {
			sig := <-signals
			logger.Infof(context.Background(), "Received signal: %v, starting server shutdown...", sig)

			// Close listener first to release port immediately,
			// so the next process can bind during our graceful drain.
			_ = serveListener.Close()

			shutdownTimeout := cfg.Server.ShutdownTimeout
			if shutdownTimeout == 0 {
				shutdownTimeout = 30 * time.Second
			}
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer shutdownCancel()

			// Second signal → force close all connections immediately
			go func() {
				sig := <-signals
				logger.Warnf(context.Background(), "Received second signal: %v, forcing shutdown...", sig)
				server.Close()
			}()

			if err := server.Shutdown(shutdownCtx); err != nil {
				logger.Errorf(context.Background(), "Server forced to shutdown: %v", err)
				server.Close()
			}

			logger.Info(context.Background(), "Cleaning up resources...")
			errs := resourceCleaner.Cleanup(shutdownCtx)
			if len(errs) > 0 {
				logger.Errorf(context.Background(), "Errors occurred during resource cleanup: %v", errs)
			}
			logger.Info(context.Background(), "Server has exited")
			done()
		}()

		runtime.LogGinRouteCount(context.Background())
		scheme := "http"
		if tlsOK {
			scheme = "https"
		}
		logger.Infof(context.Background(), "Server is running at %s://%s", scheme, addr)
		if scheme == "http" {
			logger.Infof(context.Background(), "Tip: 进程内 TLS 在 config.yaml 中配置 server.https（enabled + cert_file + key_file）；前置 Nginx 终结 TLS 时设置 server.behind_proxy=true，并省略 https 块。")
		}
		if err := server.Serve(serveListener); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %v", err)
		}

		<-ctx.Done()
		return nil
	})
	if err != nil {
		logger.Fatalf(context.Background(), "Failed to run application: %v", err)
	}
}
