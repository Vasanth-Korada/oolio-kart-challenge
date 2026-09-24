// Command server runs the food-ordering HTTP API. It loads config, connects to
// Postgres (falling back to in-memory storage), runs migrations, loads the
// coupon index and serves until SIGINT or SIGTERM.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/config"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/coupon"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/httpapi"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/order"
	"github.com/Vasanth-Korada/oolio-kart-challenge/internal/platform/logging"
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
	cfg := config.Load()
	logger := logging.New(cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var (
		productRepo product.Repository
		orderRepo   order.Repository
		dbPinger    httpapi.Pinger
		storage     string
	)

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		// Falling back here is so a reviewer can run this without first
		// standing up Postgres, purely for take-home assessment
		// convenience. A real production deployment should fail fast
		// instead.
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

	couponValidator, err := loadCouponValidator(cfg.CouponIndexPath, logger)
	if err != nil {
		return err
	}

	productService := product.NewService(productRepo, logger)
	orderService := order.NewService(productService, couponValidator, orderRepo, logger)

	router := httpapi.NewRouter(httpapi.RouterDeps{
		Product:    &httpapi.ProductHandler{Service: productService},
		Order:      &httpapi.OrderHandler{Service: orderService},
		Health:     &httpapi.HealthHandler{DB: dbPinger, Storage: storage},
		Logger:     logger,
		APIKey:     cfg.APIKey,
		CORSOrigin: cfg.CORSOrigin,
	})

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	logger.Info("server: shut down cleanly")
	return nil
}

func loadCouponValidator(path string, logger *slog.Logger) (coupon.Validator, error) {
	idx, err := coupon.LoadIndex(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Warn("coupon: index file not found, coupon codes will all be rejected until it's built",
				slog.String("path", path))
			return coupon.NewUnavailableValidator(), nil
		}
		return nil, err
	}
	logger.Info("coupon: index loaded", slog.String("path", path), slog.Int("valid_codes", idx.Len()))
	return idx, nil
}

// noopPinger backs /readyz when running on the in-memory fallback: there's
// no external dependency left to be unready for.
type noopPinger struct{}

// Ping always succeeds: the in-memory store has no dependency to lose.
func (noopPinger) Ping(context.Context) error { return nil }
