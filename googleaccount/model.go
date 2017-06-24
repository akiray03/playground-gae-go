package googleaccount

import "time"

type Credential struct {
	ID          string
	IdToken     string
	AccessToken string
	TokenType   string
	ExpiresIn   time.Time
}

type GoogleAccount struct {
	ID         int64
	ProviderID string
	Name       string
	Email      string
	AvatarURL  string

	Credential Credential

	CreatedAt time.Time
	UpdatedAt time.Time
}
