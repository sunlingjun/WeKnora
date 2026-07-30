package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
)

// duckdbExtensions required by WeKnora data analysis (httpfs for remote CSV, spatial/excel for sheets).
var duckdbExtensions = []string{"httpfs", "spatial", "excel"}

func loadExtension(ctx context.Context, db *sql.DB, ext string) error {
	// LOAD 会优先使用本地已有的扩展文件；若缺失或未解压到期望位置，则会返回错误。
	_, err := db.ExecContext(ctx, fmt.Sprintf("LOAD %s;", ext))
	return err
}

func installExtension(ctx context.Context, db *sql.DB, ext string) error {
	const maxAttempts = 5
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if _, err := db.ExecContext(ctx, fmt.Sprintf("INSTALL %s;", ext)); err != nil {
			lastErr = err
			fmt.Printf("INSTALL %s failed (attempt %d/%d): %v\n", ext, attempt, maxAttempts, err)
			if attempt < maxAttempts {
				time.Sleep(time.Duration(attempt*5) * time.Second)
			}
			continue
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf("LOAD %s;", ext)); err != nil {
			lastErr = err
			fmt.Printf("LOAD %s failed (attempt %d/%d): %v\n", ext, attempt, maxAttempts, err)
			if attempt < maxAttempts {
				time.Sleep(time.Duration(attempt*5) * time.Second)
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("failed to install %s extension after %d attempts: %w", ext, maxAttempts, lastErr)
}

func downloadExtensions() {
	ctx := context.Background()

	sqlDB, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		panic(err)
	}
	defer sqlDB.Close()

	for _, ext := range duckdbExtensions {
		// 先尝试 LOAD：若 Dockerfile 已通过 curl 预下载并解压到 ~/.duckdb，
		// 这里通常不会触发 INSTALL 的远程下载。
		if err := loadExtension(ctx, sqlDB, ext); err == nil {
			continue
		}
		// LOAD-first 失败才走 INSTALL（并在 INSTALL 内部同时 LOAD 校验），保留重试兜底。
		if err := installExtension(ctx, sqlDB, ext); err != nil {
			panic(err)
		}
	}
}

func main() {
	downloadExtensions()
}
