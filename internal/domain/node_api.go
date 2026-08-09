package domain

type Deployment struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	PublicKey    string `json:"publicKey"`
} // @name NodeDeploymentConfig
