package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/client"
	"github.com/fox-in-the-box-ai/fox-fleet/conformance/runtime/mock_llm"
	"github.com/fox-in-the-box-ai/fox-fleet/conformance/runtime/report"
	"github.com/fox-in-the-box-ai/fox-fleet/conformance/runtime/sut"
)

const (
	testAuthSecret = "conf-test-secret-7f3a"
	testPassword   = "conf-test-pass-9b2e"
)

type Suite struct {
	Image string
}

func (s *Suite) Run(ctx context.Context) (*report.Suite, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("conformance: docker client: %w", err)
	}
	defer cli.Close()

	result := &report.Suite{Image: s.Image}
	start := time.Now()

	secret := testAuthSecret
	password := testPassword

	s.runStandalone(ctx, cli, result)
	s.runBootInvariant(ctx, cli, result)
	s.runManaged(ctx, cli, result, secret, password)
	s.runSSE(ctx, cli, result, secret, password)
	s.runContractVersion(ctx, cli, result, secret, password)

	result.Elapsed = time.Since(start)
	return result, nil
}

func (s *Suite) runStandalone(ctx context.Context, cli *client.Client, result *report.Suite) {
	h, err := sut.Start(ctx, cli, sut.Config{
		Image: s.Image,
		Mode:  sut.Standalone,
	})
	if err != nil {
		for _, num := range []int{6, 7, 8, 9, 11, 15} {
			result.Add(report.Result{
				Number: num,
				Name:   standaloneCheckName(num),
				Status: report.Fail,
				Detail: fmt.Sprintf("standalone SUT failed to start: %v", err),
			})
		}
		return
	}
	defer h.Cleanup(ctx)

	result.Add(check06StandaloneEquivalence(ctx, h))
	result.Add(check07Health(ctx, h))
	result.Add(check08Readyz(ctx, h))
	result.Add(check09VersionStandalone(ctx, h))
	result.Add(check11CapabilitiesStandalone(ctx, h))
	result.Add(check15StandaloneBoot(ctx, h))
}

func (s *Suite) runBootInvariant(ctx context.Context, cli *client.Client, result *report.Suite) {
	result.Add(check01BootInvariant(ctx, cli, s.Image))
}

func (s *Suite) runManaged(ctx context.Context, cli *client.Client, result *report.Suite, secret, password string) {
	h, err := sut.Start(ctx, cli, sut.Config{
		Image:      s.Image,
		Mode:       sut.Managed,
		AuthSecret: secret,
		Password:   password,
	})
	if err != nil {
		for _, num := range []int{2, 3, 4, 5, 10, 12} {
			result.Add(report.Result{
				Number: num,
				Name:   managedCheckName(num),
				Status: report.Fail,
				Detail: fmt.Sprintf("managed SUT failed to start: %v", err),
			})
		}
		return
	}
	defer h.Cleanup(ctx)

	result.Add(check02ManagedValidAuth(ctx, h))
	result.Add(check03ManagedInvalidAuth(ctx, h))
	result.Add(check04ManagedSessionAuth(ctx, h))
	result.Add(check05ManagedNoAuth(ctx, h))
	result.Add(check10VersionManaged(ctx, h))
	result.Add(check12CapabilitiesManaged(ctx, h))
}

func (s *Suite) runSSE(ctx context.Context, cli *client.Client, result *report.Suite, secret, password string) {
	mock, err := mock_llm.Start()
	if err != nil {
		for _, num := range []int{13, 14} {
			result.Add(report.Result{
				Number: num,
				Name:   sseCheckName(num),
				Status: report.Skip,
				Detail: fmt.Sprintf("mock LLM failed to start: %v", err),
			})
		}
		return
	}
	defer mock.Close()

	h, err := sut.Start(ctx, cli, sut.Config{
		Image:      s.Image,
		Mode:       sut.Managed,
		AuthSecret: secret,
		Password:   password,
		ExtraEnv: map[string]string{
			"FOX_PROXY_ENDPOINT": mock.ContainerURL(),
		},
	})
	if err != nil {
		for _, num := range []int{13, 14} {
			result.Add(report.Result{
				Number: num,
				Name:   sseCheckName(num),
				Status: report.Skip,
				Detail: fmt.Sprintf("SSE SUT failed to start: %v", err),
			})
		}
		return
	}
	defer h.Cleanup(ctx)

	result.Add(check13SSEContractEvents(ctx, h))

	mock.SetFailNext()
	result.Add(check14SSEAppError(ctx, h))
}

func (s *Suite) runContractVersion(ctx context.Context, cli *client.Client, result *report.Suite, secret, password string) {
	h, err := sut.Start(ctx, cli, sut.Config{
		Image:      s.Image,
		Mode:       sut.Managed,
		AuthSecret: secret,
		Password:   password,
	})
	if err != nil {
		result.Add(report.Result{
			Number: 16,
			Name:   "Contract version",
			Status: report.Fail,
			Detail: fmt.Sprintf("SUT failed to start: %v", err),
		})
		return
	}
	defer h.Cleanup(ctx)

	result.Add(check16ContractVersion(ctx, h))
}

func standaloneCheckName(num int) string {
	names := map[int]string{
		6: "Standalone: vanilla equivalence", 7: "GET /health",
		8: "GET /readyz", 9: "GET /version (standalone)",
		11: "GET /capabilities (standalone)", 15: "Standalone boot",
	}
	return names[num]
}

func managedCheckName(num int) string {
	names := map[int]string{
		2: "Managed: valid X-Fox-Auth", 3: "Managed: invalid X-Fox-Auth",
		4: "Managed: valid session, no X-Fox-Auth", 5: "Managed: no auth",
		10: "GET /version (managed, no auth)", 12: "GET /capabilities (managed, no auth)",
	}
	return names[num]
}

func sseCheckName(num int) string {
	names := map[int]string{13: "SSE: contract events", 14: "SSE: apperror on provider error"}
	return names[num]
}
