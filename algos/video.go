package algos

import (
	"encoding/json"
	"fmt"
	"github.com/goldbattle/twitch_vods/helpers"
	"github.com/goldbattle/twitch_vods/models"
	"github.com/goldbattle/twitch_vods/twitch"
	"github.com/grafov/m3u8"
	"github.com/nicklaw5/helix"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func DownloadVodLatest(client *helix.Client, username string, usernameId string, config models.ConfigurationFile) {

	// Get our VODs
	vods, err := twitch.GetLatestVods(client, usernameId, config.DownloadNum)
	if err != nil {
		log.Printf("VIDEO: %s - error %s\n", username, err)
		return
	}

	// For each vod lets download it
	for ct, vod := range vods {
		log.Printf("VIDEO: %s - vod id %s downloading (%d/%d)\n", username, vod.ID, ct, config.DownloadNum)
		DownloadVod(client, username, usernameId, config, vod)
	}

}

func DownloadVod(client *helix.Client, username string, usernameId string, config models.ConfigurationFile, vod helix.Video) {

	// Skip if already downloaded
	// NOTE: We will still try to download recent vods (to make sure we get everything)
	// NOTE: Thus we will add the start time to the duration to get the time the vod ends
	// NOTE: We can then compare that to our current time (UTC) and see if it could have been recently updated
	tm0 := time.Now().UTC()
	tm1, _ := time.Parse("2006-01-02T15:04:05Z", vod.CreatedAt)
	tm1Dur, _ := time.ParseDuration(vod.Duration)
	diff := tm0.Sub(tm1.Add(tm1Dur))
	if helpers.IsVodDownloaded(config.SaveDirectory, username, usernameId, vod) && int(diff.Minutes()) > config.SkipIfOlderMin {
		log.Printf("VIDEO: %s - vod %s, skipping (updated %d min ago)\n", username, vod.ID, int(diff.Minutes()))
		return
	}

	// Query twitch to get our request signature for m3u8 files
	jsonPayload := map[string]string{
		"query": `
            {
			  videoPlaybackAccessToken(id: ` + vod.ID + `, params: {platform: "web", playerBackend: "mediaplayer", playerType: "site"}) {
				signature
				value
			  }
			}
        `,
	}
	body, err := twitch.CallGraphQl("https://gql.twitch.tv/gql", jsonPayload)
	if err != nil {
		log.Printf("VIDEO: %s - error %s\n", username, err)
		return
	}

	// Convert to the api response
	apiResponse := models.GraphQLVideoPlaybackAccessResponse{}
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		log.Printf("VIDEO: %s - error api response is bad %s\n", username, err)
		return
	}

	// Call our api endpoint
	baseUrl := "http://usher.twitch.tv/vod/" + vod.ID
	baseUrl += "?nauth=" + apiResponse.Data.VideoPlaybackAccessToken.Value
	baseUrl += "&nauthsig=" + apiResponse.Data.VideoPlaybackAccessToken.Signature
	baseUrl += "&allow_source=true&player=twitchweb"
	res, err := http.Get(baseUrl)
	if err != nil {
		log.Printf("VIDEO: %s - error %s\n", username, err)
		return
	}
	defer res.Body.Close()

	// Parse the m3u8 playlist
	playlist, listType, err := m3u8.DecodeFrom(res.Body, false)
	if err != nil {
		log.Printf("VIDEO: %s - error %s\n", username, err)
		return
	}
	if listType != m3u8.MASTER {
		log.Printf("VIDEO: %s - error playlist is not m3u8.MASTER\n", username)
		return
	}
	masterPlaylist := playlist.(*m3u8.MasterPlaylist)
	log.Printf("VIDEO: %s - found %d variants", username, len(masterPlaylist.Variants))
	indexVideo := -1
	cleanResolution := strings.ReplaceAll(config.VideoResolution, "p", "")
	for idx, variant := range masterPlaylist.Variants {
		if strings.Contains(variant.Resolution, cleanResolution) {
			indexVideo = idx
			break
		}
	}
	if indexVideo == -1 {
		log.Printf("VIDEO: %s - unable to find requested %s res in vod playlist\n", username, config.VideoResolution)
		return
	}
	//log.Printf("VIDEO: %s - resolution = %s\n", username, masterPlaylist.Variants[indexVideo].Resolution)
	//log.Printf("VIDEO: %s - url = %s\n", username, masterPlaylist.Variants[indexVideo].URI)
	masterPlaylistUri := masterPlaylist.Variants[indexVideo].URI

	// Call our api endpoint
	res, err = http.Get(masterPlaylistUri)
	if err != nil {
		log.Printf("VIDEO: %s - error %s\n", username, err)
		return
	}
	defer res.Body.Close()

	// Parse the m3u8 playlist
	playlist, listType, err = m3u8.DecodeFrom(res.Body, false)
	if err != nil {
		log.Printf("VIDEO: %s - error %s\n", username, err)
		return
	}
	if listType != m3u8.MEDIA {
		log.Printf("VIDEO: %s - error playlist is not m3u8.MEDIA\n", username)
		return
	}
	segmentPlaylist := playlist.(*m3u8.MediaPlaylist)
	log.Printf("VIDEO: %s - found %d video segments", username, len(segmentPlaylist.Segments))

	// Parse VOD date
	tm, _ := time.Parse("2006-01-02T15:04:05Z", vod.CreatedAt)
	yearFolder := strconv.Itoa(tm.Year()) + "-" + fmt.Sprintf("%02d", int(tm.Month()))

	// Create file / folders if needed to save into
	saveDir := filepath.Join(config.SaveDirectory, strings.ToLower(username), yearFolder, vod.ID)
	err = os.MkdirAll(saveDir, os.ModePerm)
	if err != nil {
		log.Printf("VIDEO: %s - error %s", username, err)
		return
	}

	// Count total valid segments (non-null)
	countTotalSegments := 0
	for _, segment := range segmentPlaylist.Segments {
		if segment != nil {
			countTotalSegments++
		}
	}
	if countTotalSegments < 1 {
		log.Printf("VIDEO: %s - no segments to download....", username)
		return
	}
	log.Printf("VIDEO: %s - found %d video VALID segments", username, countTotalSegments)

	// Download segments we don't already have
	for idx, segment := range segmentPlaylist.Segments {

		// Skip invalid / end segments
		if segment == nil {
			continue
		}

		// Check the file segment on disk
		// Also use this to check if the file exists
		saveFile := filepath.Join(saveDir, segment.URI)
		fi, err := os.Stat(saveFile)

		// Skip if file exists and has data
		if err == nil && fi.Size() > 0 {
			continue
		}
		if err == nil && fi.Size() <= 0 {
			log.Printf("VIDEO: deleting bad file %s", segment.URI)
			_ = os.Remove(saveFile)
		}
		log.Printf("VIDEO: %s - downloading %s (%d / %d)", username, segment.URI, idx, countTotalSegments)

		// Download and save to file
		segmentRemoteUri := masterPlaylistUri[0:strings.LastIndex(masterPlaylistUri, "/")] + "/" + segment.URI
		resp, err := http.Get(segmentRemoteUri)
		if err != nil {
			log.Printf("VIDEO: error %s", err)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			log.Printf("VIDEO: %s - invalid response code: %d", username, resp.StatusCode)
			continue
		}

		// Create local file and write to it
		out, err := os.Create(saveFile)
		if err != nil {
			log.Printf("VIDEO: %s - error %s", username, err)
			continue
		}
		defer out.Close()
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			log.Printf("VIDEO: %s - error %s", username, err)
			continue
		}

	}

	/// Done :)
	log.Printf("VIDEO: %s - done downloading video segments!!!", username)

}
