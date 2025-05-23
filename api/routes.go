package api

import "github.com/gin-gonic/gin"

func SetupRoutes(router *gin.Engine) {
	router.GET("/server", getServer)
	router.GET("/status", getStatus)
	router.POST("/server", getServerByID)
}
