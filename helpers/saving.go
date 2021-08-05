package helpers

import (
	"../models"
	"encoding/json"
	"github.com/nicklaw5/helix"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func SaveChatToFile(folder string, username string, usernameId string, vod helix.Video, comments []models.Comments) {

	// Create file / folders if needed to save into
	saveDir := filepath.Join(folder, strings.ToLower(username))
	saveFile := filepath.Join(saveDir, vod.ID+"_live_chat.json")
	err := os.MkdirAll(saveDir, os.ModePerm)
	if err != nil {
		log.Printf("CHAT: error %s", err)
		return
	}

	// Get last message recorded offset
	latestCommentOffset := 0.0
	for _, comment := range comments {
		latestCommentOffset = math.Max(latestCommentOffset, comment.ContentOffsetSeconds)
	}

	// Create data structure to match the twichdownload chat render
	// https://github.com/lay295/TwitchDownloader/blob/master/TwitchDownloaderCore/ChatDownloader.cs#L77
	data := models.ChatRenderStructure{}
	data.Streamer.Name = username
	data.Streamer.ID, _ = strconv.Atoi(usernameId)
	data.Video.Start = 0.0
	data.Video.End = latestCommentOffset
	data.Comments = comments
	data.Emotes.Firstparty = make([]models.Firstparty, 0)
	data.Emotes.Thirdparty = make([]models.Thirdparty, 0)
	file, _ := json.MarshalIndent(data, "", " ")
	_ = ioutil.WriteFile(saveFile, file, 0644)

}
