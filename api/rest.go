package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type User struct {
	Display    string   `json:"display"`
	Email      string   `json:"email"`
	Roles      []string `json:"roles"`
	ID         string   `json:"id"`
	Username   string   `json:"username"`
	FamilyName string   `json:"familyname"`
	Role       string   `json:"role"`
	IsClient   bool     `json:"isClient"`
	VHInfo     VHInfo   `json:"vhinfo"`
	Allowed    bool     `json:"allowed"`
	System     string   `json:"system"`
	Extra      Extra    `json:"extra"`
	Geo        Geo      `json:"geo"`
	IP         string   `json:"ip"`
	Country    string   `json:"country"`
	Room       int      `json:"room"`
	Janus      string   `json:"janus"`
	Group      string   `json:"group"`
	Camera     bool     `json:"camera"`
	Question   bool     `json:"question"`
	Timestamp  int64    `json:"timestamp"`
	Session    int64    `json:"session"`
	Handle     int64    `json:"handle"`
	RFID       int64    `json:"rfid"`
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
	}

	srv, err := getBestServer()
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
	}

	if err != nil {
		NewInternalError(err).Abort(c)
	} else {
		c.JSON(http.StatusOK, gin.H{"server": srv})
	}
}
