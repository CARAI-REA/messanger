package app

import (
	"context"

	"github.com/CARAI-REA/messanger/platform/pkg/closer"
	"github.com/CARAI-REA/messanger/platform/pkg/tokens"
	jwtTokens "github.com/CARAI-REA/messanger/platform/pkg/tokens/jwt"
	searchv1 "github.com/CARAI-REA/messanger/shared/pkg/proto/search/v1"

	searchv1api "search/internal/api/search/v1"
	"search/internal/config"
	"search/internal/kafka"
	osclient "search/internal/opensearch"
	searchsvc "search/internal/service/search"
)

type diContainer struct {
	os             *osclient.Client
	svc            *searchsvc.Service
	api            searchv1.SearchServiceServer
	consumer       *kafka.Consumer
	accessVerifier tokens.AccessTokenVerifier
}

func NewDiContainer() *diContainer { return &diContainer{} }

func (d *diContainer) OpenSearch() *osclient.Client {
	if d.os != nil {
		return d.os
	}
	c, err := osclient.New(config.AppConfig().OpenSearch.URL(), config.AppConfig().OpenSearch.MessagesIndex())
	if err != nil {
		panic(err)
	}
	d.os = c
	return d.os
}

func (d *diContainer) Service() *searchsvc.Service {
	if d.svc != nil {
		return d.svc
	}
	d.svc = searchsvc.NewService(d.OpenSearch())
	return d.svc
}

func (d *diContainer) API() searchv1.SearchServiceServer {
	if d.api != nil {
		return d.api
	}
	d.api = searchv1api.New(d.Service())
	return d.api
}

func (d *diContainer) AccessVerifier() tokens.AccessTokenVerifier {
	if d.accessVerifier != nil {
		return d.accessVerifier
	}
	d.accessVerifier = jwtTokens.NewAccessJWTVerifier(config.AppConfig().JWT)
	return d.accessVerifier
}

func (d *diContainer) Consumer() *kafka.Consumer {
	if d.consumer != nil {
		return d.consumer
	}
	cfg := config.AppConfig().Kafka
	c, err := kafka.NewConsumer(cfg.Brokers(), cfg.GroupID(), []string{cfg.ChatEventsTopic()}, d.OpenSearch())
	if err != nil {
		panic(err)
	}
	closer.AddNamed("kafka consumer", func(context.Context) error { return c.Close() })
	d.consumer = c
	return d.consumer
}
