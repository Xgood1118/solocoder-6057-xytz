package types

import "time"

type SubscriptionType string

const (
	SubscriptionTypeChannel  SubscriptionType = "channel"
	SubscriptionTypePlaylist SubscriptionType = "playlist"
)

type Subscription struct {
	ID           string           `yaml:"id"`
	Type         SubscriptionType `yaml:"type"`
	OriginalID   string           `yaml:"original_id"`
	DisplayName  string           `yaml:"display_name"`
	IsPaused     bool             `yaml:"is_paused"`
	LastFetched  time.Time        `yaml:"last_fetched,omitempty"`
	AddedAt      time.Time        `yaml:"added_at"`
	URL          string           `yaml:"url"`
}

type SubscriptionVideo struct {
	VideoItem
	SubscriptionID   string    `yaml:"subscription_id"`
	SubscriptionName string    `yaml:"subscription_name"`
	IsRead           bool      `yaml:"is_read"`
	FetchedAt        time.Time `yaml:"fetched_at"`
}

type SubscriptionState struct {
	Subscriptions []Subscription      `yaml:"subscriptions"`
	Videos        []SubscriptionVideo `yaml:"videos"`
}

type AddSubscriptionMsg struct {
	Subscription Subscription
}

type RemoveSubscriptionMsg struct {
	ID string
}

type ToggleSubscriptionPauseMsg struct {
	ID string
}

type RenameSubscriptionMsg struct {
	ID          string
	DisplayName string
}

type SubscriptionAddedMsg struct {
	Subscription Subscription
	Err          string
}

type SubscriptionRemovedMsg struct {
	ID  string
	Err string
}

type SubscriptionsLoadedMsg struct {
	Subscriptions []Subscription
	Videos        []SubscriptionVideo
	Err           string
}

type FetchSubscriptionsMsg struct {
	Count int
}

type FetchSubscriptionsResultMsg struct {
	Videos []SubscriptionVideo
	Err    string
}

type MarkVideoReadMsg struct {
	VideoID string
}

type MarkAllVideosReadMsg struct {
	SubscriptionID string
}

type ShowSubscriptionsMsg struct{}

type ShowUpdatesMsg struct{}

type UpdatesBatchDownloadMsg struct{}
