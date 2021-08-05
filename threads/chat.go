package threads

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

func DownloadLiveChat(client *helix.Client, username string, usernameId string, folder string) {

	// Check if user live
	stream, err := twitch.GetLatestStream(client, usernameId)
	if err != nil {
		log.Printf("CHAT: %s - error %s\n", username, err)
		return
	}
	log.Printf("CHAT: %s - %s -> %s\n", username, stream.StartedAt, stream.Title)

	// Get our VOD
	vod, err := twitch.GetLatestVodId(client, usernameId)
	if err != nil {
		log.Printf("CHAT: %s - error %s\n", username, err)
		return
	}
	log.Printf("CHAT: %s - vod id %s found\n", username, vod.ID)

	// Now can start downloading the chat
	currentCursor := ""
	isStart := true
	isFirstFullDownload := true
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
			continue
		}
		req.Header.Set("Accept", "application/vnd.twitchtv.twitch+json; charset=UTF-8")
		req.Header.Set("Client-Id", "kimne78kx3ncx6brgo4mv6wki5h1ko")
		res, err := clientComments.Do(req)
		if err != nil {
			log.Printf("CHAT: %s - error %s\n", username, err)
			continue
		}
		defer res.Body.Close()
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Printf("CHAT: %s - error %s\n", username, err)
			continue
		}

		// Convert to the api response
		apiResponse := models.CommentsV5ApiResponse{}
		err = json.Unmarshal(body, &apiResponse)
		if err != nil {
			log.Printf("CHAT: %s - error api response is bad %s\n", username, err)
			continue
		}

		// Move forward in time
		comments = append(comments, apiResponse.Comments...)
		if !isFirstFullDownload {
			log.Printf("CHAT: %s - %d comments (%d in total so far)\n", username, len(apiResponse.Comments), len(comments))
		} else if isStart {
			log.Printf("CHAT: %s - starting mass download of chat history....\n", username)
		}
		isStart = false

		// If same or empty then we should check if we are live
		// If we are live, then we should wait a bit then re-try
		// TODO: should check to see if we have the same vod id still...
		if apiResponse.Next == "" || currentCursor == apiResponse.Next {
			_, err = twitch.GetLatestStream(client, usernameId)
			if err != nil {
				log.Printf("CHAT: %s - stream is offline!!!!\n", username)
				helpers.SaveChatToFile(folder, username, usernameId, vod, comments)
				break
			} else {
				// todo; should fail save and break out if waited too long here....
				// todo; check if vod id is the same? or just if no new messages in 20 minutes, then break out?
				log.Printf("CHAT: %s - stream is live, waiting a bit...\n", username)
				helpers.SaveChatToFile(folder, username, usernameId, vod, comments)
				time.Sleep(4 * time.Minute)
				isFirstFullDownload = false
				continue
			}
		}
		currentCursor = apiResponse.Next

	}

}
