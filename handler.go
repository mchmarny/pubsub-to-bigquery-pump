package main

import (
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
		IncidentID    string      `json:"incident_id"`
		ResourceID    string      `json:"resource_id"`
		ResourceName  string      `json:"resource_name"`
		State         string      `json:"state"`
		StartedAt     int         `json:"started_at"`
		EndedAt       interface{} `json:"ended_at"`
		PolicyName    string      `json:"policy_name"`
		ConditionName string      `json:"condition_name"`
		URL           string      `json:"url"`
		Summary       string      `json:"summary"`
	} `json:"incident"`
	Version string `json:"version"`
}

func notifHandler(c *gin.Context) {

	// get PumpJob instance from HTTP notification
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
	c.JSON(http.StatusOK, gin.H{
		"message": "Success",
		"status":  "OK",
	})
}
