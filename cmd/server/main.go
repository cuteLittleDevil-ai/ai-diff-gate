package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/config"
	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/handler"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
		if _, err := os.Stat(configPath); err != nil {
			if _, err2 := os.Stat("config.example.yaml"); err2 == nil {
				configPath = "config.example.yaml"
			}
		}
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config %s: %v", configPath, err)
	}

	providers, err := config.BuildProviders(cfg, &http.Client{})
	if err != nil {
		log.Fatalf("build providers: %v", err)
	}

	tmplPath := filepath.Join("web", "compare.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Fatalf("parse template %s: %v", tmplPath, err)
	}

	srv := handler.New(cfg, providers, tmpl)
	mux := http.NewServeMux()
	srv.Register(mux)

	addr := cfg.Server.Addr
	log.Printf("ai-diff-gate listening on %s (config: %s)", addr, configPath)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}
