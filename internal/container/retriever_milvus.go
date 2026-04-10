//go:build milvus

package container

import (
	"context"
	"os"
	"slices"
	"time"

	milvusRepo "github.com/Tencent/WeKnora/internal/application/repository/retriever/milvus"
	"github.com/Tencent/WeKnora/internal/application/service/retriever"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func registerMilvusRetrieverEngine(retrieveDriver []string, registry interfaces.RetrieveEngineRegistry, log *logrus.Entry) {
	if !slices.Contains(retrieveDriver, "milvus") {
		return
	}

	milvusCfg := milvusclient.ClientConfig{DialOptions: []grpc.DialOption{grpc.WithTimeout(5 * time.Second)}}
	milvusAddress := os.Getenv("MILVUS_ADDRESS")
	if milvusAddress == "" {
		milvusAddress = "localhost:19530"
	}
	milvusCfg.Address = milvusAddress
	if v := os.Getenv("MILVUS_USERNAME"); v != "" {
		milvusCfg.Username = v
	}
	if v := os.Getenv("MILVUS_PASSWORD"); v != "" {
		milvusCfg.Password = v
	}
	if v := os.Getenv("MILVUS_DB_NAME"); v != "" {
		milvusCfg.DBName = v
	}

	milvusCli, err := milvusclient.New(context.Background(), &milvusCfg)
	if err != nil {
		log.Errorf("Create milvus client failed: %v", err)
		return
	}

	milvusRepository := milvusRepo.NewMilvusRetrieveEngineRepository(milvusCli)
	if err := registry.Register(retriever.NewKVHybridRetrieveEngine(milvusRepository, types.MilvusRetrieverEngineType)); err != nil {
		log.Errorf("Register milvus retrieve engine failed: %v", err)
	} else {
		log.Infof("Register milvus retrieve engine success")
	}
}
