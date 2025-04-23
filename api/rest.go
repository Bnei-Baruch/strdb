package api

import (
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"io/ioutil"
	"net/http"
	"os"
)

func getFilesList(c *gin.Context) {

	var list []string
	files, err := ioutil.ReadDir(viper.GetString("workflow.capture_path"))
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
	}

	for _, f := range files {
		list = append(list, f.Name())
	}

	c.JSON(http.StatusOK, list)
}

func getData(c *gin.Context) {
	file := c.Params.ByName("file")

	if _, err := os.Stat(viper.GetString("workflow.capture_path") + file); os.IsNotExist(err) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": "success"})
}

func getFile(c *gin.Context) {
	file := c.Params.ByName("file")

	http.ServeFile(c.Writer, c.Request, viper.GetString("workflow.capture_path")+file)
}
