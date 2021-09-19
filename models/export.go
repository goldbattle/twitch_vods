package models

import (
	"github.com/nicklaw5/helix"
	"time"
)

type ChatRenderStructure struct {
	Streamer Streamer   `json:"streamer"`
	Comments []Comments `json:"comments"`
	Video    Video      `json:"video"`
	Emotes   Emotes     `json:"emotes"`
}

type Streamer struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

type Video struct {
	Start int     `json:"start"`
	End   float64 `json:"end"`
}

type Emotes struct {
	Thirdparty []Thirdparty `json:"thirdParty"`
	Firstparty []Firstparty `json:"firstParty"`
}

type Thirdparty struct {
	ID         string `json:"id"`
	Imagescale int    `json:"imageScale"`
	Data       string `json:"data"`
	Name       string `json:"name"`
}

type Firstparty struct {
	ID         string `json:"id"`
	Imagescale int    `json:"imageScale"`
	Data       string `json:"data"`
}

type MappingStreamToVod struct {
	Data map[string]helix.Video `json:"data"`
}

type StreamMetaData struct {
	Id       string `json:"id"`
	IdStream string `json:"id_stream"`
	UserId   string `json:"user_id"`
	UserName string `json:"user_name"`
	Title    string `json:"title"`
	Titles  []Moment `json:"titles"`
	Duration string `json:"duration"`
	Game     string `json:"game"`
	Url      string `json:"url"`
	Views    int    `json:"views"`
	Moments  []Moment `json:"moments"`
	MutedSegments []interface{} `json:"muted_segments"`
	RecordedAt    time.Time     `json:"recorded_at"`
}

type Moment struct {
	Duration int    `json:"duration"`
	Offset   int    `json:"offset"`
	Id       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
}