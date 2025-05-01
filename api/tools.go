package api

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"
)

type Server struct {
	Name     string `json:"name"`
	DNS      string `json:"dns"`
	Sessions int    `json:"sessions"`
	Enable   bool   `json:"enable"`
	Online   bool   `json:"online"`
}

type Config map[string]Server

var (
	StrDB Config
	mutex sync.RWMutex
)

func getJson() (*Config, error) {
	req, err := http.NewRequest("GET", viper.GetString("server.cfg_url"), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	conf := Config{}
	err = json.Unmarshal(body, &conf)
	if err != nil {
		return nil, err
	}

	return &conf, nil
}

func InitConf() error {
	strdb, err := getJson()
	if err != nil {
		strdb, err = getConf()
	}

	if err != nil {
		log.Errorf("Get conf error: %s", err)
		return err
	}
	mutex.Lock()
	StrDB = *strdb
	mutex.Unlock()
	return err
}

func getConf() (*Config, error) {
	file, err := os.Open("conf.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	Config := Config{}
	err = decoder.Decode(&Config)

	if err != nil {
		return nil, err
	}
	return &Config, nil
}

func getRandomServer() (string, error) {
	mutex.RLock()
	defer mutex.RUnlock()

	var available []string
	for _, server := range StrDB {
		if server.Online && server.Enable {
			available = append(available, server.Name)
		}
	}

	if len(available) == 0 {
		err := errors.New("getRandomServer: no available servers")
		return "", err
	}

	rand.Seed(time.Now().UnixNano()) // инициализация генератора
	randomIndex := rand.Intn(len(available))
	return available[randomIndex], nil
}

func SetOnline(name string, status bool) {
	mutex.Lock()
	defer mutex.Unlock()

	if server, ok := StrDB[name]; ok {
		server.Online = status
		StrDB[name] = server
	}
}

func PrintServers() {
	mutex.RLock()
	defer mutex.RUnlock()

	for name, server := range StrDB {
		fmt.Printf("%s => %+v\n", name, server)
	}
}
