package main

import (
	"./helpers"
	"./threads"
	"./twitch"
	"errors"
	"github.com/nicklaw5/helix"
	"log"
	"os"
	"sync"
	"time"
)

func main() {

	// Load the config
	if len(os.Args) < 2 {
		log.Fatalf("CONFIG: please pass path to config as argument\n")
	}
	log.Printf("CONFIG: loading %s\n", os.Args[1])
	config := helpers.LoadConfigFile(os.Args[1])

	// Create the client
	client, err := helix.NewClient(&helix.Options{
		ClientID:      config.TwitchClientId,
		ClientSecret:  config.TwitchSecretId,
		RateLimitFunc: twitch.RateLimitCallback,
	})
	if err != nil {
		log.Fatalf("%v", err)
	}

	// Initialize methods responsible for refreshing oauth
	waitForFirstAppAccessToken := make(chan struct{})
	go twitch.InitAppAccessToken(client, waitForFirstAppAccessToken)
	<-waitForFirstAppAccessToken

	// Ensure we have channels
	if len(config.ChannelsChat) < 1 {
		log.Fatalf("CONFIG: please specify at least one chat channel to watch\n")
	}

	// Get the user ids for this user
	var usernameIds []string
	for _, username := range config.ChannelsChat {
		user := helix.User{}
		err := errors.New("startup")
		for err != nil {
			user, err = twitch.GetUser(client, username)
			if err != nil {
				log.Printf("ERROR: %s\n", err)
			} else {
				log.Printf("CLIENT: user %s -> %s\n", username, user.ID)
			}
		}
		usernameIds = append(usernameIds, user.ID)
	}

	// Start group
	var wg sync.WaitGroup
	for i := range usernameIds {
		wg.Add(1)
		go func(client *helix.Client, username string, usernameId string, folder string) {
			defer wg.Done()
			for true {
				threads.DownloadLiveChat(client, username, usernameId, folder)
				time.Sleep(5 * time.Minute)
			}
		}(client, config.ChannelsChat[i], usernameIds[i], config.SaveDirectory)
	}

	// Wait for all to complete
	wg.Wait()

}
