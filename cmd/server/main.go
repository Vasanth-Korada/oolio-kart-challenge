// Command server runs the food-ordering HTTP API. It loads
// config/<APP_ENV>.json plus env overrides, connects to Postgres (falling back
// to in-memory storage in dev only), runs migrations, loads the coupon index,
// sets up JWT auth and serves until SIGINT or SIGTERM.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/auth"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/config"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/coupon"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/httpapi"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/order"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/logging"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/metrics"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/postgres"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/product"
	"github.com/Vasanth-Korada/oolio-kart-challenge/migrations"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server: fatal", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := logging.New(cfg.Log.Level)
	slog.SetDefault(logger)
	logger.Info("config: loaded", slog.String("env", cfg.Env), slog.String("file", cfg.File))

	met := metrics.New()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var (
		productRepo product.Repository
		orderRepo   order.Repository
		dbPinger    httpapi.Pinger
		storage     string
	)

	tokens, users, err := newAuth(cfg, logger)
	if err != nil {
		return err
	}

	pool, err := postgres.Connect(ctx, cfg.Database.URL)
	if err != nil {
		// Only dev may fall back, so a reviewer can run this without
		// Postgres; stage and prod fail fast (config.Validate enforces it).
		if !cfg.Database.FallbackToMemory {
			return fmt.Errorf("no in-memory fallback in %s: %w", cfg.Env, err)
		}
		logger.Warn("postgres unreachable, falling back to in-memory storage; data will not persist across restarts",
			slog.Any("error", err))
		productRepo = product.NewMemoryRepository(product.SeedProducts())
		orderRepo = order.NewMemoryRepository()
		dbPinger = noopPinger{}
		storage = "in-memory (fallback)"
	} else {
		defer pool.Close()
		logger.Info("server: running migrations")
		if err := postgres.Migrate(ctx, pool, migrations.FS); err != nil {
			return err
		}
		productRepo = product.NewDBRepository(pool)
		orderRepo = order.NewDBRepository(pool)
		dbPinger = pool
		storage = "postgres"
	}

	met.SetStorage(storage)

	couponValidator, codes, err := loadCouponValidator(cfg.Coupon.IndexPath, logger)
	if err != nil {
		return err
	}
	met.SetCouponIndexCodes(codes)

	productService := product.NewService(productRepo, logger)
	orderService := order.NewService(productService, couponValidator, orderRepo, met, logger)

	router := httpapi.NewRouter(httpapi.RouterDeps{
		Product:    &httpapi.ProductHandler{Service: productService},
		Order:      &httpapi.OrderHandler{Service: orderService},
		Health:     &httpapi.HealthHandler{DB: dbPinger, Storage: storage},
		Logger:     logger,
		CORSOrigin: cfg.CORS.AllowedOrigin,

		Auth:          &httpapi.AuthHandler{Users: users, Tokens: tokens, Metrics: met},
		TokenVerifier: tokens,
		APIKey:        cfg.Auth.APIKey,

		Metrics:        met,
		MetricsHandler: met.Handler(),
		AuthMetrics:    met,
	})

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           router,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server: listening", slog.String("addr", cfg.Addr()))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		logger.Info("server: shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	logger.Info("server: shut down cleanly")
	return nil
}

// loadCouponValidator also returns how many valid codes were loaded (0 for
// the fail-closed fallback), for the coupon index metric.
func loadCouponValidator(path string, logger *slog.Logger) (coupon.Validator, int, error) {
	idx, err := coupon.LoadIndex(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Warn("coupon: index file not found, coupon codes will all be rejected until it's built",
				slog.String("path", path))
			return coupon.NewUnavailableValidator(), 0, nil
		}
		return nil, 0, err
	}
	logger.Info("coupon: index loaded", slog.String("path", path), slog.Int("valid_codes", idx.Len()))
	return idx, idx.Len(), nil
}

// newAuth builds the JWT manager and the user store holding the one
// configured demo user. With no JWT_SECRET it generates a random secret, so
// tokens stop working on restart and are not shared across replicas.
func newAuth(cfg config.Config, logger *slog.Logger) (*auth.JWTManager, *auth.MemoryUserStore, error) {
	secret := []byte(cfg.Auth.JWTSecret)
	if len(secret) == 0 {
		var err error
		if secret, err = auth.RandomSecret(); err != nil {
			return nil, nil, err
		}
		logger.Warn("auth: JWT_SECRET not set, using a random secret; tokens will not survive a restart")
	}
	tokens, err := auth.NewJWTManager(secret, cfg.Auth.JWTTTL)
	if err != nil {
		return nil, nil, fmt.Errorf("auth: %w", err)
	}

	users, err := auth.NewMemoryUserStore()
	if err != nil {
		return nil, nil, err
	}
	if err := users.Add(cfg.Auth.Username, cfg.Auth.Password, auth.ScopeCreateOrder); err != nil {
		return nil, nil, fmt.Errorf("auth: %w", err)
	}
	logger.Info("auth: JWT enabled", slog.String("user", cfg.Auth.Username), slog.String("token_ttl", cfg.Auth.JWTTTL.String()))
	return tokens, users, nil
}

// noopPinger backs /readyz when running on the in-memory fallback: there's
// no external dependency left to be unready for.
type noopPinger struct{}

// Ping always succeeds: the in-memory store has no dependency to lose.
func (noopPinger) Ping(context.Context) error { return nil }
