package models

type ConfigurationFile struct {
	TwitchClientId  string   `json:"twitch_client_id"`
	TwitchSecretId  string   `json:"twitch_secret_id"`
	SaveDirectory   string   `json:"save_directory"`
	VideoResolution string   `json:"video_resolution"`
	ChannelsChat    []string `json:"channels_chat"`
	ChannelsVideo   []string `json:"channels_video"`
}
