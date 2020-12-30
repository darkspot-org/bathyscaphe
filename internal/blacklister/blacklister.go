package blacklister

import (
	configapi "github.com/creekorful/trandoshan/internal/configapi/client"
	"github.com/creekorful/trandoshan/internal/event"
	"github.com/creekorful/trandoshan/internal/process"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
	"net/http"
	"net/url"
)

// State represent the application state
type State struct {
	configClient configapi.Client
}

// Name return the process name
func (state *State) Name() string {
	return "blacklister"
}

// CommonFlags return process common flags
func (state *State) CommonFlags() []string {
	return []string{process.HubURIFlag, process.ConfigAPIURIFlag}
}

// CustomFlags return process custom flags
func (state *State) CustomFlags() []cli.Flag {
	return []cli.Flag{}
}

// Initialize the process
func (state *State) Initialize(provider process.Provider) error {
	client, err := provider.ConfigClient([]string{configapi.ForbiddenHostnamesKey})
	if err != nil {
		return err
	}
	state.configClient = client

	return nil
}

// Subscribers return the process subscribers
func (state *State) Subscribers() []process.SubscriberDef {
	return []process.SubscriberDef{
		{Exchange: event.TimeoutURLExchange, Queue: "blacklistingQueue", Handler: state.handleTimeoutURLEvent},
	}
}

// HTTPHandler returns the HTTP API the process expose
func (state *State) HTTPHandler(provider process.Provider) http.Handler {
	return nil
}

func (state *State) handleTimeoutURLEvent(subscriber event.Subscriber, msg event.RawMessage) error {
	var evt event.TimeoutURLEvent
	if err := subscriber.Read(&msg, &evt); err != nil {
		return err
	}

	u, err := url.Parse(evt.URL)
	if err != nil {
		return err
	}

	forbiddenHostnames, err := state.configClient.GetForbiddenHostnames()
	if err != nil {
		return err
	}

	// prevent duplicates
	for _, hostname := range forbiddenHostnames {
		if hostname.Hostname == u.Hostname() {
			log.Trace().Str("hostname", u.Hostname()).Msg("skipping duplicate hostname")
			return nil
		}
	}

	forbiddenHostnames = append(forbiddenHostnames, configapi.ForbiddenHostname{Hostname: u.Hostname()})

	if err := state.configClient.Set(configapi.ForbiddenHostnamesKey, forbiddenHostnames); err != nil {
		return err
	}

	log.Debug().
		Str("url", evt.URL).
		Str("hostname", u.Hostname()).
		Msg("Blacklisted new hostname")

	return nil
}