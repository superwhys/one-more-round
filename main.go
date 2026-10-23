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

	"github.com/miebyte/authkit/admin"
	"github.com/miebyte/goutils/cores"
	"github.com/miebyte/goutils/flags/v2"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/share"

	"github.com/superwhys/one-more-round/api"
	"github.com/superwhys/one-more-round/config"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/cli"
	"github.com/superwhys/one-more-round/internal/infra/bgg"
	"github.com/superwhys/one-more-round/internal/infra/mail"
	"github.com/superwhys/one-more-round/internal/infra/mysql"
	"github.com/superwhys/one-more-round/internal/infra/photos"
	"github.com/superwhys/one-more-round/internal/infra/wechat"
	"github.com/superwhys/one-more-round/internal/worker"
	"github.com/superwhys/one-more-round/web"
)

func init() {
	share.SetServiceName("one-more-round")
}

var (
	runtimeConfig = flags.Struct("config", &config.Runtime{}, "application configuration")
)

// main is the composition root: it loads the configuration, builds the
// infrastructure, wires the application services and starts the process.
func main() {
	flags.Parse(flags.WithRemoteConfig())
	defer flags.Close()

	confLoader, err := runtimeConfig()
	logging.PanicError(err)

	runtime := confLoader.Load()
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

	emailSender, err := mail.NewSender()
	logging.PanicError(err)

	photoFiles, err := photos.NewOSS(runtime.OSS)
	logging.PanicError(err)
	appCtx := &services.AppContext{
		Repos:  repos,
		Mailer: emailSender,
		Photos: photoFiles,
	}
	if runtime.Wechat.Enabled() {
		appCtx.Wechat = wechat.New(runtime.Wechat)
	}
	if runtime.BGG.Enabled() {
		appCtx.Catalogue = bgg.New(runtime.BGG.Token)
	}

	authkitSrv, err := services.NewAuthkitService(appCtx)
	logging.PanicError(err)

	authApp := services.NewAuthApp(appCtx, authkitSrv)
	groupApp := services.NewGroupApp(appCtx)
	roundApp := services.NewRoundApp(appCtx)
	photoApp := services.NewPhotoApp(appCtx)
	notificationApp := services.NewNotificationApp(appCtx)
	commentApp := services.NewCommentApp(appCtx)

	backend := api.NewAPI(
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

	adminHandler, err := admin.NewHTTPHandler(
		authkitSrv,
		runtime.AdminID,
		admin.Config{Title: "OMR"},
	)
	logging.PanicError(err)

	httpConfig := &cores.HttpServerConfig{}
	httpConfig.SetDefault()
	// Allow the bounded OSS upload and compensation to finish before responding.
	httpConfig.WriteTimeout = time.Minute

	srv := cores.NewCores(
		cores.WithConsulRegister(),
		cores.WithHttpHandler(
			cores.HttpHandler{ApiPrefix: "/", Handler: frontend},
			cores.HttpHandler{ApiPrefix: "/api", Handler: backend.SetupRouter()},
			cores.HttpHandler{ApiPrefix: "/admin", Handler: adminHandler},
		),
		cores.WithServiceAddress(runtime.Listen),
		cores.WithHttpServerConfig(httpConfig),
		cores.WithNameWorker("photo-cleanup", worker.PhotoCleanup(repos, photoFiles)),
	)
	logging.PanicError(cores.Start(srv))
}
