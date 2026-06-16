package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"UWP-TCP-Con/internal/ping"
)

type favorite struct {
	Name       string       `json:"name"`
	Edition    ping.Edition `json:"edition"`
	Host       string       `json:"host"`
	Port       int          `json:"port"`
	CreatedAt  string       `json:"created_at"`
	LastUsedAt string       `json:"last_used_at,omitempty"`
}

func loadFavorites() ([]favorite, error) {
	path, err := configFile("favorites.json")
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var favorites []favorite
	if err := json.Unmarshal(data, &favorites); err != nil {
		return nil, err
	}
	return favorites, nil
}

func saveFavorites(favorites []favorite) error {
	path, err := configFile("favorites.json")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(favorites, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func (a *App) manageFavorites() error {
	for {
		favorites, err := loadFavorites()
		if err != nil {
			return err
		}
		options := []string{
			fmt.Sprintf("Run favorite: %d saved", len(favorites)),
			"Add favorite: Save a server profile",
			"Delete favorite: Remove a saved profile",
			"Back",
		}
		index, err := selectOption("Favorites", options)
		if err != nil {
			return err
		}
		switch index {
		case 0:
			if len(favorites) == 0 {
				renderTextPage("Favorites", "No favorites saved yet.")
				_ = waitForEnter()
				continue
			}
			favIndex, err := selectFavorite(favorites, "Run favorite")
			if err != nil {
				return err
			}
			if favIndex < 0 {
				continue
			}
			favorites[favIndex].LastUsedAt = time.Now().Format(time.RFC3339)
			if err := saveFavorites(favorites); err != nil {
				return err
			}
			fav := favorites[favIndex]
			return a.executeDirect(DirectConfig{
				Host:    fav.Host,
				Port:    fav.Port,
				Edition: fav.Edition,
			})
		case 1:
			fav, err := a.collectFavorite()
			if err != nil {
				return err
			}
			favorites = append(favorites, fav)
			if err := saveFavorites(favorites); err != nil {
				return err
			}
		case 2:
			if len(favorites) == 0 {
				renderTextPage("Favorites", "No favorites saved yet.")
				_ = waitForEnter()
				continue
			}
			favIndex, err := selectFavorite(favorites, "Delete favorite")
			if err != nil {
				return err
			}
			if favIndex < 0 {
				continue
			}
			if ok, err := askConfirm(fmt.Sprintf("Delete %s?", favorites[favIndex].Name)); err != nil {
				return err
			} else if !ok {
				continue
			}
			favorites = append(favorites[:favIndex], favorites[favIndex+1:]...)
			if err := saveFavorites(favorites); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

func selectFavorite(favorites []favorite, title string) (int, error) {
	options := make([]string, 0, len(favorites)+1)
	for _, fav := range favorites {
		options = append(options, fmt.Sprintf("%s: %s %s:%d", fav.Name, fav.Edition, fav.Host, fav.Port))
	}
	options = append(options, "Back")
	index, err := selectOption(title, options)
	if err != nil {
		return -1, err
	}
	if index == len(options)-1 {
		return -1, nil
	}
	return index, nil
}

func (a *App) collectFavorite() (favorite, error) {
	edition, err := a.askEdition()
	if err != nil {
		return favorite{}, err
	}
	host, err := a.askHost()
	if err != nil {
		return favorite{}, err
	}
	port, err := a.askPort(edition)
	if err != nil {
		return favorite{}, err
	}
	defaultName := fmt.Sprintf("%s:%d", host, port)
	name, err := promptInput("Favorite name", fmt.Sprintf("Default: %s. Leave empty to use it.", defaultName), "")
	if err != nil {
		return favorite{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = defaultName
	}
	return favorite{
		Name:      name,
		Edition:   edition,
		Host:      host,
		Port:      port,
		CreatedAt: time.Now().Format(time.RFC3339),
	}, nil
}
