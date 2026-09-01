package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"UWP-TCP-Con/internal/ping"
)

type StatusAPIConfig struct {
	Addr        string
	Timeout     time.Duration
	Concurrency int
	EnableSRV   bool
	IPMode      ping.IPMode
}

type StatusAPIServer struct {
	config StatusAPIConfig
	server *http.Server
}

type StatusServerTarget struct {
	Name    string       `json:"name,omitempty"`
	Edition ping.Edition `json:"edition"`
	Host    string       `json:"host"`
	Port    int          `json:"port"`
}

type StatusRequest struct {
	Servers     []StatusServerTarget `json:"servers"`
	TimeoutMS   int                  `json:"timeout_ms,omitempty"`
	Concurrency int                  `json:"concurrency,omitempty"`
}

type StatusPlayers struct {
	Current int `json:"current"`
	Max     int `json:"max"`
}

type StatusIcon struct {
	Type   string `json:"type,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Bytes  int    `json:"bytes"`
}

type StatusEntry struct {
	Name                string            `json:"name,omitempty"`
	Edition             ping.Edition      `json:"edition"`
	Host                string            `json:"host"`
	Port                int               `json:"port"`
	Online              bool              `json:"online"`
	Players             *StatusPlayers    `json:"players,omitempty"`
	PlayerSample        []ping.JavaPlayer `json:"player_sample,omitempty"`
	MOTD                string            `json:"motd,omitempty"`
	CleanMOTD           string            `json:"clean_motd,omitempty"`
	SubMOTD             string            `json:"sub_motd,omitempty"`
	Version             string            `json:"version,omitempty"`
	ProtocolVersion     string            `json:"protocol_version,omitempty"`
	LatencyMillis       int64             `json:"latency_ms,omitempty"`
	ServerID            string            `json:"server_id,omitempty"`
	ServerGUID          uint64            `json:"server_guid,omitempty"`
	GameMode            string            `json:"game_mode,omitempty"`
	GameModeNumeric     string            `json:"game_mode_numeric,omitempty"`
	AdvertisedIPv4Port  int               `json:"advertised_ipv4_port,omitempty"`
	AdvertisedIPv6Port  int               `json:"advertised_ipv6_port,omitempty"`
	EnforcesSecureChat  *bool             `json:"enforces_secure_chat,omitempty"`
	PreviewsChat        *bool             `json:"previews_chat,omitempty"`
	PreventsChatReports *bool             `json:"prevents_chat_reports,omitempty"`
	ModLoader           string            `json:"mod_loader,omitempty"`
	Mods                []ping.JavaMod    `json:"mods,omitempty"`
	ModChannels         int               `json:"mod_channels,omitempty"`
	ModDataTruncated    bool              `json:"mod_data_truncated,omitempty"`
	Icon                *StatusIcon       `json:"icon,omitempty"`
	ResponseBytes       int               `json:"response_bytes,omitempty"`
	ExtraFields         []string          `json:"extra_fields,omitempty"`
	Error               string            `json:"error,omitempty"`
	CheckedAt           time.Time         `json:"checked_at"`
	DurationMillis      int64             `json:"duration_ms"`
}

type StatusResponse struct {
	Servers        []StatusEntry `json:"servers"`
	CheckedAt      time.Time     `json:"checked_at"`
	DurationMillis int64         `json:"duration_ms"`
}

func NewStatusAPIServer(config StatusAPIConfig) *StatusAPIServer {
	if config.Addr == "" {
		config.Addr = "127.0.0.1:8080"
	}
	if config.Timeout < 0 {
		config.Timeout = 0
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 32
	}

	api := &StatusAPIServer{config: config}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", api.handleHealth)
	mux.HandleFunc("/api/status", api.handleStatus)
	mux.HandleFunc("/api/stream", api.handleStream)
	api.server = &http.Server{
		Addr:    config.Addr,
		Handler: mux,
	}
	return api
}

func (s *StatusAPIServer) ListenAndServe() error {
	return s.server.ListenAndServe()
}

func (s *StatusAPIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *StatusAPIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	req, err := s.readStatusRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	response := s.queryStatus(r.Context(), req)
	writeJSON(w, http.StatusOK, response)
}

func (s *StatusAPIServer) handleStream(w http.ResponseWriter, r *http.Request) {
	req, err := s.readStatusRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	interval := parseDurationMillis(r.URL.Query().Get("interval_ms"), 2*time.Second)
	if interval < 250*time.Millisecond {
		interval = 250 * time.Millisecond
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	send := func() bool {
		response := s.queryStatus(r.Context(), req)
		data, err := json.Marshal(response)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "event: status\ndata: %s\n\n", data); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	if !send() {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if !send() {
				return
			}
		}
	}
}

func (s *StatusAPIServer) readStatusRequest(r *http.Request) (StatusRequest, error) {
	var req StatusRequest
	switch r.Method {
	case http.MethodGet:
		req.Servers = parseQueryTargets(r)
		req.TimeoutMS = parseIntDefault(r.URL.Query().Get("timeout_ms"), 0)
		req.Concurrency = parseIntDefault(r.URL.Query().Get("concurrency"), 0)
	case http.MethodPost:
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return StatusRequest{}, fmt.Errorf("invalid json body")
		}
	default:
		return StatusRequest{}, fmt.Errorf("method not allowed")
	}

	for i := range req.Servers {
		if err := normalizeTarget(&req.Servers[i]); err != nil {
			return StatusRequest{}, err
		}
	}
	if len(req.Servers) == 0 {
		return StatusRequest{}, fmt.Errorf("at least one server is required")
	}
	if req.TimeoutMS < 0 {
		return StatusRequest{}, fmt.Errorf("timeout_ms cannot be negative")
	}
	if req.Concurrency < 0 {
		return StatusRequest{}, fmt.Errorf("concurrency cannot be negative")
	}
	return req, nil
}

func (s *StatusAPIServer) queryStatus(ctx context.Context, req StatusRequest) StatusResponse {
	start := time.Now()
	entries := make([]StatusEntry, len(req.Servers))
	concurrency := req.Concurrency
	if concurrency <= 0 {
		concurrency = s.config.Concurrency
	}
	if concurrency <= 0 {
		concurrency = 32
	}
	if concurrency > len(req.Servers) {
		concurrency = len(req.Servers)
	}

	timeout := s.config.Timeout
	if req.TimeoutMS > 0 {
		timeout = time.Duration(req.TimeoutMS) * time.Millisecond
	}

	jobs := make(chan int, len(req.Servers))
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				entries[index] = s.queryOne(ctx, req.Servers[index], timeout)
			}
		}()
	}
	for i := range req.Servers {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	return StatusResponse{
		Servers:        entries,
		CheckedAt:      start.UTC(),
		DurationMillis: time.Since(start).Milliseconds(),
	}
}

func (s *StatusAPIServer) queryOne(ctx context.Context, target StatusServerTarget, timeout time.Duration) StatusEntry {
	start := time.Now()
	checkedAt := start.UTC()
	queryCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		queryCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	if cancel != nil {
		defer cancel()
	}

	result, _, err := ping.Execute(queryCtx, ping.ExecuteConfig{
		Edition:         target.Edition,
		Host:            target.Host,
		Port:            target.Port,
		Timeout:         timeout,
		RetryCount:      0,
		RetryDelay:      0,
		EnableSRV:       s.config.EnableSRV,
		IPMode:          s.config.IPMode,
		SkipJavaLatency: true,
	})

	entry := StatusEntry{
		Name:           target.Name,
		Edition:        target.Edition,
		Host:           target.Host,
		Port:           target.Port,
		Online:         err == nil,
		CheckedAt:      checkedAt,
		DurationMillis: time.Since(start).Milliseconds(),
	}
	if err != nil {
		entry.Error = err.Error()
		return entry
	}

	switch value := result.(type) {
	case ping.BedrockPong:
		entry.MOTD = value.MOTD
		entry.CleanMOTD = value.CleanMOTD
		entry.Version = value.GameVersion
		entry.ProtocolVersion = value.ProtocolVersion
		entry.SubMOTD = value.SubMOTD
		entry.LatencyMillis = value.LatencyMillis
		entry.ServerID = value.ServerID
		entry.ServerGUID = value.ServerGUID
		entry.GameMode = value.GameMode
		entry.GameModeNumeric = value.GameModeNumeric
		entry.AdvertisedIPv4Port = value.AdvertisedIPv4Port
		entry.AdvertisedIPv6Port = value.AdvertisedIPv6Port
		entry.ResponseBytes = value.ResponseBytes
		entry.ExtraFields = append([]string(nil), value.ExtraFields...)
		entry.Players = parseStatusPlayers(value.CurrentPlayers, value.MaxPlayers)
	case ping.JavaStatus:
		entry.MOTD = value.MOTD
		entry.CleanMOTD = value.CleanMOTD
		entry.Version = value.VersionName
		entry.ProtocolVersion = strconv.Itoa(value.ProtocolVersion)
		entry.LatencyMillis = value.LatencyMillis
		entry.Players = &StatusPlayers{Current: value.CurrentPlayers, Max: value.MaxPlayers}
		entry.PlayerSample = append([]ping.JavaPlayer(nil), value.PlayerSample...)
		entry.EnforcesSecureChat = value.EnforcesSecureChat
		entry.PreviewsChat = value.PreviewsChat
		entry.PreventsChatReports = value.PreventsChatReports
		entry.ModLoader = value.ModLoader
		entry.Mods = append([]ping.JavaMod(nil), value.Mods...)
		entry.ModChannels = value.ModChannels
		entry.ModDataTruncated = value.ModDataTruncated
		entry.ResponseBytes = value.StatusJSONBytes
		entry.ExtraFields = append([]string(nil), value.ExtraFields...)
		if len(value.IconPNG) > 0 {
			entry.Icon = &StatusIcon{Type: value.IconType, Width: value.IconWidth, Height: value.IconHeight, Bytes: len(value.IconPNG)}
		}
	}
	return entry
}

func parseQueryTargets(r *http.Request) []StatusServerTarget {
	query := r.URL.Query()
	var targets []StatusServerTarget
	for _, value := range query["server"] {
		target, ok := parseServerValue(value)
		if ok {
			targets = append(targets, target)
		}
	}
	if len(targets) > 0 {
		return targets
	}
	edition := ping.Edition(strings.TrimSpace(query.Get("edition")))
	host := strings.TrimSpace(query.Get("host"))
	port := parseIntDefault(query.Get("port"), 0)
	name := strings.TrimSpace(query.Get("name"))
	if edition != "" || host != "" || port != 0 {
		targets = append(targets, StatusServerTarget{
			Name:    name,
			Edition: edition,
			Host:    host,
			Port:    port,
		})
	}
	return targets
}

func parseServerValue(value string) (StatusServerTarget, bool) {
	parts := strings.Split(value, ",")
	if len(parts) < 2 || len(parts) > 4 {
		return StatusServerTarget{}, false
	}
	target := StatusServerTarget{
		Edition: ping.Edition(strings.TrimSpace(parts[0])),
		Host:    strings.TrimSpace(parts[1]),
	}
	if len(parts) >= 3 {
		target.Port = parseIntDefault(strings.TrimSpace(parts[2]), 0)
	}
	if len(parts) == 4 {
		target.Name = strings.TrimSpace(parts[3])
	}
	return target, true
}

func normalizeTarget(target *StatusServerTarget) error {
	target.Name = strings.TrimSpace(target.Name)
	target.Host = strings.TrimSpace(target.Host)
	target.Edition = ping.Edition(strings.ToLower(strings.TrimSpace(string(target.Edition))))
	if target.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	switch target.Edition {
	case ping.EditionBedrock, ping.EditionJava:
	default:
		return fmt.Errorf("edition must be bedrock or java")
	}
	if target.Port == 0 {
		target.Port = ping.DefaultPort(target.Edition)
	}
	if target.Port < 1 || target.Port > 65535 {
		return fmt.Errorf("port out of range")
	}
	return nil
}

func parseStatusPlayers(current, max string) *StatusPlayers {
	currentInt, currentErr := strconv.Atoi(strings.TrimSpace(current))
	maxInt, maxErr := strconv.Atoi(strings.TrimSpace(max))
	if currentErr != nil || maxErr != nil {
		return nil
	}
	return &StatusPlayers{Current: currentInt, Max: maxInt}
}

func parseIntDefault(value string, fallback int) int {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func parseDurationMillis(value string, fallback time.Duration) time.Duration {
	parsed := parseIntDefault(value, 0)
	if parsed <= 0 {
		return fallback
	}
	return time.Duration(parsed) * time.Millisecond
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
