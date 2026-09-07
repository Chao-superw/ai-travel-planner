#!/usr/bin/env python3
import pathlib,re
root=pathlib.Path(__file__).resolve().parents[1]
methods=re.findall(r'model.Response (\w+)\(', (root/'idl/travel.thrift').read_text())
s='''package rpc
import("context";"time";"ai-travel/internal/domain";"ai-travel/internal/service";"ai-travel/kitex_gen/model";"ai-travel/kitex_gen/travel/travelservice";"github.com/cloudwego/kitex/client";"github.com/cloudwego/kitex/client/callopt";"github.com/cloudwego/kitex/pkg/transmeta";"github.com/cloudwego/kitex/transport")
type TravelHandler struct{Service *service.Travel}
func(h *TravelHandler)invoke(ctx context.Context,r *model.Request,method string)(*model.Response,error){if r==nil{return convert[model.Response](domain.Response{Error:domain.Err(400,"INVALID_INPUT","缺少请求")})};req,e:=convert[domain.Request](r);if e!=nil{return nil,e};return convert[model.Response](h.Service.Handle(ctx,method,*req))}
'''
for m in methods:s+=f'func(h *TravelHandler){m}(ctx context.Context,r *model.Request)(*model.Response,error){{return h.invoke(ctx,r,"{m}")}}\n'
s+='''type TravelClient struct{Client travelservice.Client}
func NewTravelClient(addr string)(*TravelClient,error){c,e:=travelservice.NewClient("travel",client.WithHostPorts(addr),client.WithTransportProtocol(transport.TTHeader),client.WithMetaHandler(transmeta.ClientTTHeaderHandler),client.WithRPCTimeout(3*time.Second),client.WithConnectTimeout(time.Second));return &TravelClient{c},e}
func(c *TravelClient)Call(ctx context.Context,method string,r domain.Request)(domain.Response,error){req,e:=convert[model.Request](r);if e!=nil{return domain.Response{},e};timeout:=3*time.Second;if method=="SearchPlaces"||method=="CreatePlace"||method=="UpdatePlace"{timeout=7*time.Second};if method=="RequestRegistrationCode"||method=="RequestEmailBindingCode"{timeout=14*time.Second};opts:=[]callopt.Option{callopt.WithRPCTimeout(timeout)};var out *model.Response
switch method {
'''
for m in methods:s+=f'case "{m}":out,e=c.Client.{m}(ctx,req,opts...)\n'
s+='''default:return domain.Response{},domain.Err(404,"NOT_FOUND","RPC method not found")
};if e!=nil{return domain.Response{},e};if out==nil{return domain.Response{},domain.Err(503,"RPC_UNAVAILABLE","RPC returned empty response")};res,e:=convert[domain.Response](out);if e!=nil{return domain.Response{},e};return *res,nil}
'''
(root/'internal/rpc/travel.go').write_text(s)
