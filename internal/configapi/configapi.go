package configapi

import (
	"github.com/creekorful/trandoshan/internal/configapi/api"
	"github.com/creekorful/trandoshan/internal/configapi/database"
	"github.com/creekorful/trandoshan/internal/configapi/service"
	"github.com/creekorful/trandoshan/internal/event"
	"github.com/creekorful/trandoshan/internal/logging"
	"github.com/creekorful/trandoshan/internal/util"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"net/http"
	"time"
)

// GetApp return the config api app
func GetApp() *cli.App {
	return &cli.App{
		Name:    "tdsh-configapi",
		Version: "0.7.0",
		Usage:   "Trandoshan ConfigAPI component",
		Flags: []cli.Flag{
			logging.GetLogFlag(),
			util.GetHubURI(),
			&cli.StringFlag{
				Name:     "db-uri",
				Usage:    "URI to the database server",
				Required: true,
			},
		},
		Action: execute,
	}
}

func execute(ctx *cli.Context) error {
	logging.ConfigureLogger(ctx)

	log.Info().
		Str("ver", ctx.App.Version).
		Str("hub-uri", ctx.String("hub-uri")).
		Str("db-uri", ctx.String("db-uri")).
		Msg("Starting tdsh-configapi")

	// Create publisher
	pub, err := event.NewPublisher(ctx.String("hub-uri"))
	if err != nil {
		return err
	}

	// Create the ConfigAPI service
	db, err := database.NewRedisDatabase(ctx.String("db-uri"))
	if err != nil {
		return err
	}

	s, err := service.NewService(db, pub)

	state := state{
		api: s,
	}

	r := mux.NewRouter()
	r.HandleFunc("/config/{key}", state.getConfiguration).Methods(http.MethodGet)
	r.HandleFunc("/config/{key}", state.setConfiguration).Methods(http.MethodPut)

	srv := &http.Server{
		Addr: "0.0.0.0:8080",
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r, // Pass our instance of gorilla/mux in.
	}

	return srv.ListenAndServe()
}

type state struct {
	api api.ConfigAPI
}

func (state *state) getConfiguration(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	b, err := state.api.Get(key)
	if err != nil {
		log.Err(err).Msg("error while retrieving configuration")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

func (state *state) setConfiguration(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Err(err).Msg("error while reading body")
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	if err := state.api.Set(key, b); err != nil {
		log.Err(err).Msg("error while setting configuration")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}