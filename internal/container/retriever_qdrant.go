//go:build qdrant

package container

import (
	"os"
	"slices"
	"strconv"
	"strings"

	qdrantRepo "github.com/Tencent/WeKnora/internal/application/repository/retriever/qdrant"
	"github.com/Tencent/WeKnora/internal/application/service/retriever"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/qdrant/go-client/qdrant"
	"github.com/sirupsen/logrus"
)

func registerQdrantRetrieverEngine(retrieveDriver []string, registry interfaces.RetrieveEngineRegistry, log *logrus.Entry) {
	if !slices.Contains(retrieveDriver, "qdrant") {
		return
	}

	qdrantHost := os.Getenv("QDRANT_HOST")
	if qdrantHost == "" {
		qdrantHost = "localhost"
	}

	qdrantPort := 6334
	if portStr := os.Getenv("QDRANT_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			qdrantPort = port
		}
	}

	qdrantAPIKey := os.Getenv("QDRANT_API_KEY")
	qdrantUseTLS := false
	if useTLSStr := os.Getenv("QDRANT_USE_TLS"); useTLSStr != "" {
		useTLSLower := strings.ToLower(strings.TrimSpace(useTLSStr))
		qdrantUseTLS = useTLSLower != "false" && useTLSLower != "0"
	}

	log.Infof("Connecting to Qdrant at %s:%d (TLS: %v)", qdrantHost, qdrantPort, qdrantUseTLS)
	client, err := qdrant.NewClient(&qdrant.Config{Host: qdrantHost, Port: qdrantPort, APIKey: qdrantAPIKey, UseTLS: qdrantUseTLS})
	if err != nil {
		log.Errorf("Create qdrant client failed: %v", err)
		return
	}

	qdrantRepository := qdrantRepo.NewQdrantRetrieveEngineRepository(client)
	if err := registry.Register(retriever.NewKVHybridRetrieveEngine(qdrantRepository, types.QdrantRetrieverEngineType)); err != nil {
		log.Errorf("Register qdrant retrieve engine failed: %v", err)
	} else {
		log.Infof("Register qdrant retrieve engine success")
	}
}
