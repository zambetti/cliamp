// Package radio implements a playlist.Provider for internet radio stations.
// It includes a built-in cliamp radio stream, user-defined stations from
// ~/.config/cliamp/radios.toml, favorites from radio_favorites.toml, and
// lazy-loaded catalog stations from the Radio Browser API.
package radio

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"cliamp/internal/appdir"
	"cliamp/internal/tomlutil"
	"cliamp/playlist"
)

const builtinName = "cliamp radio"
const builtinURL = "https://radio.cliamp.stream/streams.m3u"

// Provider serves radio stations as single-track playlists.
// It combines local stations, user favorites, and catalog stations
// from the Radio Browser API into a single unified list.
type Provider struct {
	mu            sync.Mutex
	stations      []station        // built-in + user-defined (radios.toml)
	favorites     *Favorites       // user favorites (radio_favorites.toml)
	catalog       []CatalogStation // lazily loaded from Radio Browser API
	searchResults []CatalogStation // non-nil when API search is active
}

type station struct {
	name string
	url  string
}

// New creates a Provider with the built-in station plus any user-defined
// stations from ~/.config/cliamp/radios.toml and favorites.
func New() *Provider {
	p := &Provider{
		stations: []station{
			{name: builtinName, url: builtinURL},
		},
	}

	dir, err := appdir.Dir()
	if err != nil {
		p.favorites = &Favorites{byURL: make(map[string]struct{})}
		return p
	}
	if extra, err := loadStations(filepath.Join(dir, "radios.toml")); err == nil {
		p.stations = append(p.stations, extra...)
	}
	p.favorites = LoadFavorites()
	return p
}

func (p *Provider) Name() string { return "Radio" }

// Playlists returns a unified list: local stations, then favorites (★ prefixed),
// then catalog stations (with metadata). IDs are prefixed: "l:", "f:", "c:".
func (p *Provider) Playlists() ([]playlist.PlaylistInfo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var out []playlist.PlaylistInfo

	// When search is active, show only search results.
	if p.searchResults != nil {
		for i, s := range p.searchResults {
			name := formatCatalogName(s)
			if p.favorites != nil && p.favorites.Contains(s.URL) {
				name = "★ " + name
			}
			out = append(out, playlist.PlaylistInfo{
				ID:   fmt.Sprintf("s:%d", i),
				Name: name,
			})
		}
		return out, nil
	}

	// Local stations.
	for i, s := range p.stations {
		out = append(out, playlist.PlaylistInfo{
			ID:   fmt.Sprintf("l:%d", i),
			Name: s.name,
		})
	}

	// Favorites.
	if p.favorites != nil {
		favs := p.favorites.Stations()
		for i, s := range favs {
			out = append(out, playlist.PlaylistInfo{
				ID:   fmt.Sprintf("f:%d", i),
				Name: "★ " + formatCatalogName(s),
			})
		}
	}

	// Catalog stations.
	{
		for i, s := range p.catalog {
			name := formatCatalogName(s)
			if p.favorites != nil && p.favorites.Contains(s.URL) {
				name = "★ " + name
			}
			out = append(out, playlist.PlaylistInfo{
				ID:   fmt.Sprintf("c:%d", i),
				Name: name,
			})
		}
	}

	return out, nil
}

// Tracks returns a single-track playlist for the given station ID.
func (p *Provider) Tracks(id string) ([]playlist.Track, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	prefix, idxStr, ok := strings.Cut(id, ":")
	if !ok {
		// Legacy numeric ID: treat as local station index.
		idx, err := strconv.Atoi(id)
		if err != nil || idx < 0 || idx >= len(p.stations) {
			return nil, errors.New("invalid station ID")
		}
		s := p.stations[idx]
		return []playlist.Track{{
			Path: s.url, Title: s.name, Stream: true, Realtime: true,
		}}, nil
	}

	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		return nil, errors.New("invalid station ID")
	}

	switch prefix {
	case "l":
		if idx < 0 || idx >= len(p.stations) {
			return nil, errors.New("invalid local station index")
		}
		s := p.stations[idx]
		return []playlist.Track{{
			Path: s.url, Title: s.name, Stream: true, Realtime: true,
		}}, nil
	case "f":
		if p.favorites == nil {
			return nil, errors.New("no favorites loaded")
		}
		favs := p.favorites.Stations()
		if idx < 0 || idx >= len(favs) {
			return nil, errors.New("invalid favorite index")
		}
		s := favs[idx]
		return []playlist.Track{{
			Path: s.URL, Title: s.Name, Stream: true, Realtime: true,
		}}, nil
	case "c":
		if idx < 0 || idx >= len(p.catalog) {
			return nil, errors.New("invalid catalog station index")
		}
		s := p.catalog[idx]
		return []playlist.Track{{
			Path: s.URL, Title: s.Name, Stream: true, Realtime: true,
		}}, nil
	case "s":
		if p.searchResults == nil || idx < 0 || idx >= len(p.searchResults) {
			return nil, errors.New("invalid search result index")
		}
		s := p.searchResults[idx]
		return []playlist.Track{{
			Path: s.URL, Title: s.Name, Stream: true, Realtime: true,
		}}, nil
	default:
		return nil, errors.New("unknown station type")
	}
}

// AppendCatalog adds catalog stations fetched from the Radio Browser API.
func (p *Provider) AppendCatalog(stations []CatalogStation) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.catalog = append(p.catalog, stations...)
}

// CatalogLen returns the number of catalog stations currently loaded.
func (p *Provider) CatalogLen() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.catalog)
}

// LocalCount returns the number of local stations (built-in + radios.toml).
func (p *Provider) LocalCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.stations)
}

// FavCount returns the number of favorite stations.
func (p *Provider) FavCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.favorites == nil {
		return 0
	}
	return len(p.favorites.Stations())
}

// Favorites returns the favorites manager.
func (p *Provider) Favorites() *Favorites {
	return p.favorites
}

// ToggleFavorite toggles the favorite status of a catalog or favorite entry.
// Returns (true, name) if added, (false, name) if removed.
func (p *Provider) ToggleFavorite(id string) (added bool, name string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.favorites == nil {
		return false, "", errors.New("favorites not loaded")
	}

	prefix, idxStr, ok := strings.Cut(id, ":")
	if !ok {
		return false, "", errors.New("cannot favorite this entry")
	}
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		return false, "", errors.New("invalid ID")
	}

	var s CatalogStation
	switch prefix {
	case "c":
		if idx < 0 || idx >= len(p.catalog) {
			return false, "", errors.New("invalid catalog index")
		}
		s = p.catalog[idx]
	case "s":
		if p.searchResults == nil || idx < 0 || idx >= len(p.searchResults) {
			return false, "", errors.New("invalid search result index")
		}
		s = p.searchResults[idx]
	case "f":
		favs := p.favorites.Stations()
		if idx < 0 || idx >= len(favs) {
			return false, "", errors.New("invalid favorite index")
		}
		s = favs[idx]
	default:
		return false, "", errors.New("cannot favorite local stations")
	}

	if p.favorites.Contains(s.URL) {
		_ = p.favorites.Remove(s.URL)
		return false, s.Name, nil
	}
	_ = p.favorites.Add(s)
	return true, s.Name, nil
}

// SetSearchResults activates search mode with the given results.
// Playlists() will return search results instead of catalog stations.
func (p *Provider) SetSearchResults(stations []CatalogStation) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.searchResults = stations
}

// ClearSearch deactivates search mode, restoring the catalog view.
func (p *Provider) ClearSearch() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.searchResults = nil
}

// IsSearching returns true if API search results are active.
func (p *Provider) IsSearching() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.searchResults != nil
}

// IsCatalogOrFavID returns true if the ID belongs to a catalog, search, or favorite entry.
func IsCatalogOrFavID(id string) bool {
	return strings.HasPrefix(id, "c:") || strings.HasPrefix(id, "f:") || strings.HasPrefix(id, "s:")
}

// IDPrefix returns the type prefix of a provider list ID ("l", "f", "c", or "").
func IDPrefix(id string) string {
	prefix, _, ok := strings.Cut(id, ":")
	if !ok {
		return ""
	}
	return prefix
}

// formatCatalogName builds a display name from a CatalogStation.
func formatCatalogName(s CatalogStation) string {
	name := s.Name
	if s.Bitrate > 0 {
		name += fmt.Sprintf(" [%dk]", s.Bitrate)
	}
	if s.Country != "" {
		name += " · " + s.Country
	}
	return name
}

// loadStations parses a TOML file with [[station]] sections.
func loadStations(path string) ([]station, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var stations []station
	var current *station

	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if line == "[[station]]" {
			if current != nil && current.name != "" && current.url != "" {
				stations = append(stations, *current)
			}
			current = &station{}
			continue
		}
		if current == nil {
			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		val = tomlutil.Unquote(val)

		switch key {
		case "name":
			current.name = val
		case "url":
			current.url = val
		}
	}
	if current != nil && current.name != "" && current.url != "" {
		stations = append(stations, *current)
	}
	return stations, nil
}
