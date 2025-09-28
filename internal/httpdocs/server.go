package httpdocs

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/melkior/nodestatus/internal/redisstore"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Server struct {
	engine *gin.Engine
	store  *redisstore.Store
}

func NewServer(store *redisstore.Store) *Server {
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())

	s := &Server{
		engine: engine,
		store:  store,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.engine.GET("/healthz", s.healthHandler)
	s.engine.GET("/readyz", s.readinessHandler)

	s.engine.StaticFile("/openapi.json", "./gen/openapiv2/openapi.swagger.json")

	s.engine.GET("/docs/*any", ginSwagger.WrapHandler(swaggerfiles.Handler, ginSwagger.URL("/openapi.json")))
}

func (s *Server) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

func (s *Server) readinessHandler(c *gin.Context) {
	ctx := c.Request.Context()
	if _, err := s.store.ListNodes(ctx, 0, 0, 0, 1); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}

func (s *Server) Run(addr string) error {
	return s.engine.Run(addr)
}