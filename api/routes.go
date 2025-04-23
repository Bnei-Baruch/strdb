package api

import "github.com/gin-gonic/gin"

func SetupRoutes(router *gin.Engine) {
	router.GET("/test/:file", getData)
	router.GET("/get/:file", getFile)
	router.GET("/files/list", getFilesList)
}
