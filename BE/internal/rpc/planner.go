package rpc

import (
	"ai-travel/internal/config"
	"ai-travel/internal/domain"
	"ai-travel/internal/service"
	"ai-travel/kitex_gen/model"
	"ai-travel/kitex_gen/planner/plannerservice"
	"context"
	"crypto/subtle"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/transmeta"
	"github.com/cloudwego/kitex/transport"
	"time"
)

type PlannerHandler struct {
	Model service.Planner
	Token string
}

func (h *PlannerHandler) call(ctx context.Context, r *model.PlanningRequest, revise bool) (*model.PlanningResult_, error) {
	if r == nil || h.Token == "" || subtle.ConstantTimeCompare([]byte(r.GetServiceToken()), []byte(h.Token)) != 1 {
		return convert[model.PlanningResult_](domain.PlanningResult{Error: domain.Err(401, "UNAUTHENTICATED", "服务认证失败")})
	}
	req, e := convert[domain.PlanningRequest](r)
	if e != nil {
		return nil, e
	}
	var out domain.PlanningResult
	if revise {
		out, e = h.Model.Revise(ctx, *req)
	} else {
		out, e = h.Model.Generate(ctx, *req)
	}
	if e != nil {
		out.Error = service.AsError(e)
	}
	return convert[model.PlanningResult_](out)
}
func (h *PlannerHandler) GenerateItinerary(ctx context.Context, r *model.PlanningRequest) (*model.PlanningResult_, error) {
	return h.call(ctx, r, false)
}
func (h *PlannerHandler) ReviseItinerary(ctx context.Context, r *model.PlanningRequest) (*model.PlanningResult_, error) {
	return h.call(ctx, r, true)
}

type PlannerClient struct {
	Client plannerservice.Client
	Token  string
}

func NewPlannerClient(addr, token string) (*PlannerClient, error) {
	c, e := plannerservice.NewClient("planner", client.WithHostPorts(addr), client.WithTransportProtocol(transport.TTHeader), client.WithMetaHandler(transmeta.ClientTTHeaderHandler), client.WithRPCTimeout(config.PlannerRPCTimeout), client.WithConnectTimeout(time.Second))
	return &PlannerClient{c, token}, e
}
func (c *PlannerClient) call(ctx context.Context, r domain.PlanningRequest, revise bool) (domain.PlanningResult, error) {
	r.ServiceToken = c.Token
	req, e := convert[model.PlanningRequest](r)
	if e != nil {
		return domain.PlanningResult{}, e
	}
	var out *model.PlanningResult_
	if revise {
		out, e = c.Client.ReviseItinerary(ctx, req)
	} else {
		out, e = c.Client.GenerateItinerary(ctx, req)
	}
	if e != nil {
		return domain.PlanningResult{}, domain.Err(503, "PLANNER_UNAVAILABLE", "规划服务不可用或调用超时")
	}
	if out == nil {
		return domain.PlanningResult{}, domain.Err(503, "PLANNER_UNAVAILABLE", "规划服务返回空响应")
	}
	res, e := convert[domain.PlanningResult](out)
	if e != nil {
		return domain.PlanningResult{}, e
	}
	if res.Error != nil {
		return *res, res.Error
	}
	return *res, nil
}
func (c *PlannerClient) Generate(ctx context.Context, r domain.PlanningRequest) (domain.PlanningResult, error) {
	return c.call(ctx, r, false)
}
func (c *PlannerClient) Revise(ctx context.Context, r domain.PlanningRequest) (domain.PlanningResult, error) {
	return c.call(ctx, r, true)
}
