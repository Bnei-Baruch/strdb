package api

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"io/ioutil"
	"net/http"
	"os"
)

type Config struct {
	Name        string
	Ip          string
	Description string
	Services    []Service
}

type Service struct {
	ID          string
	Name        string
	Description string
	Args        []string
}

func getJson(ep string) (*Config, error) {
	req, err := http.NewRequest("GET", viper.GetString("server.cfg_url")+ep, nil)
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

func logTail(fname string) {
	file, err := os.Open(fname)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	buf := make([]byte, 215)
	stat, err := os.Stat(fname)
	start := stat.Size() - 215
	_, err = file.ReadAt(buf, start)
	if err == nil {
		fmt.Printf("%s\n", buf)
	}

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
	//fmt.Println(Config)
	if err != nil {
		return nil, err
	}
	return &Config, nil
}

func removeProgress(file string) {
	_, err := os.Stat(file)
	if err == nil {
		e := os.Remove(file)
		if e != nil {
			fmt.Printf("%s\n", e)
		}
	}
}
