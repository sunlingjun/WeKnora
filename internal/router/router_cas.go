package router

import (
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterCASRoutes 注册 CAS 认证相关的路由（不需要认证中间件）
func RegisterCASRoutes(r *gin.Engine, handler *handler.CASAuthHandler) {
	cas := r.Group("/api/v1/cas")
	{
		cas.GET("/validate", handler.ValidateCASSession)
	}
}
