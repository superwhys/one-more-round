// File:		main.go
// Created by:	Hoven
// Created on:	2026-09-18
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package main

import (
	"context"
	"time"

	"github.com/miebyte/goutils/buildinfo"
	"github.com/miebyte/goutils/cores"
	"github.com/miebyte/goutils/flags"
	"github.com/miebyte/goutils/logging"

	"github.com/superwhys/one-more-round/api"
	"github.com/superwhys/one-more-round/config"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/cli"
	"github.com/superwhys/one-more-round/internal/infra/bgg"
	"github.com/superwhys/one-more-round/internal/infra/mail"
	"github.com/superwhys/one-more-round/internal/infra/mysql"
	"github.com/superwhys/one-more-round/internal/infra/photos"
	"github.com/superwhys/one-more-round/internal/worker"
	"github.com/superwhys/one-more-round/web"
)

var (
	listen        = flags.String("listen", "127.0.0.1:8080", "HTTP listen address")
	runtimeConfig = flags.Struct("app", &config.Runtime{}, "application configuration")
)

// main is the composition root: it loads the configuration, builds the
// infrastructure, wires the application services and starts the process.
func main() {
	flags.Parse()

	runtime := &config.Runtime{Listen: listen()}
	logging.PanicError(runtimeConfig(runtime))
	logging.PanicError(runtime.Validate())

	client, err := mysql.Open(runtime.MySQL)
	logging.PanicError(err)
	defer client.Close()
	logging.PanicError(client.AutoMigrate())

	repos := mysql.NewRepositoryFactory(client.Gorm)
	if handled, err := cli.RunTrial(context.Background(), repos.Trial(), runtime.Origin); handled {
		logging.PanicError(err)
		return
	}

	photoFiles, err := photos.NewOSS(runtime.OSS)
	logging.PanicError(err)
	appCtx := &services.AppContext{
		Repos:  repos,
		Mailer: &mail.Sender{Config: runtime.SMTP},
		Photos: photoFiles,
	}
	if runtime.BGG.Enabled() {
		appCtx.Catalogue = bgg.New(runtime.BGG.Token)
	}
	authApp := services.NewAuthApp(appCtx)
	groupApp := services.NewGroupApp(appCtx)
	roundApp := services.NewRoundApp(appCtx)
	photoApp := services.NewPhotoApp(appCtx)
	notificationApp := services.NewNotificationApp(appCtx)
	commentApp := services.NewCommentApp(appCtx)

	backend := api.NewAPI(
		buildinfo.Version,
		runtime,
		authApp,
		groupApp,
		roundApp,
		photoApp,
		notificationApp,
		commentApp,
	)
	frontend, err := web.NewHandler()
	logging.PanicError(err)

	httpConfig := &cores.HttpServerConfig{}
	httpConfig.SetDefault()
	// Allow the bounded OSS upload and compensation to finish before responding.
	httpConfig.WriteTimeout = time.Minute
	srv := cores.NewCores(
		cores.WithHttpServerConfig(httpConfig),
		cores.WithNameWorker("photo-cleanup", worker.PhotoCleanup(repos, photoFiles)),
		cores.WithHttpHandler("/", frontend),
		cores.WithHttpHandler("/api", backend.SetupRouter()),
		cores.WithHttpHandler("/swagger", backend.SwaggerRouter(runtime.IsProd)),
	)

	logging.PanicError(cores.Start(srv, runtime.Listen))
}
