package algos

import (
	"bufio"
	"encoding/json"
	"fmt"
	twitchirc "github.com/gempir/go-twitch-irc/v3"
	"github.com/goldbattle/twitch_vods/models"
	"github.com/goldbattle/twitch_vods/twitch"
	"github.com/nicklaw5/helix"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func DownloadStreamLiveStreamLink(client *helix.Client, username string, usernameId string, config models.ConfigurationFile) {

	// Our data structures
	stream := helix.Stream{}
	vod := helix.Video{}

	// Check if we have a stream that is live
	//err1 := twitch.TestIfStreamIsLiveM3U8(username)
	stream, err := twitch.GetLatestStream(client, usernameId)
	if err != nil {
		log.Printf("LIVE: %s - %s | %s\n", username, err)
		return
	}

	// Convert the stream id to the vod id that we will save into
	vod, errVod := twitch.GetVodFromStreamId(client, username, usernameId, config, stream)
	if errVod == nil {
		log.Printf("LIVE: %s - stream id = %s | vod id = %s", username, stream.ID, vod.ID)
	} else {
		log.Printf("LIVE: %s - stream id = %s | vod id = unknown", username, stream.ID)
	}

	// VOD metadata
	metaData := models.StreamMetaData{}
	if errVod == nil {
		metaData.Id = vod.ID
		metaData.Url = "https://www.twitch.tv/videos/" + vod.ID
	}
	metaData.IdStream = stream.ID
	metaData.UserId = stream.UserID
	metaData.UserName = stream.UserName
	metaData.Title = stream.Title
	metaData.Game = stream.GameName
	metaData.Views = -1
	metaData.RecordedAt = stream.StartedAt
	metaData.Titles = make([]models.Moment, 0)
	metaData.Moments = make([]models.Moment, 0)
	metaData.MutedSegments = make([]interface{}, 0)

	// Master id we will use
	ID := stream.ID
	if errVod == nil {
		ID = vod.ID
	}

	// Create the current moments
	currentMomentGameTime := time.Now()
	currentMomentGame := models.Moment{}
	currentMomentGame.Id = stream.GameID
	currentMomentGame.Name = stream.GameName
	currentMomentGame.Offset = 0
	currentMomentGame.Duration = 0
	currentMomentGame.Type = "GAME_CHANGE"
	currentMomentTitleTime := time.Now()
	currentMomentTitle := models.Moment{}
	currentMomentTitle.Name = stream.Title
	currentMomentTitle.Offset = 0
	currentMomentTitle.Duration = 0
	currentMomentTitle.Type = "TITLE_CHANGE"

	// Parse VOD date
	tm := stream.StartedAt
	if errVod == nil {
		tm, _ = time.Parse("2006-01-02T15:04:05Z", vod.CreatedAt)
	}
	yearFolder := strconv.Itoa(tm.Year()) + "-" + fmt.Sprintf("%02d", int(tm.Month()))

	// Create file / folders if needed to save into
	saveDir := filepath.Join(config.SaveDirectory, strings.ToLower(username), yearFolder)
	err = os.MkdirAll(saveDir, os.ModePerm)
	if err != nil {
		log.Printf("LIVE: %s - error %s", username, err)
		return
	}

	// Loop through and try to create a valid
	fileCounter := 0
	filePrefix := ID + "_" + fmt.Sprintf("%03d", fileCounter)
	pathVideo := filepath.Join(saveDir, filePrefix+".mp4")
	pathVideoTmp := filepath.Join(saveDir, filePrefix+".tmp.mp4")
	for true {
		_, err1 := os.Stat(pathVideo)
		_, err2 := os.Stat(pathVideoTmp)
		if os.IsNotExist(err1) && os.IsNotExist(err2) {
			break
		}
		fileCounter++
		filePrefix = ID + "_" + fmt.Sprintf("%03d", fileCounter)
		pathVideo = filepath.Join(saveDir, filePrefix+".mp4")
		pathVideoTmp = filepath.Join(saveDir, filePrefix+".tmp.mp4")
	}
	//pathVideo, _ = filepath.Abs(pathVideo)
	//pathVideoTmp, _ = filepath.Abs(pathVideoTmp)

	// Open our log file writter
	pathLog := filepath.Join(saveDir, filePrefix+"_streamlink.log")
	logfile, err := os.Create(pathLog)
	if err != nil {
		log.Printf("LIVE: %s - error %s\n", username, err)
		return
	}
	defer logfile.Close()
	logfileWriter := bufio.NewWriter(logfile)
	defer logfileWriter.Flush()
	log.Printf("LIVE: %s - %s\n", username, pathVideo)

	// Write the video info the file
	pathInfoJson := filepath.Join(saveDir, filePrefix+"_info.json")
	file, _ := json.MarshalIndent(metaData, "", " ")
	_ = ioutil.WriteFile(pathInfoJson, file, 0644)

	// Chat file writer
	pathIrcChat := filepath.Join(saveDir, filePrefix+"_irc.log")
	pathIrcChatJson := filepath.Join(saveDir, filePrefix+"_chat.json")
	fileIrc, err := os.Create(pathIrcChat)
	if err != nil {
		log.Printf("LIVE: %s - error %s\n", username, err)
		return
	}
	defer fileIrc.Close()

	// Our comment object we will save to file and its multi-threaded mutex
	ircChatMutex := sync.Mutex{}
	ircChatComments := []models.Comments{}
	ircChatLastSaveTime := time.Now()

	// Start download of live chat
	ircStartTime := time.Now()
	ircClient := twitchirc.NewAnonymousClient()
	ircClient.OnPrivateMessage(func(message twitchirc.PrivateMessage) {

		// Create the VOD comment!
		comment := models.Comments{}
		comment.Id = message.ID
		comment.CreatedAt = message.Time
		comment.UpdatedAt = message.Time
		comment.ChannelId = message.RoomID
		comment.ContentType = "video"
		if errVod == nil {
			comment.ContentId = vod.ID
		}
		comment.ContentOffsetSeconds = time.Since(ircStartTime).Seconds()
		comment.Commenter.DisplayName = message.User.DisplayName
		comment.Commenter.Id = message.User.ID
		comment.Commenter.Name = message.User.Name
		comment.Commenter.Type = "user"
		comment.Source = "chat"
		comment.State = "published"
		comment.MoreReplies = false
		comment.Message.Body = message.Message
		comment.Message.BitsSpent = message.Bits
		comment.Message.IsAction = message.Action
		if len(message.User.Color) > 0 {
			comment.Message.UserColor = &message.User.Color
		}

		// Loop through all user badges (sub, mod, etc..)
		for id, ver := range message.User.Badges {
			userbadge := models.UserBadge{}
			userbadge.Id = id
			userbadge.Version = strconv.Itoa(ver)
			comment.Message.UserBadges = append(comment.Message.UserBadges, userbadge)
		}

		// Our emotes provide their ids, name, along with positions in the message
		for _, emote := range message.Emotes {
			for _, pos := range emote.Positions {
				tmp := models.Emoticon{}
				tmp.Id = emote.ID
				tmp.Begin = pos.Start
				tmp.End = pos.End
				comment.Message.Emoticons = append(comment.Message.Emoticons, tmp)
			}
		}

		// Loop through our message, and try to split it into fragments
		currentEmote := -1
		fragCurrent := models.Fragment{}
		for pos, ch := range comment.Message.Body {
			// find what emote index this current char should be
			newEmote := -1
			for e, emote := range comment.Message.Emoticons {
				if pos >= emote.Begin && pos <= emote.End {
					newEmote = e
					break
				}
			}
			// loop through all emotes and see if next char should be in an emote
			if newEmote != currentEmote {
				if pos != 0 {
					comment.Message.Fragments = append(comment.Message.Fragments, fragCurrent)
				}
				fragCurrent = models.Fragment{}
				if newEmote > 0 {
					fragCurrent.Emoticon = &models.EmoticonFragment{}
					fragCurrent.Emoticon.EmoticonId = comment.Message.Emoticons[newEmote].Id
				}
				currentEmote = newEmote
			}
			// append the current string
			fragCurrent.Text += string(ch)

		}
		comment.Message.Fragments = append(comment.Message.Fragments, fragCurrent)

		// Append to comment array
		// Create data structure to match the twichdownload chat render
		// https://github.com/lay295/TwitchDownloader/blob/master/TwitchDownloaderCore/ChatDownloader.cs#L77
		ircChatMutex.Lock()
		defer ircChatMutex.Unlock()
		ircChatComments = append(ircChatComments, comment)
		if time.Since(ircChatLastSaveTime) > 3*time.Minute {
			data := models.ChatRenderStructure{}
			data.Streamer.Name = username
			data.Streamer.ID, _ = strconv.Atoi(usernameId)
			data.Video.Start = 0.0
			data.Video.End = time.Since(ircStartTime).Seconds()
			data.Comments = ircChatComments
			data.Emotes.Firstparty = make([]models.Firstparty, 0)
			data.Emotes.Thirdparty = make([]models.Thirdparty, 0)
			file, _ := json.MarshalIndent(data, "", " ")
			_ = ioutil.WriteFile(pathIrcChatJson, file, 0644)
			ircChatLastSaveTime = time.Now()
		}
		_, _ = fileIrc.Write([]byte(message.Raw + "\n"))
	})
	ircClient.OnUserNoticeMessage(func(message twitchirc.UserNoticeMessage) {

		// Create the VOD comment!
		comment := models.Comments{}
		comment.Id = message.ID
		comment.CreatedAt = message.Time
		comment.UpdatedAt = message.Time
		comment.ChannelId = message.RoomID
		comment.ContentType = "video"
		if errVod == nil {
			comment.ContentId = vod.ID
		}
		comment.ContentOffsetSeconds = time.Since(ircStartTime).Seconds()
		comment.Commenter.DisplayName = message.User.DisplayName
		comment.Commenter.Id = message.User.ID
		comment.Commenter.Name = message.User.Name
		comment.Commenter.Type = "user"
		comment.Source = "chat"
		comment.State = "published"
		comment.MoreReplies = false
		comment.Message.Body = message.SystemMsg + " " + message.Message
		comment.Message.BitsSpent = 0
		comment.Message.IsAction = false
		if len(message.User.Color) > 0 {
			comment.Message.UserColor = &message.User.Color
		}
		comment.Message.UserNoticeParams = models.UserNoticeParams{MsgId: &message.MsgID}

		// Loop through all user badges (sub, mod, etc..)
		for id, ver := range message.User.Badges {
			userbadge := models.UserBadge{}
			userbadge.Id = id
			userbadge.Version = strconv.Itoa(ver)
			comment.Message.UserBadges = append(comment.Message.UserBadges, userbadge)
		}

		// Our emotes provide their ids, name, along with positions in the message
		for _, emote := range message.Emotes {
			for _, pos := range emote.Positions {
				tmp := models.Emoticon{}
				tmp.Id = emote.ID
				tmp.Begin = pos.Start
				tmp.End = pos.End
				comment.Message.Emoticons = append(comment.Message.Emoticons, tmp)
			}
		}

		// Loop through our message, and try to split it into fragments
		currentEmote := -1
		fragCurrent := models.Fragment{}
		for pos, ch := range comment.Message.Body {
			// find what emote index this current char should be
			newEmote := -1
			for e, emote := range comment.Message.Emoticons {
				if pos >= emote.Begin && pos <= emote.End {
					newEmote = e
					break
				}
			}
			// loop through all emotes and see if next char should be in an emote
			if newEmote != currentEmote {
				if pos != 0 {
					comment.Message.Fragments = append(comment.Message.Fragments, fragCurrent)
				}
				fragCurrent = models.Fragment{}
				if newEmote > 0 {
					fragCurrent.Emoticon = &models.EmoticonFragment{}
					fragCurrent.Emoticon.EmoticonId = comment.Message.Emoticons[newEmote].Id
				}
				currentEmote = newEmote
			}
			// append the current string
			fragCurrent.Text += string(ch)
		}
		comment.Message.Fragments = append(comment.Message.Fragments, fragCurrent)

		// Append to comment array
		// Create data structure to match the TwitchDownload chat render
		// https://github.com/lay295/TwitchDownloader/blob/master/TwitchDownloaderCore/ChatDownloader.cs#L77
		ircChatMutex.Lock()
		defer ircChatMutex.Unlock()
		ircChatComments = append(ircChatComments, comment)
		if time.Since(ircChatLastSaveTime) > 3*time.Minute {
			data := models.ChatRenderStructure{}
			data.Streamer.Name = username
			data.Streamer.ID, _ = strconv.Atoi(usernameId)
			data.Video.Start = 0.0
			data.Video.End = time.Since(ircStartTime).Seconds()
			data.Comments = ircChatComments
			data.Emotes.Firstparty = make([]models.Firstparty, 0)
			data.Emotes.Thirdparty = make([]models.Thirdparty, 0)
			file, _ := json.MarshalIndent(data, "", " ")
			_ = ioutil.WriteFile(pathIrcChatJson, file, 0644)
			ircChatLastSaveTime = time.Now()
		}
		_, _ = fileIrc.Write([]byte(message.Raw + "\n"))
	})
	ircClient.Join(username)
	go func() {
		// wait till video file has been created
		for true {
			if _, err := os.Stat(pathVideoTmp); err == nil {
				break
			}
			time.Sleep(250 * time.Millisecond)
		}
		// start recording our chat messages
		// will return an error on disconnect that we can just ignore
		ircStartTime = time.Now()
		currentMomentGameTime = time.Now()
		currentMomentTitleTime = time.Now()
		_ = ircClient.Connect()
	}()

	// Open our streamlink!
	cmd := exec.Command(config.Streamlink, "twitch.tv/"+username, "best", "--loglevel",
		"debug", "-o", pathVideoTmp, "--twitch-disable-hosting", "--twitch-disable-ads", "--twitch-disable-reruns", "--twitch-ttvlol", "--twitch-proxy-playlist-fallback")
	cmd.Stdout = logfileWriter
	cmd.Stdout = logfileWriter
	err = cmd.Start()
	if err != nil {
		log.Printf("LIVE: %s - error %s\n", username, err)
		return
	}

	// Create a listener for the sigterm to close our threads
	// https://gist.github.com/uudashr/3cf820e3ba902d3c6387abc82c815e66
	gracefullSigterm := false
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		//log.Printf("LIVE: %s - stream ended reqested!\n", username)
		_ = cmd.Process.Kill()
		gracefullSigterm = true
	}()

	// Create listener for game and title changes
	// We will record any changes to the metadata info file!
	go func() {
		for !gracefullSigterm {
			stream, err := twitch.GetLatestStream(client, usernameId)
			if err == nil {
				doSave := false
				if currentMomentGame.Name != stream.GameName {
					// append
					currentMomentGame.Duration = int(time.Since(currentMomentGameTime).Seconds())
					metaData.Moments = append(metaData.Moments, currentMomentGame)
					// create new one
					currentMomentGame = models.Moment{}
					currentMomentGame.Id = stream.GameID
					currentMomentGame.Name = stream.GameName
					currentMomentGame.Offset = int(time.Since(currentMomentGameTime).Seconds())
					currentMomentGame.Duration = 0
					currentMomentGame.Type = "GAME_CHANGE"
					currentMomentGameTime = time.Now()
					doSave = true
					log.Println("NEW GAME")
					log.Println(currentMomentGame)
				}
				if currentMomentTitle.Name != stream.Title {
					// append
					currentMomentTitle.Duration = int(time.Since(currentMomentTitleTime).Seconds())
					metaData.Titles = append(metaData.Titles, currentMomentTitle)
					// create new one
					currentMomentTitle = models.Moment{}
					currentMomentTitle.Name = stream.Title
					currentMomentTitle.Offset = int(time.Since(currentMomentTitleTime).Seconds())
					currentMomentTitle.Duration = 0
					currentMomentTitle.Type = "TITLE_CHANGE"
					currentMomentTitleTime = time.Now()
					doSave = true
					log.Println("NEW TITLE")
					log.Println(currentMomentTitle)
				}
				metaData.Views = int(math.Max(float64(metaData.Views), float64(stream.ViewerCount)))
				metaData.Duration = time.Since(ircStartTime).String()
				if doSave {
					file, _ = json.MarshalIndent(metaData, "", " ")
					_ = ioutil.WriteFile(pathInfoJson, file, 0644)
				}
			}
			if !gracefullSigterm {
				time.Sleep(time.Duration(config.QueryLiveMin) * time.Minute)
			}
		}
	}()

	// Seems to exit with a status 1, when the stream ends...
	// Not sure if something that we can fix in streamlink, or just assume it has been ok...
	_ = cmd.Wait()
	_ = ircClient.Disconnect()
	log.Printf("LIVE: %s - stream has ended (%s)\n", username, time.Since(ircStartTime).String())

	// Save the chat one more time
	data := models.ChatRenderStructure{}
	data.Streamer.Name = username
	data.Streamer.ID, _ = strconv.Atoi(usernameId)
	data.Video.Start = 0.0
	data.Video.End = time.Since(ircStartTime).Seconds()
	data.Comments = ircChatComments
	data.Emotes.Firstparty = make([]models.Firstparty, 0)
	data.Emotes.Thirdparty = make([]models.Thirdparty, 0)
	file, _ = json.MarshalIndent(data, "", " ")
	_ = ioutil.WriteFile(pathIrcChatJson, file, 0644)

	// Save the vod info (append the current moment also!)
	currentMomentGame.Duration = int(time.Since(currentMomentGameTime).Seconds())
	metaData.Moments = append(metaData.Moments, currentMomentGame)
	currentMomentTitle.Duration = int(time.Since(currentMomentTitleTime).Seconds())
	metaData.Titles = append(metaData.Titles, currentMomentTitle)
	file, _ = json.MarshalIndent(metaData, "", " ")
	_ = ioutil.WriteFile(pathInfoJson, file, 0644)

	// ffmpeg clean the video file
	timeConversion := time.Now()
	cmd = exec.Command(config.Ffmpeg, "-err_detect", "ignore_err", "-i", pathVideoTmp, "-c", "copy", pathVideo)
	//log.Println(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stdout = os.Stdout
	err = cmd.Start()
	if err != nil {
		log.Printf("LIVE: %s - ffmpeg start error %s\n", username, err)
		return
	}
	err = cmd.Wait()
	if err != nil {
		log.Printf("LIVE: %s - ffmpeg error %s\n", username, err)
		return
	}
	log.Printf("LIVE: %s - ffpmeg converted stream in %s!\n", username, time.Since(timeConversion).String())
	os.Remove(pathVideoTmp)

}
