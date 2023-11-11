package main

import (
	"errors"
	"github.com/goldbattle/twitch_vods/algos"
	"github.com/goldbattle/twitch_vods/helpers"
	"github.com/goldbattle/twitch_vods/models"
	"github.com/goldbattle/twitch_vods/twitch"
	"github.com/nicklaw5/helix"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
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
	if len(config.ChannelsLive) < 1 {
		log.Fatalf("CONFIG: please specify at least one chat channel to watch\n")
	}

	// Get the user ids for this user
	var usernames []string
	var usernameIds []string
	var shouldDownloadVideo []bool
	for _, username := range config.ChannelsLive {
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
		usernames = append(usernames, username)
		usernameIds = append(usernameIds, user.ID)
		shouldDownloadVideo = append(shouldDownloadVideo, true)
	}
	for _, username := range config.ChannelsLiveChat {
		// Check to see if we are already recording the video+chat
		found := false
		for _, usernameLive := range config.ChannelsLive {
			if strings.Contains(usernameLive, username) {
				found = true
			}
		}
		if found {
			continue
		}
		// Else this is an additional user, so append it
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
		usernames = append(usernames, username)
		usernameIds = append(usernameIds, user.ID)
		shouldDownloadVideo = append(shouldDownloadVideo, false)
	}

	// Create a listener for the sigterm to close our threads
	// https://gist.github.com/uudashr/3cf820e3ba902d3c6387abc82c815e66
	gracefullSigterm := false
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		gracefullSigterm = true
	}()

	// Start group
	var wg sync.WaitGroup
	for i := range usernameIds {
		wg.Add(1)
		go func(client *helix.Client, username string, usernameId string, downloadVideo bool, config models.ConfigurationFile) {
			defer wg.Done()
			for !gracefullSigterm {
				//algos.DownloadStreamLive(client, username, usernameId, config)
				algos.DownloadStreamLiveStreamLink(client, username, usernameId, downloadVideo, config)
				if !gracefullSigterm {
					time.Sleep(time.Duration(config.QueryLiveMin) * time.Minute)
				}
			}
		}(client, usernames[i], usernameIds[i], shouldDownloadVideo[i], config)
	}

	// Wait for all to complete
	wg.Wait()

}
