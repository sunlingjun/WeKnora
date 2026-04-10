//go:build !milvus

package container

import (
	"slices"

	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/sirupsen/logrus"
)

func registerMilvusRetrieverEngine(retrieveDriver []string, _ interfaces.RetrieveEngineRegistry, log *logrus.Entry) {
	if slices.Contains(retrieveDriver, "milvus") {
		log.Warn("RETRIEVE_DRIVER includes milvus but binary is built without 'milvus' tag; skipping milvus retriever")
	}
}
