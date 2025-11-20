package oauth2

import (
	"encoding/json"
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/SkyPanel/SkyPanel/v3/utils"
	"net/url"
)

func GetInfo(token string, hint string) (TokenInfoResponse, error) {
	data := url.Values{}
	data.Set("token", token)

	if hint != "" {
		data.Set("token_type_hint", hint)
	}

	var info TokenInfoResponse

	request := createRequest(data)
	response, err := SkyPanel.Http().Do(request)
	defer utils.CloseResponse(response)
	if err != nil {
		return info, err
	}

	err = json.NewDecoder(response.Body).Decode(&info)
	return info, err
}
