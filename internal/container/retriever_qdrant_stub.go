//go:build !qdrant

package container

import (
	"slices"

	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/sirupsen/logrus"
)

func registerQdrantRetrieverEngine(retrieveDriver []string, _ interfaces.RetrieveEngineRegistry, log *logrus.Entry) {
	if slices.Contains(retrieveDriver, "qdrant") {
		log.Warn("RETRIEVE_DRIVER includes qdrant but binary is built without 'qdrant' tag; skipping qdrant retriever")
	}
}
