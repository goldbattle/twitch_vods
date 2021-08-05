package twitch

import (
	"errors"
	"github.com/nicklaw5/helix"
)

func GetUser(client *helix.Client, username string) (helix.User, error) {

	// Get this user's information so we can get their id
	respUser, err := client.GetUsers(&helix.UsersParams{
		Logins: []string{username},
	})
	if err != nil {
		return helix.User{}, err
	}
	if len(respUser.Data.Users) != 1 {
		return helix.User{}, errors.New("no known user")
	}
	return respUser.Data.Users[0], nil

}

func GetLatestStream(client *helix.Client, usernameId string) (helix.Stream, error) {

	// Get the streams for this user
	respStreams, err := client.GetStreams(&helix.StreamsParams{
		UserIDs: []string{usernameId},
		First:   5,
	})
	if err != nil {
		return helix.Stream{}, err
	}
	if len(respStreams.Data.Streams) < 1 {
		return helix.Stream{}, errors.New("no live streams")
	}
	//for _, video := range respStreams.Data.Streams {
	//	fmt.Printf("%s - %s - %s\n", video.StartedAt, video.ID, video.Title)
	//}
	return respStreams.Data.Streams[0], nil
}

func GetLatestVodId(client *helix.Client, usernameId string) (helix.Video, error) {

	// Get videos for this specific user
	respVideos, err := client.GetVideos(&helix.VideosParams{
		UserID: usernameId,
		First:  1,
		Sort:   "time",
	})
	if err != nil {
		return helix.Video{}, err
	}
	if len(respVideos.Data.Videos) < 1 {
		return helix.Video{}, errors.New("no vod returned")
	}
	//for _, video := range respVideos.Data.Videos {
	//	fmt.Printf("%s - %s - %s\n", video.CreatedAt, video.ID, video.Title)
	//}
	return respVideos.Data.Videos[0], nil

}
