package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Server struct {
	Name       string   `json:"name"`
	DNS        string   `json:"dns"`
	Sessions   int      `json:"sessions"`
	Enable     bool     `json:"enable"`
	Online     bool     `json:"online"`
	Region     string   `json:"region"`             // Region restriction, e.g., "RU" for Russia-only servers
	Networks   []string `json:"networks,omitempty"` // Exclusive client networks (CIDR), e.g., "5.29.112.160/28"
	MissedPing int      `json:"-"`                  // Not serialized - counts missed admin responses
	LastSeen   int64    `json:"last_seen"`          // Unix timestamp of last successful admin response

	nets []*net.IPNet // Parsed Networks
}

// parseNetworks parses CIDR strings of every server. Invalid entries are logged and skipped.
// A server whose Networks list is non-empty stays exclusive even if all entries are invalid,
// so a typo never turns a dedicated server into a public one.
func parseNetworks(conf Config) {
	for name, server := range conf {
		server.nets = nil
		for _, cidr := range server.Networks {
			_, ipNet, err := net.ParseCIDR(strings.TrimSpace(cidr))
			if err != nil {
				log.WithFields(log.Fields{
					"server":  name,
					"network": cidr,
					"error":   err.Error(),
				}).Error("Invalid network in server config, skipped")
				continue
			}
			server.nets = append(server.nets, ipNet)
		}
		conf[name] = server
	}
}

// matchNetwork returns the server network containing ip, or nil.
func (s Server) matchNetwork(ip net.IP) *net.IPNet {
	if ip == nil {
		return nil
	}
	for _, n := range s.nets {
		if n.Contains(ip) {
			return n
		}
	}
	return nil
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
	parseNetworks(*strdb)
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
	return getBestServerForClient("", "")
}

func getBestServerForClient(countryCode string, clientIP string) (string, error) {
	mutex.RLock()
	defer mutex.RUnlock()

	ip := net.ParseIP(strings.TrimSpace(clientIP))

	var available []Server
	var networkServers []Server
	var regionalServers []Server
	var globalServers []Server
	var matchedNetworks []string

	// Filter servers based on client network, country code and region restrictions
	for _, server := range StrDB {
		if !server.Online || !server.Enable {
			continue
		}

		if len(server.Networks) > 0 {
			// Network-dedicated server: exclusive to clients from its networks
			if n := server.matchNetwork(ip); n != nil {
				networkServers = append(networkServers, server)
				matchedNetworks = append(matchedNetworks, n.String())
			}
			continue
		}

		if server.Region == "" {
			// Global server
			globalServers = append(globalServers, server)
		} else if server.Region == countryCode {
			// Regional server matching client's country
			regionalServers = append(regionalServers, server)
		}
		// Otherwise skip - this server is for a different region
	}

	// Logic: client network servers first, then regional servers for the country,
	// otherwise global servers
	var poolType string
	if len(networkServers) > 0 {
		available = networkServers
		poolType = "network"
		log.WithFields(log.Fields{
			"client_ip":        clientIP,
			"country_code":     countryCode,
			"network_servers":  len(networkServers),
			"matched_networks": matchedNetworks,
			"pool_type":        poolType,
		}).Info("Using network server pool (client IP matched dedicated network)")
	} else if len(regionalServers) > 0 {
		// Country has dedicated regional servers - use only those
		available = regionalServers
		poolType = "regional"
		log.WithFields(log.Fields{
			"country_code":     countryCode,
			"regional_servers": len(regionalServers),
			"global_servers":   len(globalServers),
			"pool_type":        poolType,
		}).Info("Using regional server pool (global servers excluded)")
	} else {
		// No regional servers for this country - use global servers
		available = globalServers
		poolType = "global"
		log.WithFields(log.Fields{
			"country_code":     countryCode,
			"regional_servers": 0,
			"global_servers":   len(globalServers),
			"pool_type":        poolType,
		}).Info("Using global server pool (no regional servers for this country)")
	}

	if len(available) == 0 {
		err := errors.New("getBestServerForClient: no available servers")
		log.WithFields(log.Fields{
			"country_code": countryCode,
		}).Error(err)
		return "", err
	}

	// Build list of available server names for logging
	var availableNames []string
	for _, s := range available {
		availableNames = append(availableNames, fmt.Sprintf("%s(%d)", s.Name, s.Sessions))
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

	// Build list of candidates with minimum sessions
	var candidateNames []string
	for _, s := range minSessionsServers {
		candidateNames = append(candidateNames, s.Name)
	}

	// If we have multiple servers with the same minimum sessions, choose randomly
	randomIndex := rnd.Intn(len(minSessionsServers))
	selectedServer := minSessionsServers[randomIndex]

	selectionReason := "minimum sessions"
	if len(minSessionsServers) > 1 {
		selectionReason = fmt.Sprintf("random from %d servers with minimum sessions", len(minSessionsServers))
	}

	log.WithFields(log.Fields{
		"client_ip":         clientIP,
		"country_code":      countryCode,
		"pool_type":         poolType,
		"available_servers": availableNames,
		"min_sessions":      minSessions,
		"candidates":        candidateNames,
		"selected_server":   selectedServer.Name,
		"server_dns":        selectedServer.DNS,
		"server_sessions":   selectedServer.Sessions,
		"server_region":     selectedServer.Region,
		"selection_reason":  selectionReason,
	}).Info("Server selected for client")

	return selectedServer.Name, nil
}

func SetOnline(name string, status bool) {
	mutex.Lock()
	defer mutex.Unlock()

	if server, ok := StrDB[name]; ok {
		if server.Online != status {
			log.WithFields(log.Fields{
				"server":     name,
				"old_status": server.Online,
				"new_status": status,
			}).Info("Server status changed via MQTT")
		}
		server.Online = status
		if status {
			server.MissedPing = 0
		}
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
