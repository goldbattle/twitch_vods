package algos

import (
	"../helpers"
	"../models"
	"../twitch"
	"encoding/json"
	"github.com/nicklaw5/helix"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

func DownloadChatLatest(client *helix.Client, username string, usernameId string, config models.ConfigurationFile) {

	// Get our VODs
	vods, err := twitch.GetLatestVods(client, usernameId, config.DownloadNum)
	if err != nil {
		log.Printf("CHAT: %s - error %s\n", username, err)
		return
	}

	// For each vod lets download it
	for ct, vod := range vods {
		log.Printf("CHAT: %s - vod id %s downloading (%d/%d)\n", username, vod.ID, ct+1, config.DownloadNum)
		DownloadChat(client, username, usernameId, config, vod)
	}

}

func DownloadChat(client *helix.Client, username string, usernameId string, config models.ConfigurationFile, vod helix.Video) {

	// Skip if already downloaded
	// NOTE: We will still try to download recent vods (to make sure we get everything)
	// NOTE: Thus we will add the start time to the duration to get the time the vod ends
	// NOTE: We can then compare that to our current time (UTC) and see if it could have been recently updated
	tm0 := time.Now().UTC()
	tm1, _ := time.Parse("2006-01-02T15:04:05Z", vod.CreatedAt)
	tm1Dur, _ := time.ParseDuration(vod.Duration)
	diff := tm0.Sub(tm1.Add(tm1Dur))
	if helpers.IsChatDownloaded(config.SaveDirectory, username, usernameId, vod) && int(diff.Minutes()) > config.SkipIfOlderMin {
		log.Printf("CHAT: %s - vod %s, skipping (updated %d min ago)\n", username, vod.ID, int(diff.Minutes()))
		return
	}

	// Now can start downloading the chat
	currentCursor := ""
	isStart := true
	hasError := false
	var comments []models.Comments
	for currentCursor != "" || isStart {

		// Call our api endpoint
		baseUrl := "https://api.twitch.tv/v5/videos/" + vod.ID + "/comments"
		if currentCursor != "" {
			baseUrl += "?cursor=" + currentCursor
		} else {
			baseUrl += "?content_offset_seconds=0"
		}
		clientComments := &http.Client{}
		req, err := http.NewRequest("GET", baseUrl, nil)
		if err != nil {
			log.Printf("CHAT: %s - error %s\n", username, err)
			hasError = true
			break
		}
		req.Header.Set("Accept", "application/vnd.twitchtv.twitch+json; charset=UTF-8")
		req.Header.Set("Client-Id", "kimne78kx3ncx6brgo4mv6wki5h1ko")
		res, err := clientComments.Do(req)
		if err != nil {
			log.Printf("CHAT: %s - error %s\n", username, err)
			hasError = true
			break
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Printf("CHAT: %s - error %s\n", username, err)
			hasError = true
			break
		}

		// Convert to the api response
		apiResponse := models.CommentsV5ApiResponse{}
		err = json.Unmarshal(body, &apiResponse)
		if err != nil {
			log.Printf("CHAT: %s - error api response is bad %s\n", username, err)
			hasError = true
			break
		}

		// Move forward in time
		comments = append(comments, apiResponse.Comments...)
		currentCursor = apiResponse.Next
		isStart = false

	}

	// If we have chat messages then save to file
	if len(comments) > 0 && !hasError {
		helpers.SaveChatToFile(config.SaveDirectory, username, usernameId, vod, comments)
	}

}
