package algos

import (
	"../models"
	"../twitch"
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/grafov/m3u8"
	"github.com/nicklaw5/helix"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	saveDir := filepath.Join(config.SaveDirectory, strings.ToLower(username), yearFolder, vod.ID)
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
