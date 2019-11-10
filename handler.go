package main

import (
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func healthHandler(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

func defaultHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"release":      release,
		"request_on":   time.Now(),
		"request_from": c.Request.RemoteAddr,
	})
}

// Notification represents stackdriver notification
type Notification struct {
	Incident struct {
		IncidentID    string `json:"incident_id"`
		ResourceID    string `json:"resource_id"`
		ResourceName  string `json:"resource_name"`
		StartedAt     int    `json:"started_at"`
		PolicyName    string `json:"policy_name"`
		ConditionName string `json:"condition_name"`
		URL           string `json:"url"`
		Documentation struct {
			Content  string `json:"content"`
			MimeType string `json:"mime_type"`
		} `json:"documentation"`
		State   string `json:"state"`
		EndedAt int    `json:"ended_at"`
		Summary string `json:"summary"`
	} `json:"incident"`
	Version string `json:"version"`
}

func notifHandler(c *gin.Context) {

	if debug == 1 {
		contentBytes, _ := ioutil.ReadAll(c.Request.Body)
		logger.Println(string(contentBytes))
	}

	token := c.Param("token")
	if accessToken != token {
		logger.Println("invalid access token")
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Invalid access token",
			"status":  "Unauthorized",
		})
		return
	}

	var notif Notification
	if bindErr := c.BindJSON(&notif); bindErr != nil {
		logger.Printf("error binding notification: %v", bindErr)
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid notification format",
			"status":  "BadRequest",
		})
		return
	}

	logger.Printf("notification: %v", notif)

	insertedCount, err := pump("", "", "")
	if err != nil {
		logger.Printf("Error on pump exec: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error processing request, see logs",
			"status":  "InternalServerError",
		})
		return
	}

	logger.Printf("Inserted %d records", insertedCount)

	c.JSON(http.StatusOK, gin.H{
		"message": "Success",
		"status":  "OK",
	})
}
