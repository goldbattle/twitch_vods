package models

import "time"

type Comments struct {
	Id                   string    `json:"_id"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	ChannelId            string    `json:"channel_id"`
	ContentType          string    `json:"content_type"`
	ContentId            string    `json:"content_id"`
	ContentOffsetSeconds float64   `json:"content_offset_seconds"`
	Commenter            Commenter `json:"commenter"`
	Source               string    `json:"source"`
	State                string    `json:"state"`
	Message              Message   `json:"message"`
	MoreReplies          bool      `json:"more_replies"`
}

type Commenter struct {
	DisplayName string    `json:"display_name"`
	Id          string    `json:"_id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Bio         *string   `json:"bio"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Logo        *string   `json:"logo"`
}

type Message struct {
	Body             string            `json:"body"`
	BitsSpent        int               `json:"bits_spent"`
	Fragments        []Fragment        `json:"fragments"`
	IsAction         bool              `json:"is_action"`
	UserBadges       []UserBadge       `json:"user_badges"`
	UserColor        *string           `json:"user_color"`
	UserNoticeParams UserNoticeParams `json:"user_notice_params"`
	Emoticons        []Emoticon        `json:"emoticons"`
}

type Fragment struct {
	Text     string            `json:"text"`
	Emoticon *EmoticonFragment `json:"emoticon"`
}
type UserNoticeParams struct {
	MsgId *string `json:"msg-id"`
}

type EmoticonFragment struct {
	EmoticonId    string `json:"emoticon_id"`
	EmoticonSetId string `json:"emoticon_set_id"`
}

type UserBadge struct {
	Id      string `json:"_id"`
	Version string `json:"version"`
}

type Emoticon struct {
	Id    string `json:"_id"`
	Begin int    `json:"begin"`
	End   int    `json:"end"`
}
