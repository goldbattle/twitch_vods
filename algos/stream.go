package algos

import (
	"../models"
	"../twitch"
	"bufio"
	"encoding/json"
	"fmt"
	twitchirc "github.com/gempir/go-twitch-irc"
	"github.com/grafov/m3u8"
	"github.com/nicklaw5/helix"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
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

func DownloadStreamLive(client *helix.Client, username string, usernameId string, config models.ConfigurationFile) {

	// Check if we have a stream that is live
	stream, err := twitch.GetLatestStream(client, usernameId)
	if err != nil {
		log.Printf("LIVE: %s - error %s\n", username, err)
		return
	}

	// Convert the stream id to the vod id that we will save into
	vod, err := twitch.GetVodFromStreamId(client, username, usernameId, config, stream)
	if err != nil {
		log.Printf("LIVE: %s - error %s\n", username, err)
		return
	}
	log.Printf("LIVE: %s - stream id = %s", username, stream.ID)
	log.Printf("LIVE: %s - vod id = %s", username, vod.ID)

	// Parse VOD date
	tm, _ := time.Parse("2006-01-02T15:04:05Z", vod.CreatedAt)
	yearFolder := strconv.Itoa(tm.Year()) + "-" + fmt.Sprintf("%02d", int(tm.Month()))

	// Create file / folders if needed to save into
	saveDir := filepath.Join(config.SaveDirectory, strings.ToLower(username), yearFolder, vod.ID+"_live")
	err = os.MkdirAll(saveDir, os.ModePerm)
	if err != nil {
		log.Printf("LIVE: %s - error %s", username, err)
		return
	}

	// Download the playlist if we do not already have it downloaded
	saveFilePlaylist := filepath.Join(saveDir, "playlist.m4u8")
	if _, err := os.Stat(saveFilePlaylist); err != nil {

		// Now lets try to get the video
		// Query twitch to get our request signature for m3u8 files
		jsonPayload := map[string]string{
			"query": `
           {
			  streamPlaybackAccessToken(channelName: "` + strings.ToLower(stream.UserName) + `", params: {platform: "web", playerBackend: "mediaplayer", playerType: "site"}) {
				signature
				value
			  }
			}
       `,
		}
		body, err := twitch.CallGraphQl("https://gql.twitch.tv/gql", jsonPayload)
		if err != nil {
			log.Printf("LIVE: %s - error %s\n", username, err)
			return
		}

		// Convert to the api response
		apiResponse := models.GraphQLStreamPlaybackAccessResponse{}
		err = json.Unmarshal(body, &apiResponse)
		if err != nil {
			log.Printf("LIVE: %s - error api response is bad %s\n", username, err)
			return
		}

		// Call our api endpoint to get the playlist
		baseUrl := "http://usher.twitch.tv/api/channel/hls/" + strings.ToLower(stream.UserName) + ".m3u8"
		baseUrl += "?sig=" + apiResponse.Data.StreamPlaybackAccessToken.Signature
		baseUrl += "&token=" + apiResponse.Data.StreamPlaybackAccessToken.Value
		baseUrl += "&p=" + strconv.Itoa(rand.Intn(999999))
		baseUrl += "&player=twitchweb&type=any&allow_source=true&playlist_include_framerate=true"
		res, err := http.Get(baseUrl)
		if err != nil {
			log.Printf("LIVE: %s - error %s\n", username, err)
			return
		}
		defer res.Body.Close()

		// Finally save the playlist to file
		body, err = ioutil.ReadAll(res.Body)
		if err != nil {
			log.Printf("LIVE: %s - error %s\n", username, err)
			return
		}
		err = ioutil.WriteFile(saveFilePlaylist, body, 0644)
		if err != nil {
			log.Printf("LIVE: %s - error %s\n", username, err)
			return
		}
	}

	// Parse the m3u8 playlist
	//playlist, listType, err := m3u8.DecodeFrom(res.Body, false)
	file, err := os.Open(saveFilePlaylist)
	if err != nil {
		log.Printf("LIVE: %s - error %s\n", username, err)
		return
	}
	playlist, listType, err := m3u8.DecodeFrom(bufio.NewReader(file), false)
	if err != nil {
		log.Printf("LIVE: %s - error %s\n", username, err)
		log.Printf("LIVE: %s - deleting bad file %s", username, saveFilePlaylist)
		_ = os.Remove(saveFilePlaylist)
		return
	}
	if listType != m3u8.MASTER {
		log.Printf("LIVE: %s - error playlist is not m3u8.MASTER\n", username)
		log.Printf("LIVE: %s - deleting bad file %s", username, saveFilePlaylist)
		_ = os.Remove(saveFilePlaylist)
		return
	}
	masterPlaylist := playlist.(*m3u8.MasterPlaylist)
	log.Printf("LIVE: %s - found %d variants", username, len(masterPlaylist.Variants))
	indexVideo := -1
	cleanResolution := strings.ReplaceAll(config.VideoResolution, "p", "")
	for idx, variant := range masterPlaylist.Variants {
		if strings.Contains(variant.Resolution, cleanResolution) {
			indexVideo = idx
			break
		}
	}
	if indexVideo == -1 {
		log.Printf("LIVE: %s - unable to find requested %s res in playlist\n", username, config.VideoResolution)
		log.Printf("LIVE: %s - deleting bad file %s", username, saveFilePlaylist)
		_ = os.Remove(saveFilePlaylist)
		return
	}
	//log.Printf("LIVE: %s - resolution = %s\n", username, masterPlaylist.Variants[indexVideo].Resolution)
	//log.Printf("LIVE: %s - url = %s\n", username, masterPlaylist.Variants[indexVideo].URI)
	masterPlaylistUri := masterPlaylist.Variants[indexVideo].URI

	// Now lets query our master playlist for new segments
	// NOTE: the first few segments will be amazon ads, we try to skip those...
	// NOTE: the same playlist should be used to keep the ids the same
	for true {

		// Call our api endpoint
		res, err := http.Get(masterPlaylistUri)
		if err != nil {
			log.Printf("LIVE: %s - error %s\n", username, err)
			return
		}
		defer res.Body.Close()

		// Parse the m3u8 playlist
		playlist, listType, err = m3u8.DecodeFrom(res.Body, false)
		if err != nil {
			log.Printf("LIVE: %s - error %s\n", username, err)
			return
		}
		if listType != m3u8.MEDIA {
			log.Printf("LIVE: %s - error playlist is not m3u8.MEDIA\n", username)
			return
		}
		segmentPlaylist := playlist.(*m3u8.MediaPlaylist)
		//log.Printf("LIVE: %s - found %d video segments", username, len(segmentPlaylist.Segments))

		// Count total valid segments (non-null)
		countTotalSegments := 0
		for _, segment := range segmentPlaylist.Segments {
			if segment != nil {
				countTotalSegments++
			}
		}
		if countTotalSegments < 1 {
			log.Printf("LIVE: %s - no segments to download....", username)
			log.Printf("LIVE: %s - deleting bad file %s", username, saveFilePlaylist)
			_ = os.Remove(saveFilePlaylist)
			return
		}
		//log.Printf("LIVE: %s - found %d video VALID segments", username, countTotalSegments)

		// Download segments we don't already have
		for _, segment := range segmentPlaylist.Segments {

			// Skip invalid / end segments
			if segment == nil {
				continue
			}

			// Try to skip amazon ad segmetns
			if strings.Contains(strings.ToLower(segment.Title), "amazon") {
				log.Printf("LIVE: %s - skipping seg %d, detected as ad (title was %s)", username, segment.SeqId, segment.Title)
				continue
			}

			// Debug print statments
			//log.Println(segment)
			log.Printf("seq id = %d\n", segment.SeqId)
			log.Printf("\t-> title = %s", segment.Title)
			log.Printf("\t-> date = %s", segment.ProgramDateTime)
			//log.Printf("\t-> duration = %f", segment.Duration)
			//log.Printf("\t-> diff with curr = %s", time.Now().UTC().Sub(segment.ProgramDateTime))

			// Check the file segment on disk
			// Also use this to check if the file exists
			ext := filepath.Ext(segment.URI)
			filename := strconv.FormatInt(segment.ProgramDateTime.UnixNano(), 10)
			filename += "_" + strconv.FormatUint(segment.SeqId, 10) + ext
			saveFile := filepath.Join(saveDir, filename)
			fi, err := os.Stat(saveFile)

			// Skip if file exists and has data
			if err == nil && fi.Size() > 0 {
				continue
			}
			if err == nil && fi.Size() <= 0 {
				log.Printf("LIVE: deleting bad file %s", filename)
				_ = os.Remove(saveFile)
			}
			log.Printf("LIVE: %s - downloading seg %d into %s", username, segment.SeqId, filename)

			// Download and save to file
			resp, err := http.Get(segment.URI)
			if err != nil {
				log.Printf("LIVE: error %s", err)
				continue
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				log.Printf("LIVE: %s - invalid response code: %d", username, resp.StatusCode)
				continue
			}

			// Create local file and write to it
			out, err := os.Create(saveFile)
			if err != nil {
				log.Printf("LIVE: %s - error %s", username, err)
				continue
			}
			defer out.Close()
			_, err = io.Copy(out, resp.Body)
			if err != nil {
				log.Printf("LIVE: %s - error %s", username, err)
				continue
			}

		}

		/// Done, sleep for a bit...
		//log.Printf("LIVE: %s - done downloading video segments!!!", username)
		time.Sleep(15 * time.Second)
	}

}

func DownloadStreamLiveStreamLink(client *helix.Client, username string, usernameId string, config models.ConfigurationFile) {

	// Check if we have a stream that is live
	stream, err := twitch.GetLatestStream(client, usernameId)
	if err != nil {
		log.Printf("LIVE: %s - error %s\n", username, err)
		return
	}

	// Convert the stream id to the vod id that we will save into
	vod, err := twitch.GetVodFromStreamId(client, username, usernameId, config, stream)
	if err != nil {
		log.Printf("LIVE: %s - error %s\n", username, err)
		return
	}
	log.Printf("LIVE: %s - stream id = %s", username, stream.ID)
	log.Printf("LIVE: %s - vod id = %s", username, vod.ID)

	// Parse VOD date
	tm, _ := time.Parse("2006-01-02T15:04:05Z", vod.CreatedAt)
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
	filePrefix := vod.ID + "_" + fmt.Sprintf("%03d", fileCounter)
	pathVideo := filepath.Join(saveDir, filePrefix+"_streamlink.mp4")
	pathVideoTmp := filepath.Join(saveDir, filePrefix+"_streamlink.tmp.mp4")
	for true {
		_, err1 := os.Stat(pathVideo)
		_, err2 := os.Stat(pathVideoTmp)
		if os.IsNotExist(err1) && os.IsNotExist(err2) {
			break
		}
		fileCounter++
		filePrefix = vod.ID + "_" + fmt.Sprintf("%03d", fileCounter)
		pathVideo = filepath.Join(saveDir, filePrefix+"_streamlink.mp4")
		pathVideoTmp = filepath.Join(saveDir, filePrefix+"_streamlink.tmp.mp4")
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

	// Chat file writer
	pathIrcChat := filepath.Join(saveDir, filePrefix+"_irc.log")
	pathIrcChatJson := filepath.Join(saveDir, filePrefix+"_irc.json")
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
		comment.ContentId = vod.ID
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
		comment.ContentId = vod.ID
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
		comment.Message.UserNoticeParams = &models.UserNoticeParams{MsgId: message.MsgID}

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
		_ = ircClient.Connect()
	}()

	// Open our streamlink!
	cmd := exec.Command(config.Streamlink, "twitch.tv/"+username, "best", "--loglevel",
		"debug", "-o", pathVideoTmp, "--twitch-disable-hosting", "--twitch-disable-ads", "--twitch-disable-reruns")
	cmd.Stdout = logfileWriter
	cmd.Stdout = logfileWriter
	err = cmd.Start()
	if err != nil {
		log.Printf("LIVE: %s - error %s\n", username, err)
		return
	}

	// Create a listener for the sigterm to close our threads
	// https://gist.github.com/uudashr/3cf820e3ba902d3c6387abc82c815e66
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		log.Printf("LIVE: %s - stream ended reqested!\n", username)
		_ = cmd.Process.Kill()
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
	file, _ := json.MarshalIndent(data, "", " ")
	_ = ioutil.WriteFile(pathIrcChatJson, file, 0644)

	// ffmpeg clean the video file
	//timeConversion := time.Now()
	//cmd = exec.Command(config.Ffmpeg, "-err_detect ignore_err", "-i", pathVideoTmp, "-c copy", pathVideo)
	//log.Println(cmd)
	//cmd.Stdout = os.Stdout
	//cmd.Stdout = os.Stdout
	//err = cmd.Start()
	//if err != nil {
	//	log.Printf("LIVE: %s - error2  %s\n", username, err)
	//	return
	//}
	//err = cmd.Wait()
	//if err != nil {
	//	log.Printf("LIVE: %s - error3 %s\n", username, err)
	//	return
	//}
	//log.Printf("LIVE: %s - done converting stream (%s)!\n", username, time.Since(timeConversion).String())

}
