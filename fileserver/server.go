package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"hash/maphash"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type serverConfig struct {
	Port        string `yaml:"port" env:"FILESERVER_PORT" env-required:"true"`
	UploadDir   string `yaml:"upload_dir" env:"FILESERVER_UPLOAD_DIR" env-default:"./uploads"`
	MaxFileSize int64  `yaml:"max_file_size" env:"FILESERVER_MAX_FILE_SIZE" env-default:"10485760"`
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))

	cfgPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	if err := run(*cfgPath); err != nil {
		slog.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func run(cfgPath string) error {
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err = os.MkdirAll(cfg.UploadDir, 0o755); err != nil {
		return fmt.Errorf("failed create upload dir: %w", err)
	}

	if err = os.MkdirAll(filepath.Join(cfg.UploadDir, "tmp"), 0o755); err != nil {
		return fmt.Errorf("failed create tmp dir: %w", err)
	}

	uploadDir, err := os.OpenRoot(cfg.UploadDir)
	if err != nil {
		return fmt.Errorf("failed to open upload directory: %w", err)
	}
	defer func() {
		if cerr := uploadDir.Close(); cerr != nil {
			slog.Error("failed to close upload directory", "upload dir", cfg.UploadDir, "err", cerr)
		}
	}()

	fs := fileStorage{
		uploadDir:   uploadDir,
		maxFileSize: cfg.MaxFileSize,
		seed:        maphash.MakeSeed(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /files", fs.listFilesHandler)
	mux.HandleFunc("GET /files/{filename}", fs.getFileHandler)
	mux.HandleFunc("POST /files", fs.uploadFileHandler)
	mux.HandleFunc("PUT /files/{filename}", fs.updateFileHandler)
	mux.HandleFunc("DELETE /files/{filename}", fs.deleteFileHandler)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}

	slog.Info("starting server", "addr", srv.Addr)
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// read os signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	// graceful shutdown
	case <-ctx.Done():
		slog.Info("gracefully shutting down server")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("service forced to shutdown: %w", err)
		}
		slog.Info("server exiting")

	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func loadConfig(cfgPath string) (serverConfig, error) {
	var cfg serverConfig

	if err := cleanenv.ReadConfig(cfgPath, &cfg); err != nil {
		slog.Warn("yaml config not found, trying env", "err", err)
		if err = cleanenv.ReadEnv(&cfg); err != nil {
			return cfg, fmt.Errorf("reading config: %w", err)
		}
	}

	return cfg, nil
}
