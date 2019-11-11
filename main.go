package main

import (
	"log"
	"net"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mchmarny/gcputil/env"
	"github.com/mchmarny/gcputil/project"
)

var (
	//service
	logger    = log.New(os.Stdout, "[PUMP] ", 0)
	port      = env.MustGetEnvVar("PORT", "8080")
	release   = env.MustGetEnvVar("RELEASE", "v0.0.1-default")
	debug     = env.MustGetIntEnvVar("DEBUG", 0)
	projectID = project.GetIDOrFail()

	accessToken = strings.TrimSpace(env.MustGetEnvVar("TOKEN", ""))
	maxStall    = env.MustGetIntEnvVar("MAX_STALL", 30)
	maxDuration = env.MustGetIntEnvVar("MAX_DURATION", 900)
	batchSize   = env.MustGetIntEnvVar("BATCH_SIZE", 100)
	subName     = strings.TrimSpace(env.MustGetEnvVar("SUB", ""))
	dsName      = strings.TrimSpace(env.MustGetEnvVar("DATSET", ""))
	tblName     = strings.TrimSpace(env.MustGetEnvVar("TABLE", ""))
)

func main() {

	gin.SetMode(gin.ReleaseMode)

	// router
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// simple routes
	r.GET("/", defaultHandler)
	r.GET("/health", healthHandler)

	// api
	v1 := r.Group("/v1")
	{
		v1.POST("/notif", notifHandler)
	}

	// server
	hostPort := net.JoinHostPort("0.0.0.0", port)
	logger.Printf("Server starting: %s \n", hostPort)
	if err := r.Run(hostPort); err != nil {
		logger.Fatal(err)
	}
}
