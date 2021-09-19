package models

type ConfigurationFile struct {
	TwitchClientId  string   `json:"twitch_client_id"`
	TwitchSecretId  string   `json:"twitch_secret_id"`
	SaveDirectory   string   `json:"save_directory"`
	Streamlink      string   `json:"streamlink"`
	Ffmpeg          string   `json:"ffmpeg"`
	VideoResolution string   `json:"video_resolution"`
	DownloadNum     int      `json:"download_num"`
	SkipIfOlderMin  int      `json:"skip_if_older_min"`
	ChannelsChat    []string `json:"channels_chat"`
	ChannelsVideo   []string `json:"channels_video"`
	ChannelsLive    []string `json:"channels_live"`
	QueryVodsMin    int      `json:"query_vods_min"`
	QueryLiveMin    int      `json:"query_live_min"`
}
