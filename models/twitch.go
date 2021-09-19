package models

type CommentsV5ApiResponse struct {
	Comments []Comments `json:"comments"`
	Next     string     `json:"_next"`
}

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
