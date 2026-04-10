package main

import (
	"context"
	"database/sql"

	_ "github.com/duckdb/duckdb-go/v2"
)

func downloadExtensions() {
	ctx := context.Background()

	sqlDB, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		panic(err)
	}
	defer sqlDB.Close()

	// httpfs: 用于 read_csv_auto('http(s)://...') 等远程文件读取
	for _, ext := range []string{"httpfs", "spatial"} {
		if _, err := sqlDB.ExecContext(ctx, "INSTALL "+ext+";"); err != nil {
			panic(err)
		}
		if _, err := sqlDB.ExecContext(ctx, "LOAD "+ext+";"); err != nil {
			panic(err)
		}
	}
}

func main() {
	downloadExtensions()
}
