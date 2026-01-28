package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// FlexibleInt can unmarshal from both JSON number and JSON string
type FlexibleInt int

func (fi *FlexibleInt) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as number first
	var num int
	if err := json.Unmarshal(data, &num); err == nil {
		*fi = FlexibleInt(num)
		return nil
	}

	// If failed, try as string
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	// Convert string to int
	if str == "" {
		*fi = 0
		return nil
	}

	num, err := strconv.Atoi(str)
	if err != nil {
		return err
	}

	*fi = FlexibleInt(num)
	return nil
}

// FlexibleInt64 can unmarshal from both JSON number and JSON string
type FlexibleInt64 int64

func (fi *FlexibleInt64) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as number first
	var num int64
	if err := json.Unmarshal(data, &num); err == nil {
		*fi = FlexibleInt64(num)
		return nil
	}

	// If failed, try as string
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	// Convert string to int64
	if str == "" {
		*fi = 0
		return nil
	}

	num, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return err
	}

	*fi = FlexibleInt64(num)
	return nil
}

type User struct {
	Display    string        `json:"display"`
	Email      string        `json:"email"`
	Roles      []string      `json:"roles"`
	ID         string        `json:"id"`
	Username   string        `json:"username"`
	FamilyName string        `json:"familyname"`
	Role       string        `json:"role"`
	IsClient   bool          `json:"isClient"`
	VHInfo     VHInfo        `json:"vhinfo"`
	Allowed    bool          `json:"allowed"`
	System     string        `json:"system"`
	Extra      Extra         `json:"extra"`
	Geo        Geo           `json:"geo"`
	IP         string        `json:"ip"`
	Country    string        `json:"country"`
	Room       FlexibleInt   `json:"room"`
	Janus      string        `json:"janus"`
	Group      string        `json:"group"`
	Camera     bool          `json:"camera"`
	Question   bool          `json:"question"`
	Timestamp  int64         `json:"timestamp"`
	Session    int64         `json:"session"`
	Handle     int64         `json:"handle"`
	RFID       FlexibleInt64 `json:"rfid"`
}

type VHInfo struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
}

type Extra struct {
	Streams []Stream `json:"streams"`
	IsGroup bool     `json:"isGroup"`
}

type Stream struct {
	Type        string `json:"type"`
	MIndex      int    `json:"mindex"`
	MID         string `json:"mid"`
	Codec       string `json:"codec"`
	H264Profile string `json:"h264_profile,omitempty"` // Только для видео
	FEC         bool   `json:"fec,omitempty"`          // Только для аудио
}

type Geo struct {
	CountryCode string `json:"country_code"`
	City        string `json:"city"`
	Region      string `json:"region"`
}

func getStatus(c *gin.Context) {
	mutex.RLock()
	defer mutex.RUnlock()

	c.JSON(http.StatusOK, StrDB)
}

func getServer(c *gin.Context) {
	srv, err := getBestServer()
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
	}
	c.JSON(http.StatusOK, gin.H{"server": srv})
}

func getServerByID(c *gin.Context) {
	t := &User{}
	err := c.BindJSON(&t)
	if err != nil {
		NewBadRequestError(err).Abort(c)
		return
	}

	// Get country code from Geo data
	countryCode := t.Geo.CountryCode

	// Log client request details
	log.WithFields(log.Fields{
		"username":     t.Username,
		"email":        t.Email,
		"ip":           t.IP,
		"country":      t.Country,
		"country_code": countryCode,
		"city":         t.Geo.City,
		"region":       t.Geo.Region,
		"room":         int(t.Room),
		"rfid":         int64(t.RFID),
	}).Info("Client requesting server")

	srv, err := getBestServerForCountry(countryCode)
	if err != nil {
		log.WithFields(log.Fields{
			"username":     t.Username,
			"country_code": countryCode,
			"error":        err.Error(),
		}).Error("Failed to get server for client")
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	log.WithFields(log.Fields{
		"username":        t.Username,
		"country_code":    countryCode,
		"assigned_server": srv,
	}).Info("Server assigned to client")

	c.JSON(http.StatusOK, gin.H{"server": srv})
}
