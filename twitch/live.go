package twitch

import (
	"../models"
	"encoding/json"
	"errors"
	"github.com/nicklaw5/helix"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)


func GetVodFromStreamId(client *helix.Client, username string, usernameId string, config models.ConfigurationFile, stream helix.Stream) (helix.Video, error) {

	// Try to load from file
	saveDir := filepath.Join(config.SaveDirectory, strings.ToLower(username))
	saveFile := filepath.Join(saveDir, "mapping_stream2vod.json")
	data := &models.MappingStreamToVod{}
	if _, err := os.Stat(saveDir); err == nil {
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

