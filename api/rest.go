package api

import (
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"time"
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

func getServer(c *gin.Context) {
	mutex.RLock()
	defer mutex.RUnlock()

	var available []string
	for _, server := range StrDB {
		if server.Online && server.Enable {
			available = append(available, server.Name)
		}
	}

	if len(available) == 0 {
		c.AbortWithStatus(http.StatusNotFound)
	}

	rand.Seed(time.Now().UnixNano()) // инициализация генератора
	randomIndex := rand.Intn(len(available))
	c.JSON(http.StatusOK, gin.H{"server": available[randomIndex]})
}

func getFile(c *gin.Context) {
	file := c.Params.ByName("file")

	http.ServeFile(c.Writer, c.Request, viper.GetString("workflow.capture_path")+file)
}
