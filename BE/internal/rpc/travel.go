package rpc

import (
	"ai-travel/internal/domain"
	"ai-travel/internal/service"
	"ai-travel/kitex_gen/model"
	"ai-travel/kitex_gen/travel/travelservice"
	"context"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/cloudwego/kitex/pkg/transmeta"
	"github.com/cloudwego/kitex/transport"
	"time"
)

type TravelHandler struct{ Service *service.Travel }

func (h *TravelHandler) invoke(ctx context.Context, r *model.Request, method string) (*model.Response, error) {
	if r == nil {
		return convert[model.Response](domain.Response{Error: domain.Err(400, "INVALID_INPUT", "缺少请求")})
	}
	req, e := convert[domain.Request](r)
	if e != nil {
		return nil, e
	}
	return convert[model.Response](h.Service.Handle(ctx, method, *req))
}
func (h *TravelHandler) Register(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "Register")
}
func (h *TravelHandler) Login(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "Login")
}
func (h *TravelHandler) Logout(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "Logout")
}
func (h *TravelHandler) GetCurrentUser(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "GetCurrentUser")
}
func (h *TravelHandler) SearchPlaces(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "SearchPlaces")
}
func (h *TravelHandler) CreatePlace(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "CreatePlace")
}
func (h *TravelHandler) UpdatePlace(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "UpdatePlace")
}
func (h *TravelHandler) CreatePlanningJob(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "CreatePlanningJob")
}
func (h *TravelHandler) GetPlanningJob(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "GetPlanningJob")
}
func (h *TravelHandler) RetryPlanningJob(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "RetryPlanningJob")
}
func (h *TravelHandler) ListTrips(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "ListTrips")
}
func (h *TravelHandler) GetTrip(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "GetTrip")
}
func (h *TravelHandler) UpdateTrip(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "UpdateTrip")
}
func (h *TravelHandler) CreateReplanningJob(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "CreateReplanningJob")
}
func (h *TravelHandler) ListTripVersions(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "ListTripVersions")
}
func (h *TravelHandler) GetTripVersion(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "GetTripVersion")
}
func (h *TravelHandler) GetBudgetSummary(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "GetBudgetSummary")
}
func (h *TravelHandler) Health(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "Health")
}
func (h *TravelHandler) RequestRegistrationCode(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "RequestRegistrationCode")
}
func (h *TravelHandler) LegacyLogin(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "LegacyLogin")
}
func (h *TravelHandler) RequestEmailBindingCode(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "RequestEmailBindingCode")
}
func (h *TravelHandler) BindEmail(ctx context.Context, r *model.Request) (*model.Response, error) {
	return h.invoke(ctx, r, "BindEmail")
}

type TravelClient struct{ Client travelservice.Client }

func NewTravelClient(addr string) (*TravelClient, error) {
	c, e := travelservice.NewClient("travel", client.WithHostPorts(addr), client.WithTransportProtocol(transport.TTHeader), client.WithMetaHandler(transmeta.ClientTTHeaderHandler), client.WithRPCTimeout(3*time.Second), client.WithConnectTimeout(time.Second))
	return &TravelClient{c}, e
}
func (c *TravelClient) Call(ctx context.Context, method string, r domain.Request) (domain.Response, error) {
	req, e := convert[model.Request](r)
	if e != nil {
		return domain.Response{}, e
	}
	timeout := 3 * time.Second
	if method == "SearchPlaces" || method == "CreatePlace" || method == "UpdatePlace" {
		timeout = 7 * time.Second
	}
	if method == "RequestRegistrationCode" || method == "RequestEmailBindingCode" {
		timeout = 14 * time.Second
	}
	opts := []callopt.Option{callopt.WithRPCTimeout(timeout)}
	var out *model.Response
	switch method {
	case "Register":
		out, e = c.Client.Register(ctx, req, opts...)
	case "Login":
		out, e = c.Client.Login(ctx, req, opts...)
	case "Logout":
		out, e = c.Client.Logout(ctx, req, opts...)
	case "GetCurrentUser":
		out, e = c.Client.GetCurrentUser(ctx, req, opts...)
	case "SearchPlaces":
		out, e = c.Client.SearchPlaces(ctx, req, opts...)
	case "CreatePlace":
		out, e = c.Client.CreatePlace(ctx, req, opts...)
	case "UpdatePlace":
		out, e = c.Client.UpdatePlace(ctx, req, opts...)
	case "CreatePlanningJob":
		out, e = c.Client.CreatePlanningJob(ctx, req, opts...)
	case "GetPlanningJob":
		out, e = c.Client.GetPlanningJob(ctx, req, opts...)
	case "RetryPlanningJob":
		out, e = c.Client.RetryPlanningJob(ctx, req, opts...)
	case "ListTrips":
		out, e = c.Client.ListTrips(ctx, req, opts...)
	case "GetTrip":
		out, e = c.Client.GetTrip(ctx, req, opts...)
	case "UpdateTrip":
		out, e = c.Client.UpdateTrip(ctx, req, opts...)
	case "CreateReplanningJob":
		out, e = c.Client.CreateReplanningJob(ctx, req, opts...)
	case "ListTripVersions":
		out, e = c.Client.ListTripVersions(ctx, req, opts...)
	case "GetTripVersion":
		out, e = c.Client.GetTripVersion(ctx, req, opts...)
	case "GetBudgetSummary":
		out, e = c.Client.GetBudgetSummary(ctx, req, opts...)
	case "Health":
		out, e = c.Client.Health(ctx, req, opts...)
	case "RequestRegistrationCode":
		out, e = c.Client.RequestRegistrationCode(ctx, req, opts...)
	case "LegacyLogin":
		out, e = c.Client.LegacyLogin(ctx, req, opts...)
	case "RequestEmailBindingCode":
		out, e = c.Client.RequestEmailBindingCode(ctx, req, opts...)
	case "BindEmail":
		out, e = c.Client.BindEmail(ctx, req, opts...)
	default:
		return domain.Response{}, domain.Err(404, "NOT_FOUND", "RPC method not found")
	}
	if e != nil {
		return domain.Response{}, e
	}
	if out == nil {
		return domain.Response{}, domain.Err(503, "RPC_UNAVAILABLE", "RPC returned empty response")
	}
	res, e := convert[domain.Response](out)
	if e != nil {
		return domain.Response{}, e
	}
	return *res, nil
}
