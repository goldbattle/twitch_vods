package twitch

import (
	"../models"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/grafov/m3u8"
	"github.com/nicklaw5/helix"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func GetVodFromStreamId(client *helix.Client, username string, usernameId string, config models.ConfigurationFile, stream helix.Stream) (helix.Video, error) {

	// Try to load from file
	saveDir := filepath.Join(config.SaveDirectory, strings.ToLower(username))
	saveFile := filepath.Join(saveDir, "mapping_stream2vod.json")
	data := &models.MappingStreamToVod{}
	if _, err := os.Stat(saveFile); err == nil {
		file, err := ioutil.ReadFile(saveFile)
		if err != nil {
			return helix.Video{}, errors.New("error loading mapping file")
		}
		err = json.Unmarshal(file, &data)
		if err != nil {
			return helix.Video{}, errors.New("error loading parsing mapping file")
		}
	}
	if data.Data == nil {
		data.Data = map[string]helix.Video{}
	}

	// Check to see if we have it in our vod
	if val, ok := data.Data[stream.ID]; ok {
		return val, nil
	}

	// Else lets try to get most recent vods
	vods, err := GetLatestVods(client, usernameId, 100)
	if err != nil {
		return helix.Video{}, err
	}

	// Loop through and append vods if not there
	for _, vod := range vods {
		if _, ok := data.Data[vod.StreamID]; !ok {
			data.Data[vod.StreamID] = vod
		}
	}

	// Save to file for future use
	err = os.MkdirAll(saveDir, os.ModePerm)
	if err != nil {
		return helix.Video{}, errors.New("unable to create save folder")
	}
	file, _ := json.MarshalIndent(data, "", " ")
	_ = ioutil.WriteFile(saveFile, file, 0644)

	// Check to see if we have it in our vod
	if val, ok := data.Data[stream.ID]; ok {
		return val, nil
	}
	return helix.Video{}, errors.New("unable to find vod id for stream id")

}

func TestIfStreamIsLiveM3U8(username string) error {

	// Now lets try to get the video
	// Query twitch to get our request signature for m3u8 files
	jsonPayload := map[string]string{
		"query": `
           {
			  streamPlaybackAccessToken(channelName: "` + strings.ToLower(username) + `", params: {platform: "web", playerBackend: "mediaplayer", playerType: "site"}) {
				signature
				value
			  }
			}
       `,
	}
	body, err := CallGraphQl("https://gql.twitch.tv/gql", jsonPayload)
	if err != nil {
		return err
	}

	// Convert to the api response
	apiResponse := models.GraphQLStreamPlaybackAccessResponse{}
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		return errors.New("error decoding GQL api endpoint")
	}

	// Call our api endpoint to get the playlist
	baseUrl := "http://usher.twitch.tv/api/channel/hls/" + strings.ToLower(username) + ".m3u8"
	baseUrl += "?sig=" + apiResponse.Data.StreamPlaybackAccessToken.Signature
	baseUrl += "&token=" + apiResponse.Data.StreamPlaybackAccessToken.Value
	baseUrl += "&p=" + strconv.Itoa(rand.Intn(999999))
	baseUrl += "&player=twitchweb&type=any&allow_source=true&playlist_include_framerate=true"
	res, err := http.Get(baseUrl)
	if err != nil {
		return errors.New("error requesting playlist file")
	}
	defer res.Body.Close()

	// Return if not success
	if res.StatusCode != 200 {
		return fmt.Errorf("got status code %d instead of 200", res.StatusCode)
	}

	// Parse the m3u8 playlist
	playlist, listType, err := m3u8.DecodeFrom(res.Body, false)
	if err != nil {
		return errors.New("error decoding m3u8 live file")
	}
	if listType != m3u8.MASTER {
		return errors.New("error not valid m3u8.MASTER file")
	}
	masterPlaylist := playlist.(*m3u8.MasterPlaylist)
	//log.Printf("LIVE: %s - found %d variants", username, len(masterPlaylist.Variants))
	//for idx, variant := range masterPlaylist.Variants {
	//	log.Printf("%d - %s - %s", idx, variant.Resolution, variant.URI)
	//}
	if len(masterPlaylist.Variants) < 1 {
		return errors.New("no valid playlists for the stream found")
	}
	return nil

}
