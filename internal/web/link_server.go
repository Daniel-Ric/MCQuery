package web

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type LookupLink struct {
	Name string
	Host string
	Port int
}

type LinkServer struct {
	URL      string
	server   *http.Server
	listener net.Listener
	entries  []LookupLink
}

func StartLookupLinkServer(entries []LookupLink, ttl time.Duration) (*LinkServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start link server: %w", err)
	}

	mux := http.NewServeMux()
	server := &http.Server{
		Handler: mux,
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("MCQuery link server is running. Use the links shown in the terminal.\n"))
			return
		}

		parts := strings.Split(path, "/")
		if len(parts) != 2 {
			http.NotFound(w, r)
			return
		}

		index, err := strconv.Atoi(parts[1])
		if err != nil || index < 0 || index >= len(entries) {
			http.NotFound(w, r)
			return
		}

		entry := entries[index]
		switch parts[0] {
		case "add":
			addValue := fmt.Sprintf("%s|%s:%d", entry.Name, entry.Host, entry.Port)
			target := "minecraft://?addExternalServer=" + url.QueryEscape(addValue)
			http.Redirect(w, r, target, http.StatusFound)
		case "connect":
			target := fmt.Sprintf(
				"minecraft://connect/?serverUrl=%s&serverPort=%d",
				url.QueryEscape(entry.Host),
				entry.Port,
			)
			http.Redirect(w, r, target, http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	})

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Serve(listener)
	}()

	linkServer := &LinkServer{
		URL:      fmt.Sprintf("http://%s", listener.Addr().String()),
		server:   server,
		listener: listener,
		entries:  append([]LookupLink(nil), entries...),
	}

	if ttl > 0 {
		time.AfterFunc(ttl, func() {
			_ = linkServer.Close()
		})
	}

	select {
	case err := <-serverErr:
		return nil, fmt.Errorf("start link server: %w", err)
	default:
	}

	return linkServer, nil
}

type LookupLinkURLs struct {
	Name       string
	AddURL     string
	ConnectURL string
}

func (s *LinkServer) Links() []LookupLinkURLs {
	if s == nil {
		return nil
	}
	links := make([]LookupLinkURLs, 0, len(s.entries))
	for i, entry := range s.entries {
		links = append(links, LookupLinkURLs{
			Name:       entry.Name,
			AddURL:     fmt.Sprintf("%s/add/%d", s.URL, i),
			ConnectURL: fmt.Sprintf("%s/connect/%d", s.URL, i),
		})
	}
	return links
}

func (s *LinkServer) Close() error {
	if s == nil {
		return nil
	}
	if s.server != nil {
		_ = s.server.Close()
	}
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}
