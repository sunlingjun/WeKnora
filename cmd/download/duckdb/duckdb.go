package main

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/duckdb/duckdb-go/v2"
)

// duckdbExtensions required by WeKnora data analysis (httpfs for remote CSV, spatial/excel for sheets).
var duckdbExtensions = []string{"httpfs", "spatial", "excel"}

func downloadExtensions() {
	ctx := context.Background()

	sqlDB, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		panic(err)
	}
	defer sqlDB.Close()

	for _, ext := range duckdbExtensions {
		if _, err := sqlDB.ExecContext(ctx, fmt.Sprintf("INSTALL %s;", ext)); err != nil {
			panic(fmt.Errorf("failed to install %s extension: %w", ext, err))
		}
		if _, err := sqlDB.ExecContext(ctx, fmt.Sprintf("LOAD %s;", ext)); err != nil {
			panic(fmt.Errorf("failed to load %s extension: %w", ext, err))
		}
	}
}

func main() {
	downloadExtensions()
}
