package gateway

import (
	"ai-travel/internal/domain"
	"context"
	"github.com/cloudwego/hertz/pkg/app"
	"net"
	"time"
)

type emailInput struct {
	Email string `json:"email"`
}
type registerInput struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	Code        string `json:"code"`
	ChallengeID string `json:"challenge_id"`
}
type loginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type legacyInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type bindingInput struct {
	Email       string `json:"email"`
	Code        string `json:"code"`
	ChallengeID string `json:"challenge_id"`
}

// Never use ClientIP(), whose default strategy trusts forwarding headers.
func peerIP(c *app.RequestContext) string {
	if a := c.RemoteAddr(); a != nil {
		host, _, e := net.SplitHostPort(a.String())
		if e == nil && net.ParseIP(host) != nil {
			return host
		}
	}
	return "unknown"
}
func (g *handler) invokeAuth(ctx context.Context, c *app.RequestContext, method, rid string, r domain.Request, status int, timeout time.Duration) {
	c.Header("Cache-Control", "no-store")
	r.ClientIP = peerIP(c)
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	g.invoke(callCtx, c, method, rid, r, status, func(out domain.Response) any {
		switch method {
		case "Login", "LegacyLogin":
			return out.Session
		case "RequestRegistrationCode", "RequestEmailBindingCode":
			return out.Challenge
		default:
			return out.User
		}
	})
}
func (g *handler) register(ctx context.Context, c *app.RequestContext) {
	rid := domain.ID()
	var in registerInput
	if e := decode(c, &in); e != nil {
		writeBadRequest(c, rid, e)
		return
	}
	g.invokeAuth(ctx, c, "Register", rid, domain.Request{Email: in.Email, Password: in.Password, Code: in.Code, ChallengeID: in.ChallengeID}, 201, 5*time.Second)
}
func (g *handler) login(ctx context.Context, c *app.RequestContext) {
	rid := domain.ID()
	var in loginInput
	if e := decode(c, &in); e != nil {
		writeBadRequest(c, rid, e)
		return
	}
	g.invokeAuth(ctx, c, "Login", rid, domain.Request{Email: in.Email, Password: in.Password}, 200, 5*time.Second)
}
func (g *handler) legacyLogin(ctx context.Context, c *app.RequestContext) {
	rid := domain.ID()
	var in legacyInput
	if e := decode(c, &in); e != nil {
		writeBadRequest(c, rid, e)
		return
	}
	g.invokeAuth(ctx, c, "LegacyLogin", rid, domain.Request{Username: in.Username, Password: in.Password}, 200, 5*time.Second)
}
func (g *handler) sendRegistrationCode(ctx context.Context, c *app.RequestContext) {
	rid := domain.ID()
	var in emailInput
	if e := decode(c, &in); e != nil {
		writeBadRequest(c, rid, e)
		return
	}
	g.invokeAuth(ctx, c, "RequestRegistrationCode", rid, domain.Request{Email: in.Email}, 200, 15*time.Second)
}
func (g *handler) sendBindingCode(ctx context.Context, c *app.RequestContext, rid, token string) {
	var in emailInput
	if e := decode(c, &in); e != nil {
		writeBadRequest(c, rid, e)
		return
	}
	g.invokeAuth(ctx, c, "RequestEmailBindingCode", rid, domain.Request{Token: token, Email: in.Email}, 200, 15*time.Second)
}
func (g *handler) bindEmail(ctx context.Context, c *app.RequestContext, rid, token string) {
	var in bindingInput
	if e := decode(c, &in); e != nil {
		writeBadRequest(c, rid, e)
		return
	}
	g.invokeAuth(ctx, c, "BindEmail", rid, domain.Request{Token: token, Email: in.Email, Code: in.Code, ChallengeID: in.ChallengeID}, 200, 5*time.Second)
}
