package main

import (
	"io/ioutil"
	"net/http"
	"strings"
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
		IncidentID   string `json:"incident_id"`
		ResourceID   string `json:"resource_id"`
		ResourceName string `json:"resource_name"`
		Resource     struct {
			Type   string `json:"type"`
			Labels struct {
				SubscriptionID string `json:"subscription_id"`
			} `json:"labels"`
		} `json:"resource"`
		StartedAt     int    `json:"started_at"`
		PolicyName    string `json:"policy_name"`
		ConditionName string `json:"condition_name"`
		URL           string `json:"url"`
		State         string `json:"state"`
		EndedAt       int    `json:"ended_at"`
		Summary       string `json:"summary"`
	} `json:"incident"`
	Version string `json:"version"`
}

func notifHandler(c *gin.Context) {

	if debug == 1 {
		contentBytes, _ := ioutil.ReadAll(c.Request.Body)
		logger.Println(string(contentBytes))
	}

	token := strings.TrimSpace(c.Query("token"))
	if token != accessToken {
		logger.Printf("invalid access token. Got:%s Want:%s", token, accessToken)
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

	if notif.Incident.Resource.Labels.SubscriptionID != subName {
		logger.Printf("invalid subscription. Got:%s Want:%s",
			notif.Incident.Resource.Labels.SubscriptionID, subName)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Invalid incident subscriptionID",
			"status":  "InternalServerError",
		})
		return
	}

	insertedCount, err := pump()
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
