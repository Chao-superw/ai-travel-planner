package main

import (
	"ai-travel/internal/authn"
	"ai-travel/internal/config"
	"ai-travel/internal/mailer"
	"ai-travel/internal/providers"
	"ai-travel/internal/rpc"
	"ai-travel/internal/service"
	"ai-travel/internal/store"
	"ai-travel/kitex_gen/travel/travelservice"
	"context"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/transmeta"
	"github.com/cloudwego/kitex/server"
	"log"
	"net"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	url, e := config.Require("DATABASE_URL")
	if e != nil {
		log.Fatal(e)
	}
	token, e := config.Require("PLANNER_SERVICE_TOKEN")
	if e != nil {
		log.Fatal(e)
	}
	s, e := store.Open(ctx, url)
	if e != nil {
		log.Fatal(e)
	}
	defer s.Close()
	if e = s.Migrate(ctx); e != nil {
		log.Fatal(e)
	}
	base, e := config.ProviderURL("AMAP_BASE_URL", "https://restapi.amap.com")
	if e != nil {
		log.Fatal(e)
	}
	maps := &service.RateMap{Inner: &providers.Amap{BaseURL: base, APIKey: config.Env("AMAP_API_KEY", ""), Client: &http.Client{Timeout: 5 * time.Second}}}
	planner, e := rpc.NewPlannerClient(config.Env("PLANNER_ADDR", "127.0.0.1:8889"), token)
	if e != nil {
		log.Fatal("planner RPC client initialization failed")
	}
	policy, e := authn.LoadPolicy()
	if e != nil {
		log.Fatal(e)
	}
	smtpSender, e := mailer.LoadFromEnv()
	if e != nil {
		log.Fatal("SMTP configuration invalid; check SMTP settings")
	}
	t := &service.Travel{Store: s, Map: maps, Planner: planner, AuthPolicy: policy}
	if smtpSender != nil {
		t.Mailer = smtpSender
	}
	addr, e := net.ResolveTCPAddr("tcp", config.Env("TRAVEL_ADDR", "127.0.0.1:8888"))
	if e != nil {
		log.Fatal("invalid travel address")
	}
	srv := travelservice.NewServer(&rpc.TravelHandler{Service: t}, server.WithServiceAddr(addr), server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "travel"}), server.WithMetaHandler(transmeta.ServerTTHeaderHandler), server.WithEnableContextTimeout(true))
	wg := t.StartWorkers(ctx, 2)
	go func() {
		if e := srv.Run(); e != nil {
			log.Print("travel RPC server stopped")
			cancel()
		}
	}()
	<-ctx.Done()
	srv.Stop()
	wg.Wait()
}
