package main

import (
	"context"
	"fmt"
	"os"

	"github.com/achmadsalmanr/mcp-server/internal/config"
	"github.com/achmadsalmanr/mcp-server/internal/figma"
	"github.com/achmadsalmanr/mcp-server/internal/install"
	"github.com/achmadsalmanr/mcp-server/internal/mcp"
	"github.com/achmadsalmanr/mcp-server/internal/providers/bitbucket"
	"github.com/achmadsalmanr/mcp-server/internal/providers/github"
	"github.com/achmadsalmanr/mcp-server/internal/providers/gitlab"
	"github.com/achmadsalmanr/mcp-server/internal/review"
	"github.com/achmadsalmanr/mcp-server/internal/scm"
	"github.com/achmadsalmanr/mcp-server/internal/testrail"
)

func main() {
	cfg, warnings := config.Load()
	exe, _ := os.Executable()
	opts := install.Options{
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Executable: exe,
		Config:     cfg,
		Warnings:   warnings,
	}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			os.Exit(install.RunInstall(os.Args[2:], opts))
		case "doctor":
			os.Exit(install.RunDoctor(os.Args[2:], opts))
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

	stores := buildStores(cfg)
	var tr *testrail.Client
	if cfg.TestRail.Configured() {
		tr = testrail.NewClient(cfg.TestRail.BaseURL, cfg.TestRail.Email, cfg.TestRail.APIKey)
	}
	var fg *figma.Client
	if cfg.Figma.Configured() {
		fg = figma.NewClient(cfg.Figma.Token, cfg.Figma.APIBase)
	}
	server := mcp.NewServer(review.NewService(stores...), tr, fg, cfg.Statuses(), warnings)
	if err := server.RunStdio(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "server stopped: %v\n", err)
		os.Exit(1)
	}
}

func buildStores(cfg config.Config) []scm.PullRequestStore {
	var stores []scm.PullRequestStore
	if cfg.Bitbucket.Configured() {
		stores = append(stores, bitbucket.NewClient(
			cfg.Bitbucket.Workspace,
			cfg.Bitbucket.Username,
			cfg.Bitbucket.Token,
			cfg.Bitbucket.APIBase,
		))
	}
	stores = append(stores, github.NewClient(
		cfg.GitHub.Token,
		cfg.GitHub.APIBase,
		cfg.GitHub.WebBase,
		cfg.GitHub.APIVersion,
	))
	stores = append(stores, gitlab.NewClient(
		cfg.GitLab.Token,
		cfg.GitLab.BaseURL,
		cfg.GitLab.APIBase,
	))
	return stores
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "mcp-server")
	fmt.Fprintln(os.Stderr, "  run with no arguments to start the MCP stdio server")
	fmt.Fprintln(os.Stderr, "  install --target generic --print-config")
	fmt.Fprintln(os.Stderr, "  doctor [--json]")
}
