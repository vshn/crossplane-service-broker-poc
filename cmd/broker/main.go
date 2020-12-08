package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"broker/pkg/crossplane"
	"broker/pkg/crossplanebroker"
	"broker/pkg/custom"

	"code.cloudfoundry.org/lager"
	"github.com/gorilla/mux"
	api "github.com/pivotal-cf/brokerapi/v7"
	"github.com/pivotal-cf/brokerapi/v7/auth"
	"github.com/pivotal-cf/brokerapi/v7/middlewares"
)

const (
	exitCodeErr = 1
)

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	logger := lager.NewLogger("broker")
	logger.RegisterSink(lager.NewPrettySink(os.Stdout, lager.DEBUG))

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	defer func() {
		signal.Stop(signalChan)
		cancel()
	}()

	if err := run(ctx, signalChan, logger); err != nil {
		logger.Error("application-run-failed", err)
		os.Exit(exitCodeErr)
	}
}

func run(ctx context.Context, signalChan chan os.Signal, logger lager.Logger) error {
	cfg, err := readAppConfig()
	if err != nil {
		return fmt.Errorf("unable to read app env: %w", err)
	}

	logger.WithData(lager.Data{"service": cfg.serviceIDs}).Info("starting-broker", lager.Data{"listen-addr": cfg.listenAddr})

	cp, err := crossplane.New(cfg.serviceIDs, logger)
	if err != nil {
		return fmt.Errorf("unable to create crossplane client: %w", err)
	}

	b, err := crossplanebroker.New(cp, logger.WithData(lager.Data{"module": "broker"}))
	if err != nil {
		return fmt.Errorf("unable to create broker: %w", err)
	}

	logger.Debug("basic-auth-credentials", lager.Data{"Username": cfg.username})

	credentials := api.BrokerCredentials{
		Username: cfg.username,
		Password: cfg.password,
	}

	baseRouter := mux.NewRouter()
	baseRouter.HandleFunc("/healthz", func(res http.ResponseWriter, req *http.Request) {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusOK)
		io.WriteString(res, `{"status": "ok"}`)
	}).Methods(http.MethodGet)
	baseRouter.Use(middlewares.AddCorrelationIDToContext)

	authMiddleware := auth.NewWrapper(credentials.Username, credentials.Password).Wrap
	osbRouter := baseRouter.NewRoute().Subrouter()
	osbRouter.Use(loggerMiddleware(logger))
	osbRouter.Use(authMiddleware)
	osbRouter.Use(middlewares.AddOriginatingIdentityToContext)
	osbRouter.Use(middlewares.AddInfoLocationToContext)

	apiVersionMiddleware := middlewares.APIVersionMiddleware{LoggerFactory: logger}
	apiRouter := osbRouter.NewRoute().Subrouter()
	apiRouter.Use(apiVersionMiddleware.ValidateAPIVersionHdr)

	api.AttachRoutes(apiRouter, b, logger)

	customAPIHandler := custom.NewAPIHandler(cp, logger.WithData(lager.Data{"module": "custom"}))
	custom.NewAPI(osbRouter, customAPIHandler, logger)

	srv := http.Server{
		Addr:           cfg.listenAddr,
		Handler:        baseRouter,
		ReadTimeout:    cfg.readTimeout,
		WriteTimeout:   cfg.writeTimeout,
		MaxHeaderBytes: cfg.maxHeaderBytes,
	}

	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server-error", err)
			signalChan <- syscall.SIGABRT
		}
		logger.Info("server-shutdown")
	}()

	sig := <-signalChan
	if sig == syscall.SIGABRT {
		return errors.New("unable to start server")
	}

	logger.Info("shutting down server", lager.Data{"signal": sig.String()})

	graceCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(graceCtx)
}

type appConfig struct {
	serviceIDs     []string
	username       string
	password       string
	listenAddr     string
	readTimeout    time.Duration
	writeTimeout   time.Duration
	maxHeaderBytes int
}

func readAppConfig() (*appConfig, error) {
	cfg := appConfig{
		serviceIDs: strings.Split(os.Getenv("OSB_SERVICE_IDS"), ","),
		username:   os.Getenv("OSB_USERNAME"),
		password:   os.Getenv("OSB_PASSWORD"),
		listenAddr: os.Getenv("OSB_HTTP_LISTEN_ADDR"),
	}
	for i := range cfg.serviceIDs {
		cfg.serviceIDs[i] = strings.TrimSpace(cfg.serviceIDs[i])
		if len(cfg.serviceIDs[i]) == 0 {
			return nil, errors.New("OSB_SERVICE_IDS is required")
		}
	}
	if cfg.username == "" {
		return nil, errors.New("OSB_USERNAME is required")
	}
	if cfg.password == "" {
		return nil, errors.New("OSB_PASSWORD is required")
	}

	if cfg.listenAddr == "" {
		cfg.listenAddr = ":8080"
	}

	rt, err := time.ParseDuration(os.Getenv("OSB_HTTP_READ_TIMEOUT"))
	if err != nil {
		rt = 180 * time.Second
	}
	cfg.readTimeout = rt

	wt, err := time.ParseDuration(os.Getenv("OSB_HTTP_WRITE_TIMEOUT"))
	if err != nil {
		wt = 180 * time.Second
	}
	cfg.writeTimeout = wt

	mhb, err := strconv.Atoi(os.Getenv("OSB_HTTP_MAX_HEADER_BYTES"))
	if err != nil {
		mhb = 1 << 20 // 1 MB
	}
	cfg.maxHeaderBytes = mhb

	return &cfg, nil
}

func loggerMiddleware(logger lager.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			id, ok := req.Context().Value(middlewares.CorrelationIDKey).(string)
			if !ok {
				id = "unknown"
			}
			headers := req.Header.Clone()
			if auth := headers.Get("Authorization"); auth != "" {
				headers.Set("Authorization", "****")
			}
			logger.WithData(lager.Data{
				"correlation-id": id,
				"headers":        headers,
				"URI":            req.RequestURI,
				"method":         req.Method,
			}).Debug("debug-headers")
			next.ServeHTTP(w, req)
		})
	}
}
