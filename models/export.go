package models

import "github.com/nicklaw5/helix"

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
