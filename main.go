package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/heroku/babycheck/storage"
	_ "github.com/heroku/x/hmetrics/onload"
)

func pong(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}

func action(c *gin.Context) {
	action := c.Param("action")
	time := time.Now()
	tmp, _ := c.Get("storage")
	store := tmp.(*storage.Storage)
	ok := store.Update(action, time)
	c.JSON(http.StatusOK, gin.H{
		"action": action,
		"ok":     ok,
	})
}

func search(c *gin.Context) {
	// start and stop are body params
	body := struct {
		Start int64 `json:"start"`
		Stop  int64 `json:"stop"`
	}{}
	err := c.BindJSON(&body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid json",
		})
		return
	}
	tmp, _ := c.Get("storage")
	store := tmp.(*storage.Storage)
	events := store.Search(body.Start, body.Stop)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to search",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"events": events,
	})
}

func AddAction(c *gin.Context) {
	var action struct {
		Name string `json:"action"`
		Time int64  `json:"timestamp"`
	}
	err := c.BindJSON(&action)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid json",
		})
		return
	}
	tmp, _ := c.Get("storage")
	store := tmp.(*storage.Storage)
	ok := store.Save(action.Time, action.Name)
	c.JSON(http.StatusOK, gin.H{
		"action": action,
		"ok":     ok,
	})
}

type DeleteAction struct {
	Timestamp int64 `json:"timestamp"`
}

func deleteAction(c *gin.Context) {
	payload := DeleteAction{}
	err := c.BindJSON(&payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid json",
		})
		return
	}
	tmp, _ := c.Get("storage")
	store := tmp.(*storage.Storage)
	ok := store.Delete(payload.Timestamp)
	c.JSON(http.StatusOK, gin.H{
		"action":  action,
		"deleted": ok,
		"ok":      ok,
	})
}

func getMode(c *gin.Context) {
	mode := os.Getenv("GIN_MODE")
	c.JSON(http.StatusOK, gin.H{
		"mode": mode,
	})
}

func changeTimestamp(c *gin.Context) {
	payload := struct {
		Event storage.DBBabyEvent `json:"event"`
		Ts    int64               `json:"ts"`
	}{}
	err := c.BindJSON(&payload)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid json",
		})
		return
	}
	tmp, _ := c.Get("storage")
	store := tmp.(*storage.Storage)
	ok := store.ChangeTimestamp(payload.Event, payload.Ts)
	c.JSON(http.StatusOK, gin.H{
		"action":  action,
		"changed": ok,
	})
}

func main() {
	port := os.Getenv("PORT")

	if port == "" {
		log.Fatal("$PORT must be set")
	}

	router := gin.New()
	logger := gin.Logger()
	router.Use(logger)
	// router.LoadHTMLGlob("templates/*.tmpl.html")
	router.Use(static.Serve("/", static.LocalFile("./static", true)))

	redisConnector := storage.NewStorage()
	if redisConnector == nil {
		log.Fatal("failed to create storage")
	}

	// api group
	// allow cors if dev env
	fmt.Println("GIN_MODE: ", os.Getenv("GIN_MODE"))
	if os.Getenv("GIN_MODE") != "release" {
		fmt.Println("CORS enabled")
		router.Use(func(c *gin.Context) {
			fmt.Println("CORS middleware processing")
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
			// return 200 for options
			if c.Request.Method == "OPTIONS" {
				c.JSON(http.StatusOK, gin.H{})
				return
			}
		})

	}
	api := router.Group("/api")
	{
		api.Use(func(c *gin.Context) {
			c.Set("storage", redisConnector)
			c.Next()
		})
		api.GET("/ping", pong)
		api.POST("/search", search)
		api.POST("/remote/:action", action)
		api.POST("/remote/update", changeTimestamp)
		api.POST("/add", AddAction)
		api.DELETE("/remote", deleteAction)
		api.GET("/mode", getMode)
	}
	if os.Getenv("GIN_MODE") != "release" {
		api.GET("/reset", func(c *gin.Context) {
			tmp, _ := c.Get("storage")
			store := tmp.(*storage.Storage)
			store.Erase()
			c.JSON(http.StatusOK, gin.H{
				"message": "reset",
			})
		})
	}

	router.Run(":" + port)
}
