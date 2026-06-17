package provisioner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fox-in-the-box-ai/fox-fleet/internal/config"
	"github.com/fox-in-the-box-ai/fox-fleet/internal/registry"
	"github.com/fox-in-the-box-ai/fox-fleet/plugins"
	"github.com/fox-in-the-box-ai/fox-fleet/skillsets"
)

var (
	ErrCapReached    = errors.New("provisioner: max instances reached")
	ErrPortExhausted = errors.New("provisioner: no free port in range")
)

const (
	defaultMaxInstances   = 10
	defaultHealthTimeout  = 120 * time.Second
	defaultPortRangeStart = 9100
	maxPortScan           = 1000
	registryRetries       = 3
	registryRetryDelay    = 100 * time.Millisecond
)

type Request struct {
	InstanceID       string
	AdminSecret      string
	InstancePassword string
	Image            plugins.ImageRef
	ProxyEndpoint    string
	CapabilityFlags  map[string]bool
	Env              map[string]string
	SkillsetPath     string
	DataPlaneURL     string
	PrincipalRole    string
}

type Instance struct {
	ID        string
	Port      int
	DataDir   string
	Status    string
	CreatedAt time.Time
}

type Provisioner interface {
	Provision(ctx context.Context, req Request) (*Instance, error)
	Destroy(ctx context.Context, instanceID string, removeData bool) error
}

type Options struct {
	Registry       *registry.Registry
	Plugin         plugins.DeploymentPlugin
	ConfigWriter   func(config.InjectParams) error
	DataRoot       string
	PortRangeStart int
	MaxInstances   int
	HealthTimeout  time.Duration
	HealthInterval func(elapsed time.Duration) time.Duration
}

func DefaultHealthInterval(elapsed time.Duration) time.Duration {
	if elapsed < 30*time.Second {
		return 2 * time.Second
	}
	return 5 * time.Second
}

type service struct {
	mu             sync.Mutex
	registry       *registry.Registry
	plugin         plugins.DeploymentPlugin
	configWriter   func(config.InjectParams) error
	dataRoot       string
	portRangeStart int
	maxInstances   int
	healthTimeout  time.Duration
	healthInterval func(elapsed time.Duration) time.Duration
}

func New(opts Options) Provisioner {
	if opts.ConfigWriter == nil {
		opts.ConfigWriter = config.Inject
	}
	if opts.HealthTimeout == 0 {
		opts.HealthTimeout = defaultHealthTimeout
	}
	if opts.HealthInterval == nil {
		opts.HealthInterval = DefaultHealthInterval
	}
	if opts.MaxInstances == 0 {
		opts.MaxInstances = defaultMaxInstances
	}
	if opts.PortRangeStart == 0 {
		opts.PortRangeStart = defaultPortRangeStart
	}
	return &service{
		registry:       opts.Registry,
		plugin:         opts.Plugin,
		configWriter:   opts.ConfigWriter,
		dataRoot:       opts.DataRoot,
		portRangeStart: opts.PortRangeStart,
		maxInstances:   opts.MaxInstances,
		healthTimeout:  opts.HealthTimeout,
		healthInterval: opts.HealthInterval,
	}
}

func (s *service) Provision(ctx context.Context, req Request) (*Instance, error) {
	if err := config.ValidateSecrets(req.AdminSecret, req.InstancePassword); err != nil {
		return nil, fmt.Errorf("provisioner: %w", err)
	}

	var skillsetName string
	if req.SkillsetPath != "" {
		manifest, err := skillsets.LoadFile(req.SkillsetPath)
		if err != nil {
			return nil, fmt.Errorf("provisioner: validate skillset: %w", err)
		}
		skillsetName = manifest.Name
	}

	// --- Critical section: cap check + port alloc + registry reserve ---
	s.mu.Lock()

	instances, err := s.registry.List()
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("provisioner: list instances: %w", err)
	}
	if len(instances) >= s.maxInstances {
		s.mu.Unlock()
		return nil, ErrCapReached
	}

	for _, inst := range instances {
		if inst.ID == req.InstanceID {
			s.mu.Unlock()
			return nil, fmt.Errorf("provisioner: instance %s already exists", req.InstanceID)
		}
	}

	usedPorts, err := s.registry.UsedPorts()
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("provisioner: used ports: %w", err)
	}

	port := s.allocatePort(usedPorts)
	if port == 0 {
		s.mu.Unlock()
		return nil, ErrPortExhausted
	}

	dataDir := filepath.Join(s.dataRoot, "instances", req.InstanceID)
	now := time.Now().UTC()

	queryToken, err := registry.GenerateQueryToken()
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("provisioner: %w", err)
	}

	regInst := registry.Instance{
		ID:            req.InstanceID,
		ImageDigest:   req.Image.Digest,
		Port:          port,
		DataDir:       dataDir,
		Status:        "provisioning",
		CreatedAt:     now.Format(time.RFC3339),
		SkillsetName:  skillsetName,
		PrincipalRole: req.PrincipalRole,
		QueryToken:    queryToken,
	}
	if err := s.registry.Create(regInst); err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("provisioner: reserve instance: %w", err)
	}

	s.mu.Unlock()
	// --- End critical section ---

	instanceCfg := plugins.InstanceConfig{
		AuthSecret:       req.AdminSecret,
		InstancePassword: req.InstancePassword,
		ProxyEndpoint:    req.ProxyEndpoint,
		CapabilityFlags:  req.CapabilityFlags,
		Env:              req.Env,
		SkillsetPath:     req.SkillsetPath,
		DataPlaneURL:     req.DataPlaneURL,
		PrincipalRole:    req.PrincipalRole,
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		s.rollback(ctx, req.InstanceID, dataDir, false)
		return nil, fmt.Errorf("provisioner: create data dir: %w", err)
	}

	if req.SkillsetPath != "" {
		src, err := os.ReadFile(req.SkillsetPath)
		if err != nil {
			s.rollback(ctx, req.InstanceID, dataDir, false)
			return nil, fmt.Errorf("provisioner: read skillset: %w", err)
		}
		dst := filepath.Join(dataDir, "skillset.yaml")
		if err := os.WriteFile(dst, src, 0o644); err != nil {
			s.rollback(ctx, req.InstanceID, dataDir, false)
			return nil, fmt.Errorf("provisioner: copy skillset: %w", err)
		}
		instanceCfg.SkillsetPath = "/data/skillset.yaml"
	}

	if err := s.configWriter(config.InjectParams{
		DataDir:          dataDir,
		InstancePassword: req.InstancePassword,
		Config:           instanceCfg,
		QueryToken:       queryToken,
	}); err != nil {
		s.rollback(ctx, req.InstanceID, dataDir, false)
		return nil, fmt.Errorf("provisioner: inject config: %w", err)
	}

	healthCtx, healthCancel := context.WithTimeout(ctx, s.healthTimeout)
	defer healthCancel()

	if err := s.plugin.Provision(healthCtx, plugins.ProvisionRequest{
		InstanceID: req.InstanceID,
		Image:      req.Image,
		Config:     instanceCfg,
		Port:       port,
		DataDir:    dataDir,
	}); err != nil {
		s.rollback(ctx, req.InstanceID, dataDir, true)
		return nil, fmt.Errorf("provisioner: deploy: %w", err)
	}

	if err := config.MarkOnboardingComplete(dataDir); err != nil {
		s.rollback(ctx, req.InstanceID, dataDir, true)
		return nil, fmt.Errorf("provisioner: mark onboarding complete: %w", err)
	}

	if err := s.retryUpdateStatus(req.InstanceID, "running"); err != nil {
		s.rollback(ctx, req.InstanceID, dataDir, true)
		return nil, fmt.Errorf("provisioner: update status: %w", err)
	}

	return &Instance{
		ID:        req.InstanceID,
		Port:      port,
		DataDir:   dataDir,
		Status:    "running",
		CreatedAt: now,
	}, nil
}

func (s *service) Destroy(ctx context.Context, instanceID string, removeData bool) error {
	inst, err := s.registry.Get(instanceID)
	if err != nil {
		return fmt.Errorf("provisioner: %w", err)
	}

	if err := s.plugin.Destroy(ctx, instanceID); err != nil {
		return fmt.Errorf("provisioner: destroy container %s: %w", instanceID, err)
	}

	if removeData {
		if err := os.RemoveAll(inst.DataDir); err != nil {
			return fmt.Errorf("provisioner: remove data dir %s: %w", inst.DataDir, err)
		}
	}

	if err := s.registry.Delete(instanceID); err != nil {
		return fmt.Errorf("provisioner: delete registry entry %s: %w", instanceID, err)
	}

	return nil
}

func (s *service) allocatePort(used map[int]bool) int {
	for p := s.portRangeStart; p < s.portRangeStart+maxPortScan; p++ {
		if !used[p] {
			return p
		}
	}
	return 0
}

func (s *service) rollback(ctx context.Context, instanceID, dataDir string, destroyContainer bool) {
	if destroyContainer {
		_ = s.plugin.Destroy(context.WithoutCancel(ctx), instanceID)
	}
	_ = os.RemoveAll(dataDir)
	_ = s.registry.Delete(instanceID)
}

func (s *service) retryUpdateStatus(id, status string) error {
	var err error
	for attempt := 0; attempt < registryRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(registryRetryDelay)
		}
		err = s.registry.UpdateStatus(id, status)
		if err == nil {
			return nil
		}
	}
	return err
}
