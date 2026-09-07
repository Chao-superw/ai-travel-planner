package main

import (
	"ai-travel/internal/config"
	"ai-travel/internal/providers"
	"ai-travel/internal/rpc"
	"ai-travel/kitex_gen/planner/plannerservice"
	"context"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/transmeta"
	"github.com/cloudwego/kitex/server"
	"log"
	"net"
	"net/http"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	token, e := config.Require("PLANNER_SERVICE_TOKEN")
	if e != nil {
		log.Fatal(e)
	}
	base, e := config.ProviderURL("BIGMODEL_BASE_URL", "https://open.bigmodel.cn")
	if e != nil {
		log.Fatal(e)
	}
	glm := &providers.GLM{BaseURL: base, APIKey: config.Env("BIGMODEL_API_KEY", ""), Model: config.Env("BIGMODEL_MODEL", "glm-5.3-flash"), Client: &http.Client{Timeout: config.ModelHTTPTimeout}}
	addr, e := net.ResolveTCPAddr("tcp", config.Env("PLANNER_ADDR", "127.0.0.1:8889"))
	if e != nil {
		log.Fatal("invalid planner address")
	}
	srv := plannerservice.NewServer(&rpc.PlannerHandler{Model: glm, Token: token}, server.WithServiceAddr(addr), server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "planner"}), server.WithMetaHandler(transmeta.ServerTTHeaderHandler), server.WithEnableContextTimeout(true))
	go func() {
		if e := srv.Run(); e != nil {
			log.Print("planner RPC server stopped")
			cancel()
		}
	}()
	<-ctx.Done()
	srv.Stop()
}
