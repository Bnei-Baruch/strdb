package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Server struct {
	Name     string `json:"name"`
	DNS      string `json:"dns"`
	Sessions int    `json:"sessions"`
	Enable   bool   `json:"enable"`
	Online   bool   `json:"online"`
	Region   string `json:"region"` // Region restriction, e.g., "RU" for Russia-only servers
}

type Config map[string]Server

var (
	StrDB Config
	mutex sync.RWMutex
	rnd   *rand.Rand
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
	// Initialize random generator once
	rnd = rand.New(rand.NewSource(time.Now().UnixNano()))

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

func getBestServer() (string, error) {
	return getBestServerForCountry("")
}

func getBestServerForCountry(countryCode string) (string, error) {
	mutex.RLock()
	defer mutex.RUnlock()

	var available []Server

	// Filter servers based on country code and region restrictions
	for _, server := range StrDB {
		if !server.Online || !server.Enable {
			continue
		}

		// Universal region filtering logic:
		// 1. Servers with empty region ("") are available for all countries
		// 2. Servers with specific region (e.g., "RU", "CN") are available ONLY for that country
		// 3. Clients from specific country get: their regional servers + global servers (region=="")

		if server.Region == "" {
			// Global server - available for everyone
			available = append(available, server)
		} else if server.Region == countryCode {
			// Regional server matching client's country
			available = append(available, server)
		}
		// Otherwise skip - this server is for a different region
	}

	if len(available) == 0 {
		err := errors.New("getBestServerForCountry: no available servers")
		log.WithFields(log.Fields{
			"country_code": countryCode,
		}).Error(err)
		return "", err
	}

	// Find server with minimum sessions
	minSessions := available[0].Sessions
	minSessionsServers := []Server{available[0]}

	for _, server := range available[1:] {
		if server.Sessions < minSessions {
			minSessions = server.Sessions
			minSessionsServers = []Server{server}
		} else if server.Sessions == minSessions {
			minSessionsServers = append(minSessionsServers, server)
		}
	}

	// If we have multiple servers with the same minimum sessions, choose randomly
	randomIndex := rnd.Intn(len(minSessionsServers))
	selectedServer := minSessionsServers[randomIndex]

	log.WithFields(log.Fields{
		"server":       selectedServer.Name,
		"dns":          selectedServer.DNS,
		"sessions":     selectedServer.Sessions,
		"region":       selectedServer.Region,
		"country_code": countryCode,
	}).Debug("Selected server for request")

	return selectedServer.Name, nil
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
