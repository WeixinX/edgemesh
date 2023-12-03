package monitor

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

const (
	cAdvisorBaseURL = "/api/v2.0"
	cAdvisorSpec    = cAdvisorBaseURL + "/spec"
	cAdvisorSummary = cAdvisorBaseURL + "/summary"
)

type CAdvisorClient struct {
	c      *http.Client
	target string
}

func NewCAdvisorClient(target string) *CAdvisorClient {
	return &CAdvisorClient{
		c:      http.DefaultClient,
		target: target,
	}
}

func (cc *CAdvisorClient) requestCAdvisorSpec(ctx context.Context, containerPath string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cAdvisorSpecURL(cc.target, containerPath), nil)
	if err != nil {
		return nil, err
	}
	return cc.doHttpRequest(req)
}

func (cc *CAdvisorClient) requestCAdvisorSummary(ctx context.Context, containerPath string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cAdvisorSummaryURL(cc.target, containerPath), nil)
	if err != nil {
		return nil, err
	}
	return cc.doHttpRequest(req)
}

func (cc *CAdvisorClient) doHttpRequest(req *http.Request) ([]byte, error) {
	resp, err := cc.c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func cAdvisorSpecURL(target, containerPath string) string {
	return fmt.Sprintf("%s/%s%s", target, cAdvisorSpec, containerPath)
}

func cAdvisorSummaryURL(target, containerPath string) string {
	return fmt.Sprintf("%s/%s%s", target, cAdvisorSummary, containerPath)
}
