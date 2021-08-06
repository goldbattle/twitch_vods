package twitch

import (
	"errors"
	"github.com/nicklaw5/helix"
	"log"
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
	err := errors.New("startup")
	respStreams := &helix.StreamsResponse{}
	for i := 1; i < 5; i++ {
		respStreams, err = client.GetStreams(&helix.StreamsParams{
			UserIDs: []string{usernameId},
			First:   5,
		})
		if err == nil {
			break
		}
		log.Printf("ERROR: stream api call failed %s (try %d)\n", err, i)
	}
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

	err := errors.New("startup")
	respVideos := &helix.VideosResponse{}
	for i := 1; i < 5; i++ {
		respVideos, err = client.GetVideos(&helix.VideosParams{
			UserID: usernameId,
			First:  1,
			Sort:   "time",
		})
		if err == nil {
			break
		}
		log.Printf("ERROR: vod api call failed %s (try %d)\n", err, i)
	}
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
