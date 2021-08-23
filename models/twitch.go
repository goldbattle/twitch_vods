package models

import "time"

type CommentsV5ApiResponse struct {
	Comments []Comments `json:"comments"`
	Next     string     `json:"_next"`
}

type Comments struct {
	ID                   string                 `json:"_id"`
	CreatedAt            time.Time              `json:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at"`
	ChannelID            string                 `json:"channel_id"`
	ContentType          string                 `json:"content_type"`
	ContentID            string                 `json:"content_id"`
	ContentOffsetSeconds float64                `json:"content_offset_seconds"`
	Commenter            map[string]interface{} `json:"commenter"`
	Source               string                 `json:"source"`
	State                string                 `json:"state"`
	Message              map[string]interface{} `json:"message"`
	MoreReplies          bool                   `json:"more_replies"`
}

//type Commenter struct {
//	DisplayName string      `json:"display_name"`
//	ID          string      `json:"_id"`
//	Name        string      `json:"name"`
//	Type        string      `json:"type"`
//	Bio         interface{} `json:"bio"`
//	CreatedAt   time.Time   `json:"created_at"`
//	UpdatedAt   time.Time   `json:"updated_at"`
//	Logo        string      `json:"logo"`
//}
//type Fragments struct {
//	Text string `json:"text"`
//}
//type UserBadges struct {
//	ID      string `json:"_id"`
//	Version string `json:"version"`
//}
//type UserNoticeParams struct {
//}
//type Message struct {
//	Body             string           `json:"body"`
//	Fragments        []Fragments      `json:"fragments"`
//	IsAction         bool             `json:"is_action"`
//	UserBadges       []UserBadges     `json:"user_badges"`
//	UserColor        string           `json:"user_color"`
//	UserNoticeParams UserNoticeParams `json:"user_notice_params"`
//}

type GraphQLVideoPlaybackAccessResponse struct {
	Data struct {
		VideoPlaybackAccessToken struct {
			Signature string `json:"signature"`
			Value     string `json:"value"`
		} `json:"videoPlaybackAccessToken"`
	} `json:"data"`
}

type GraphQLStreamPlaybackAccessResponse struct {
	Data struct {
		StreamPlaybackAccessToken struct {
			Signature string `json:"signature"`
			Value     string `json:"value"`
		} `json:"streamPlaybackAccessToken"`
	} `json:"data"`
	Extensions struct {
		DurationMilliseconds int    `json:"durationMilliseconds"`
		RequestID            string `json:"requestID"`
	} `json:"extensions"`
}
