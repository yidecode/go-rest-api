package httpapi

import (
	"github.com/gin-gonic/gin"

	"github.com/yidecode/go-rest-api/internal/config"
	"github.com/yidecode/go-rest-api/internal/store"
)

// NewRouter wires middleware, public routes and authenticated routes.
func NewRouter(s store.Store, cfg config.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(RequestID(), Logger(), gin.Recovery(), CORS(), RateLimit(cfg.RateLimitRPS))

	h := NewHandler(s, cfg)
	r.GET("/healthz", h.Health)

	v1 := r.Group("/api/v1")
	v1.POST("/auth/register", h.Register)
	v1.POST("/auth/login", h.Login)

	authed := v1.Group("", AuthRequired(cfg.JWTSecret))
	authed.GET("/me", h.Me)
	authed.POST("/tasks", h.CreateTask)
	authed.GET("/tasks", h.ListTasks)
	authed.GET("/tasks/:id", h.GetTask)
	authed.PUT("/tasks/:id", h.UpdateTask)
	authed.DELETE("/tasks/:id", h.DeleteTask)

	return r
}
