package utils

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	log "charm.land/log/v2"

	"github.com/xdagiz/xytz/internal/config"
	"github.com/xdagiz/xytz/internal/paths"
	"github.com/xdagiz/xytz/internal/types"
	"gopkg.in/yaml.v3"
)

const SubscriptionsFileName = "subscriptions.yaml"

var ErrInvalidSubscription = errors.New("subscription must have valid id and type")

var subscriptionsMu sync.Mutex

var GetSubscriptionsFilePath = func() string {
	configDir := paths.GetConfigDir()
	if err := paths.EnsureDirExists(configDir); err != nil {
		log.Warn("could not create config directory", "err", err)
		return SubscriptionsFileName
	}

	return filepath.Join(configDir, SubscriptionsFileName)
}

func LoadSubscriptions() (*types.SubscriptionState, error) {
	subscriptionsMu.Lock()
	defer subscriptionsMu.Unlock()
	return loadSubscriptionsUnlocked()
}

func loadSubscriptionsUnlocked() (*types.SubscriptionState, error) {
	path := GetSubscriptionsFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &types.SubscriptionState{
				Subscriptions: []types.Subscription{},
				Videos:        []types.SubscriptionVideo{},
			}, nil
		}
		return nil, err
	}

	if len(data) == 0 {
		return &types.SubscriptionState{
			Subscriptions: []types.Subscription{},
			Videos:        []types.SubscriptionVideo{},
		}, nil
	}

	var state types.SubscriptionState
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	if state.Subscriptions == nil {
		state.Subscriptions = []types.Subscription{}
	}
	if state.Videos == nil {
		state.Videos = []types.SubscriptionVideo{}
	}

	return &state, nil
}

func SaveSubscriptions(state *types.SubscriptionState) error {
	subscriptionsMu.Lock()
	defer subscriptionsMu.Unlock()
	return saveSubscriptionsUnlocked(state)
}

func saveSubscriptionsUnlocked(state *types.SubscriptionState) error {
	if state == nil {
		state = &types.SubscriptionState{
			Subscriptions: []types.Subscription{},
			Videos:        []types.SubscriptionVideo{},
		}
	}

	path := GetSubscriptionsFilePath()
	data, err := yaml.Marshal(state)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return os.WriteFile(path, data, 0o644)
}

func AddSubscription(sub types.Subscription) error {
	if sub.ID == "" || sub.Type == "" {
		return ErrInvalidSubscription
	}

	subscriptionsMu.Lock()
	defer subscriptionsMu.Unlock()

	state, err := loadSubscriptionsUnlocked()
	if err != nil {
		return err
	}

	for i, s := range state.Subscriptions {
		if s.ID == sub.ID {
			state.Subscriptions[i] = sub
			return saveSubscriptionsUnlocked(state)
		}
	}

	state.Subscriptions = append(state.Subscriptions, sub)
	return saveSubscriptionsUnlocked(state)
}

func RemoveSubscription(id string) error {
	subscriptionsMu.Lock()
	defer subscriptionsMu.Unlock()

	state, err := loadSubscriptionsUnlocked()
	if err != nil {
		return err
	}

	var newSubs []types.Subscription
	for _, s := range state.Subscriptions {
		if s.ID != id {
			newSubs = append(newSubs, s)
		}
	}

	var newVideos []types.SubscriptionVideo
	for _, v := range state.Videos {
		if v.SubscriptionID != id {
			newVideos = append(newVideos, v)
		}
	}

	state.Subscriptions = newSubs
	state.Videos = newVideos

	return saveSubscriptionsUnlocked(state)
}

func GetSubscriptionByID(id string) (*types.Subscription, error) {
	subscriptionsMu.Lock()
	defer subscriptionsMu.Unlock()

	state, err := loadSubscriptionsUnlocked()
	if err != nil {
		return nil, err
	}

	for _, s := range state.Subscriptions {
		if s.ID == id {
			return &s, nil
		}
	}

	return nil, nil
}

func IsSubscribed(id string) bool {
	sub, _ := GetSubscriptionByID(id)
	return sub != nil
}

func ToggleSubscriptionPause(id string) error {
	subscriptionsMu.Lock()
	defer subscriptionsMu.Unlock()

	state, err := loadSubscriptionsUnlocked()
	if err != nil {
		return err
	}

	for i, s := range state.Subscriptions {
		if s.ID == id {
			state.Subscriptions[i].IsPaused = !state.Subscriptions[i].IsPaused
			return saveSubscriptionsUnlocked(state)
		}
	}

	return nil
}

func RenameSubscription(id string, displayName string) error {
	subscriptionsMu.Lock()
	defer subscriptionsMu.Unlock()

	state, err := loadSubscriptionsUnlocked()
	if err != nil {
		return err
	}

	for i, s := range state.Subscriptions {
		if s.ID == id {
			state.Subscriptions[i].DisplayName = displayName
			for j, v := range state.Videos {
				if v.SubscriptionID == id {
					state.Videos[j].SubscriptionName = displayName
				}
			}
			return saveSubscriptionsUnlocked(state)
		}
	}

	return nil
}

func UpdateSubscriptionLastFetched(id string, t time.Time) error {
	subscriptionsMu.Lock()
	defer subscriptionsMu.Unlock()

	state, err := loadSubscriptionsUnlocked()
	if err != nil {
		return err
	}

	for i, s := range state.Subscriptions {
		if s.ID == id {
			state.Subscriptions[i].LastFetched = t
			return saveSubscriptionsUnlocked(state)
		}
	}

	return nil
}

func GetSubscriptionVideos() ([]types.SubscriptionVideo, error) {
	subscriptionsMu.Lock()
	defer subscriptionsMu.Unlock()

	state, err := loadSubscriptionsUnlocked()
	if err != nil {
		return nil, err
	}

	return state.Videos, nil
}

func AddSubscriptionVideos(videos []types.SubscriptionVideo) error {
	if len(videos) == 0 {
		return nil
	}

	subscriptionsMu.Lock()
	defer subscriptionsMu.Unlock()

	state, err := loadSubscriptionsUnlocked()
	if err != nil {
		return err
	}

	existingIDs := make(map[string]bool)
	for _, v := range state.Videos {
		existingIDs[v.ID] = true
	}

	for _, v := range videos {
		if !existingIDs[v.ID] {
			state.Videos = append(state.Videos, v)
		}
	}

	sort.SliceStable(state.Videos, func(i, j int) bool {
		return state.Videos[i].UploadDate > state.Videos[j].UploadDate
	})

	return saveSubscriptionsUnlocked(state)
}

func MarkVideoRead(videoID string) error {
	subscriptionsMu.Lock()
	defer subscriptionsMu.Unlock()

	state, err := loadSubscriptionsUnlocked()
	if err != nil {
		return err
	}

	for i, v := range state.Videos {
		if v.ID == videoID {
			state.Videos[i].IsRead = true
			return saveSubscriptionsUnlocked(state)
		}
	}

	return nil
}

func MarkAllVideosRead(subscriptionID string) error {
	subscriptionsMu.Lock()
	defer subscriptionsMu.Unlock()

	state, err := loadSubscriptionsUnlocked()
	if err != nil {
		return err
	}

	for i, v := range state.Videos {
		if subscriptionID == "" || v.SubscriptionID == subscriptionID {
			state.Videos[i].IsRead = true
		}
	}

	return saveSubscriptionsUnlocked(state)
}

func GetUnreadVideos() ([]types.SubscriptionVideo, error) {
	all, err := GetSubscriptionVideos()
	if err != nil {
		return nil, err
	}

	var unread []types.SubscriptionVideo
	for _, v := range all {
		if !v.IsRead {
			unread = append(unread, v)
		}
	}

	return unread, nil
}

func FetchSubscriptionVideos(cfg *config.Config, searchMgr *ExecManager, count int) ([]types.SubscriptionVideo, error) {
	state, err := LoadSubscriptions()
	if err != nil {
		return nil, err
	}

	if count <= 0 {
		count = cfg.SearchLimit
	}
	if count <= 0 {
		count = 25
	}

	var allNewVideos []types.SubscriptionVideo
	now := time.Now()

	for _, sub := range state.Subscriptions {
		if sub.IsPaused {
			continue
		}

		var videos []types.VideoItem
		var fetchErr error

		switch sub.Type {
		case types.SubscriptionTypeChannel:
			videos, fetchErr = fetchChannelVideosSync(searchMgr, cfg, sub.OriginalID, count)
		case types.SubscriptionTypePlaylist:
			videos, fetchErr = fetchPlaylistVideosSync(searchMgr, cfg, sub.URL, count)
		}

		if fetchErr != nil {
			log.Warn("failed to fetch subscription videos", "subscription", sub.ID, "err", fetchErr)
			continue
		}

		displayName := sub.DisplayName
		if displayName == "" {
			displayName = sub.OriginalID
		}

		for _, v := range videos {
			allNewVideos = append(allNewVideos, types.SubscriptionVideo{
				VideoItem:        v,
				SubscriptionID:   sub.ID,
				SubscriptionName: displayName,
				IsRead:           false,
				FetchedAt:        now,
			})
		}

		_ = UpdateSubscriptionLastFetched(sub.ID, now)
	}

	if len(allNewVideos) > 0 {
		if err := AddSubscriptionVideos(allNewVideos); err != nil {
			log.Warn("failed to add subscription videos", "err", err)
		}
	}

	return allNewVideos, nil
}

func fetchChannelVideosSync(searchMgr *ExecManager, cfg *config.Config, channelID string, count int) ([]types.VideoItem, error) {
	channelURL := BuildChannelURL(channelID)
	result := executeYTDLP(searchMgr, cfg, channelURL, count, "", "")
	if result == nil {
		return nil, errors.New("search canceled")
	}

	msg, ok := result.(types.SearchResultMsg)
	if !ok {
		return nil, nil
	}

	if msg.Err != "" {
		return nil, errors.New(msg.Err)
	}

	var videos []types.VideoItem
	for _, item := range msg.Videos {
		if v, ok := item.(types.VideoItem); ok {
			videos = append(videos, v)
		}
	}

	return videos, nil
}

func fetchPlaylistVideosSync(searchMgr *ExecManager, cfg *config.Config, playlistURL string, count int) ([]types.VideoItem, error) {
	result := executeYTDLP(searchMgr, cfg, playlistURL, count, "", "")
	if result == nil {
		return nil, errors.New("search canceled")
	}

	msg, ok := result.(types.SearchResultMsg)
	if !ok {
		return nil, nil
	}

	if msg.Err != "" {
		return nil, errors.New(msg.Err)
	}

	var videos []types.VideoItem
	for _, item := range msg.Videos {
		if v, ok := item.(types.VideoItem); ok {
			videos = append(videos, v)
		}
	}

	return videos, nil
}

func MarkSubscriptionDownloadsAbandoned(subscriptionID string) error {
	state, err := LoadSubscriptions()
	if err != nil {
		return err
	}

	var urls []string
	for _, v := range state.Videos {
		if v.SubscriptionID == subscriptionID {
			url := ResolveVideoItemURL(v.VideoItem)
			if url != "" {
				urls = append(urls, url)
			}
		}
	}

	if len(urls) == 0 {
		return nil
	}

	return RemoveUnfinishedBatch(urls)
}
