package router

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
	"go.uber.org/dig"
)

type AsynqTaskParams struct {
	dig.In

	Server               *asynq.Server
	KnowledgeService     interfaces.KnowledgeService
	KnowledgeBaseService interfaces.KnowledgeBaseService
	TagService           interfaces.KnowledgeTagService
	DataSourceService    interfaces.DataSourceService
	ChunkExtractor       interfaces.TaskHandler `name:"chunkExtractor"`
	DataTableSummary     interfaces.TaskHandler `name:"dataTableSummary"`
	ImageMultimodal      interfaces.TaskHandler `name:"imageMultimodal"`
	VideoMultimodal      interfaces.TaskHandler `name:"videoMultimodal"`
}

// getAsynqRedisConnOpt 返回通用的 Redis 连接配置，支持单机和集群模式。
// 单机：REDIS_MODE 为空或 single，使用 REDIS_ADDR / REDIS_DB 等；
// 集群：REDIS_MODE=cluster，使用 REDIS_CLUSTER_ADDRS（逗号分隔的 host:port 列表）。
func getAsynqRedisConnOpt() asynq.RedisConnOpt {
	mode := strings.ToLower(os.Getenv("REDIS_MODE"))

	username := os.Getenv("REDIS_USERNAME")
	password := os.Getenv("REDIS_PASSWORD")

	if mode == "cluster" {
		// 集群模式：REDIS_CLUSTER_ADDRS=host1:6379,host2:6379,...
		raw := os.Getenv("REDIS_CLUSTER_ADDRS")
		parts := strings.Split(raw, ",")
		addrs := make([]string, 0, len(parts))
		for _, s := range parts {
			s = strings.TrimSpace(s)
			if s != "" {
				addrs = append(addrs, s)
			}
		}

		return &asynq.RedisClusterClientOpt{
			Addrs:        addrs,
			Username:     username,
			Password:     password,
			ReadTimeout:  100 * time.Millisecond,
			WriteTimeout: 200 * time.Millisecond,
		}
	}

	// 默认单机模式，保持原有行为
	db := 0
	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		if parsed, err := strconv.Atoi(dbStr); err == nil {
			db = parsed
		}
	}

	return &asynq.RedisClientOpt{
		Addr:         os.Getenv("REDIS_ADDR"),
		Username:     username,
		Password:     password,
		ReadTimeout:  100 * time.Millisecond,
		WriteTimeout: 200 * time.Millisecond,
		DB:           db,
	}
}

func NewAsyncqClient() (*asynq.Client, error) {
	opt := getAsynqRedisConnOpt()
	client := asynq.NewClient(opt)
	err := client.Ping()
	if err != nil {
		return nil, err
	}
	return client, nil
}

func NewAsynqServer() *asynq.Server {
	opt := getAsynqRedisConnOpt()
	srv := asynq.NewServer(
		opt,
		asynq.Config{
			Queues: map[string]int{
				"critical": 6, // Highest priority queue
				"default":  3, // Default priority queue
				"low":      1, // Lowest priority queue
			},
		},
	)
	return srv
}

func RunAsynqServer(params AsynqTaskParams) *asynq.ServeMux {
	// Create a new mux and register all handlers
	mux := asynq.NewServeMux()

	// Register extract handlers - router will dispatch to appropriate handler
	mux.HandleFunc(types.TypeChunkExtract, params.ChunkExtractor.Handle)
	mux.HandleFunc(types.TypeDataTableSummary, params.DataTableSummary.Handle)

	// Register document processing handler
	mux.HandleFunc(types.TypeDocumentProcess, params.KnowledgeService.ProcessDocument)

	// Register manual knowledge processing handler (cleanup + re-indexing)
	mux.HandleFunc(types.TypeManualProcess, params.KnowledgeService.ProcessManualUpdate)

	// Register FAQ import handler (includes dry run mode)
	mux.HandleFunc(types.TypeFAQImport, params.KnowledgeService.ProcessFAQImport)

	// Register question generation handler
	mux.HandleFunc(types.TypeQuestionGeneration, params.KnowledgeService.ProcessQuestionGeneration)

	// Register summary generation handler
	mux.HandleFunc(types.TypeSummaryGeneration, params.KnowledgeService.ProcessSummaryGeneration)

	// Register KB clone handler
	mux.HandleFunc(types.TypeKBClone, params.KnowledgeService.ProcessKBClone)

	// Register knowledge move handler
	mux.HandleFunc(types.TypeKnowledgeMove, params.KnowledgeService.ProcessKnowledgeMove)

	// Register knowledge list delete handler
	mux.HandleFunc(types.TypeKnowledgeListDelete, params.KnowledgeService.ProcessKnowledgeListDelete)

	// Register index delete handler
	mux.HandleFunc(types.TypeIndexDelete, params.TagService.ProcessIndexDelete)

	// Register KB delete handler
	mux.HandleFunc(types.TypeKBDelete, params.KnowledgeBaseService.ProcessKBDelete)

	// Register image multimodal handler
	mux.HandleFunc(types.TypeImageMultimodal, params.ImageMultimodal.Handle)

	// Register video multimodal handler
	mux.HandleFunc(types.TypeVideoMultimodal, params.VideoMultimodal.Handle)

	// Register data source sync handler
	mux.HandleFunc(types.TypeDataSourceSync, params.DataSourceService.ProcessSync)

	go func() {
		// Start the server
		if err := params.Server.Run(mux); err != nil {
			log.Fatalf("could not run server: %v", err)
		}
	}()
	return mux
}
