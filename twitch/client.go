package twitch

import (
	"errors"
	"github.com/nicklaw5/helix"
	"log"
	"time"
)

// InitAppAccessToken requests and sets app access token to the provided helix.Client
// and initializes a ticker running every 24 Hours which re-requests and sets app access token
func InitAppAccessToken(helixAPI *helix.Client, tokenFetched chan struct{}) {

	// Request for a new token
	response := &helix.AppAccessTokenResponse{}
	err := errors.New("startup")
	for err != nil {
		response, err = helixAPI.RequestAppAccessToken([]string{})
		if err != nil {
			log.Printf("HELIX: error requesting app access token: %s", err)
		}
	}
	log.Printf("HELIX: requested access token, status: %d, expires in: %d", response.StatusCode, response.Data.ExpiresIn)
	helixAPI.SetAppAccessToken(response.Data.AccessToken)
	close(tokenFetched)

	// initialize the ticker
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		response, err := helixAPI.RequestAppAccessToken([]string{})
		if err != nil {
			log.Printf("HELIX: failed to re-request app access token from ticker, status: %d", response.StatusCode)
			continue
		}
		log.Printf("HELIX: re-requested access token from ticker, status: %d, expires in: %d", response.StatusCode, response.Data.ExpiresIn)
		helixAPI.SetAppAccessToken(response.Data.AccessToken)
	}
}

func RateLimitCallback(lastResponse *helix.Response) error {
	if lastResponse.GetRateLimitRemaining() > 0 {
		return nil
	}
	var reset64 int64
	reset64 = int64(lastResponse.GetRateLimitReset())
	currentTime := time.Now().Unix()
	if currentTime < reset64 {
		timeDiff := time.Duration(reset64 - currentTime)
		if timeDiff > 0 {
			log.Printf("CLIENT: waiting on rate limit (%d seconds)\n", timeDiff)
			time.Sleep(timeDiff * time.Second)
		}
	}
	return nil
}
