package driverjob

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/service/processor/models"

	"github.com/docker/docker/api/types/mount"
)

const (
	// Label keys for Docker Swarm services
	labelProcessorID = "sentio.processor.id"
	labelProjectID   = "sentio.processor.project_id"
	labelVersion     = "sentio.processor.version"
	labelManagedBy   = "sentio.managed_by"
)

// DockerSwarmManager implements DriverJobManager for Docker Swarm
type DockerSwarmManager struct {
	dockerClient *client.Client
	config       DriverConfig
}

// DockerSwarmTask implements the Pod interface for Docker Swarm tasks
type DockerSwarmTask struct {
	task swarm.Task
}

func (t *DockerSwarmTask) Name() string {
	return t.task.ID
}

func (t *DockerSwarmTask) Namespace() string {
	if ns, ok := t.task.Spec.ContainerSpec.Labels[labelManagedBy]; ok {
		return ns
	}
	return ""
}

func (t *DockerSwarmTask) PodIP() string {
	// In Docker Swarm, tasks don't have a fixed IP in the same way K8s pods do
	// Return the node IP or task ID as fallback
	for _, na := range t.task.NetworksAttachments {
		if len(na.Addresses) > 0 {
			// Extract IP from CIDR notation
			addr := na.Addresses[0]
			if idx := strings.Index(addr, "/"); idx > 0 {
				return addr[:idx]
			}
			return addr
		}
	}
	return ""
}

// NewDockerSwarmManager creates a new Docker Swarm manager
func NewDockerSwarmManager(ctx context.Context, config DriverConfig) (*DockerSwarmManager, error) {
	// Create a new Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Verify we're connected to a Swarm cluster
	info, err := cli.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Docker info: %w", err)
	}

	if !info.Swarm.ControlAvailable {
		log.Warn("Docker is not running in Swarm mode - manager functionality may be limited")
	}

	return &DockerSwarmManager{
		dockerClient: cli,
		config:       config,
	}, nil
}

const ChainConfigMountPath = "/etc/sentio/chains-config.json"
const ClickhouseConfigMountPath = "/etc/sentio/clickhouse_config.yaml"
const CacheDirMountPath = "/tmp/sentio/cache"

// buildDriverServiceSpec creates a Docker Swarm service specification for the driver
func (d *DockerSwarmManager) buildDriverServiceSpec(processor *models.Processor, networkID string) (swarm.ServiceSpec, error) {
	labels := map[string]string{
		labelProcessorID:      processor.ID,
		labelProjectID:        processor.ProjectID,
		labelVersion:          strconv.Itoa(int(processor.Version)),
		labelManagedBy:        "sentio-core",
		"sentio.service.type": "driver",
	}

	processorServiceName := d.makeProcessorServiceName(processor)

	// Construct command line arguments
	args := []string{
		fmt.Sprintf("-processor-service=%s", d.config.ProcessorService),
		fmt.Sprintf("-webhook-service=%s", ""),
		fmt.Sprintf("-billing-server=%s", ""),
		fmt.Sprintf("-rpcnode-service=%s", ""),
		fmt.Sprintf("-pubsub-topic=%s", ""),
		fmt.Sprintf("-timescale-db-config=%s", ""),
		// Point to the processor service on the internal network
		fmt.Sprintf("-external-processor=%s:9999", processorServiceName),
		fmt.Sprintf("-processor-id=%s", processor.ID),
		fmt.Sprintf("-redis=%s", d.config.Redis.Address),
		fmt.Sprintf("-cache-dir=%s", CacheDirMountPath),
		fmt.Sprintf("-chains-config=%s", ChainConfigMountPath),
		fmt.Sprintf("-clickhouse-config-path=%s", ClickhouseConfigMountPath),
	}

	if d.config.Driver.LogFormat != "" {
		args = append(args, fmt.Sprintf("-log-format=%s", d.config.Driver.LogFormat))
	}
	if d.config.Driver.RealtimeProcessingOwnerWhitelist != "" {
		args = append(args, fmt.Sprintf("-realtime-processing-owner-whitelist=%s", d.config.Driver.RealtimeProcessingOwnerWhitelist))
	}
	if d.config.Driver.AllowSingleBlockBackfillOwnerWhitelist != "" {
		args = append(args, fmt.Sprintf("-allow-single-block-backfill-owner-whitelist=%s", d.config.Driver.AllowSingleBlockBackfillOwnerWhitelist))
	}
	if d.config.Driver.Verbose != "" {
		args = append(args, fmt.Sprintf("-verbose=%s", d.config.Driver.Verbose))
	}
	if d.config.Driver.EntityStoreCacheSize > 0 {
		args = append(args, fmt.Sprintf("-entity-store-cache-size=%d", d.config.Driver.EntityStoreCacheSize))
	}
	if d.config.Driver.ProcessorUseChainServer {
		args = append(args, "-use-chain-server=true")
	}
	if d.config.Driver.SamplingInterval > 0 {
		args = append(args, fmt.Sprintf("-sampling-interval=%d", d.config.Driver.SamplingInterval))
	}
	if d.config.Clickhouse.ReadTimeout > 0 {
		args = append(args, fmt.Sprintf("-clickhouse-read-timeout=%d", d.config.Clickhouse.ReadTimeout))
	}
	if d.config.Clickhouse.DialTimeout > 0 {
		args = append(args, fmt.Sprintf("-clickhouse-dial-timeout=%d", d.config.Clickhouse.DialTimeout))
	}
	if d.config.Clickhouse.MaxIdleConns > 0 {
		args = append(args, fmt.Sprintf("-clickhouse-max-idle-conns=%d", d.config.Clickhouse.MaxIdleConns))
	}
	if d.config.Clickhouse.MaxOpenConns > 0 {
		args = append(args, fmt.Sprintf("-clickhouse-max-open-conns=%d", d.config.Clickhouse.MaxOpenConns))
	}
	if d.config.Redis.PoolSize > 0 {
		args = append(args, fmt.Sprintf("-redis-pool=%d", d.config.Redis.PoolSize))
	}

	replicas := uint64(1)
	if processor.Pause {
		replicas = 0
	}

	name := d.makeDriverServiceName(processor)

	var mounts []mount.Mount
	if d.config.ChainsConfig != "" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: d.config.ChainsConfig,
			Target: ChainConfigMountPath,
		})
	}

	if d.config.Clickhouse.ConfigPath != "" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: d.config.Clickhouse.ConfigPath,
			Target: ClickhouseConfigMountPath,
		})
	}

	if d.config.CacheDir != "" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: d.config.CacheDir,
			Target: CacheDirMountPath,
		})
	}
	return swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name:   name,
			Labels: labels,
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: &swarm.ContainerSpec{
				Image:   d.config.DriverImage,
				Labels:  labels,
				Command: []string{"/app/driver/cmd/cmd_/cmd"},
				Args:    args,
				Mounts:  mounts,
			},
			RestartPolicy: &swarm.RestartPolicy{
				Condition: swarm.RestartPolicyConditionOnFailure,
			},
			Networks: []swarm.NetworkAttachmentConfig{
				{Target: networkID},
			},
		},
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &replicas,
			},
		},
	}, nil
}

// buildProcessorServiceSpec creates a Docker Swarm service specification for the processor
func (d *DockerSwarmManager) buildProcessorServiceSpec(processor *models.Processor, networkID string, scriptPath string) (swarm.ServiceSpec, error) {
	labels := map[string]string{
		labelProcessorID:      processor.ID,
		labelProjectID:        processor.ProjectID,
		labelVersion:          strconv.Itoa(int(processor.Version)),
		labelManagedBy:        "sentio-core",
		"sentio.service.type": "processor",
	}

	replicas := uint64(1)
	if processor.Pause {
		replicas = 0
	}

	name := d.makeProcessorServiceName(processor)

	mounts := []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: scriptPath,
			Target: "/start-processor.sh",
		},
	}

	if d.config.ChainsConfig != "" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: d.config.ChainsConfig,
			Target: ChainConfigMountPath,
		})
	}

	if d.config.Clickhouse.ConfigPath != "" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: d.config.Clickhouse.ConfigPath,
			Target: ClickhouseConfigMountPath,
		})
	}

	if d.config.CacheDir != "" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: d.config.CacheDir,
			Target: CacheDirMountPath,
		})
	}

	return swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name:   name,
			Labels: labels,
		},
		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: &swarm.ContainerSpec{
				Image:   d.config.DriverImage,
				Labels:  labels,
				Command: []string{"/bin/sh", "/start-processor.sh"},
				Mounts:  mounts,
			},
			RestartPolicy: &swarm.RestartPolicy{
				Condition: swarm.RestartPolicyConditionOnFailure,
			},
			Networks: []swarm.NetworkAttachmentConfig{
				{Target: networkID},
			},
		},
		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: &replicas,
			},
		},
	}, nil
}

func (d *DockerSwarmManager) createNetwork(ctx context.Context, processor *models.Processor) (string, error) {
	networkName := fmt.Sprintf("%s-network", d.makeDriverName(processor))

	// Check if network exists
	networks, err := d.dockerClient.NetworkList(ctx, network.ListOptions{
		Filters: filters.NewArgs(filters.Arg("name", networkName)),
	})
	if err != nil {
		return "", err
	}

	if len(networks) > 0 {
		return networks[0].ID, nil
	}

	// Create network
	resp, err := d.dockerClient.NetworkCreate(ctx, networkName, network.CreateOptions{
		Driver: "overlay",
		Labels: map[string]string{
			labelProcessorID: processor.ID,
			labelManagedBy:   "sentio-core",
		},
	})
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

const processorScriptTemplate = `#!/bin/sh
cd /app/driver/cmd/cmd_/cmd.runfiles/_main
# prepare
/app/driver/cmd/cmd_/cmd -processor-service={{.ProcessorService}} \
        -cache-dir={{.CacheDirMountPath}} \
        -prepare-processor-env-only=true \
        -chains-config={{.ChainConfigMountPath}}  \
        -processor-id={{.ProcessorID}} \
        -use-pnpm=true \
        -clickhouse-config-path={{.ClickhouseConfigPath}}

# prepare2

mkdir -p {{.CacheDir}}/.pnpm-store/dumps && chmod 777 {{.CacheDir}}/.pnpm-store/dumps

TARGET_PATH=$(tail -n 1 {{.CacheDir}}/.processor-path)

if [[ "$TARGET_PATH" == */main ]]; then
    chmod +x "$TARGET_PATH"
fi

# start processor
TARGET_DIR=$(head -n 1 {{.CacheDir}}/.processor-path)
TARGET_PATH=$(tail -n 1 {{.CacheDir}}/.processor-path)
CHAINS_CONFIG_FILE=${TARGET_DIR}/chains-config.json
if [[ "$TARGET_PATH" == */main ]]; then
    exec "$TARGET_PATH" "$@" --chains-config=$CHAINS_CONFIG_FILE
else
    exec /usr/bin/env node --inspect=0.0.0.0:9229 $TARGET_DIR/node_modules/.bin/processor-runner --port 9999 --use-chainserver --concurrency=128 --chains-config=$CHAINS_CONFIG_FILE $TARGET_PATH 
fi
`

func (d *DockerSwarmManager) generateProcessorScript(processor *models.Processor) (string, error) {
	data := struct {
		ProcessorService     string
		CacheDirMountPath    string
		ChainConfigMountPath string
		ProcessorID          string
		ClickhouseConfigPath string
		CacheDir             string
	}{
		ProcessorService:     d.config.ProcessorService,
		CacheDirMountPath:    CacheDirMountPath,
		ChainConfigMountPath: ChainConfigMountPath,
		ProcessorID:          processor.ID,
		ClickhouseConfigPath: ClickhouseConfigMountPath,
		CacheDir:             CacheDirMountPath,
	}

	tmpl, err := template.New("processor-script").Parse(processorScriptTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse processor script template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute processor script template: %w", err)
	}

	// Write to temp file
	tmpDir := os.TempDir()
	fileName := fmt.Sprintf("start-processor-%s.sh", processor.ID)
	filePath := path.Join(tmpDir, fileName)

	err = os.WriteFile(filePath, buf.Bytes(), 0755)
	if err != nil {
		return "", err
	}

	return filePath, nil
}

// StartJob starts a driver job for the given processor using Docker Swarm
func (d *DockerSwarmManager) StartJob(ctx context.Context, processor *models.Processor) error {
	// 1. Create Network
	networkID, err := d.createNetwork(ctx, processor)
	if err != nil {
		return fmt.Errorf("failed to create network for processor %s: %w", processor.ID, err)
	}

	// 2. Generate Processor Script
	scriptPath, err := d.generateProcessorScript(processor)
	if err != nil {
		return fmt.Errorf("failed to generate processor script for processor %s: %w", processor.ID, err)
	}

	// 3. Create Processor Service
	procSpec, err := d.buildProcessorServiceSpec(processor, networkID, scriptPath)
	if err != nil {
		return err
	}

	createOptions := types.ServiceCreateOptions{}
	procResp, err := d.dockerClient.ServiceCreate(ctx, procSpec, createOptions)
	if err != nil {
		return fmt.Errorf("failed to create processor service for processor %s: %w", processor.ID, err)
	}

	// 4. Create Driver Service
	driverSpec, err := d.buildDriverServiceSpec(processor, networkID)
	if err != nil {
		return err
	}

	driverResp, err := d.dockerClient.ServiceCreate(ctx, driverSpec, createOptions)
	if err != nil {
		return fmt.Errorf("failed to create driver service for processor %s: %w", processor.ID, err)
	}

	log.Infow("Created Docker Swarm services for processor",
		"processorID", processor.ID,
		"driverServiceID", driverResp.ID,
		"processorServiceID", procResp.ID,
		"networkID", networkID,
	)

	return nil
}

// RestartJob restarts a driver job for the given processor
func (d *DockerSwarmManager) RestartJob(ctx context.Context, processor *models.Processor) error {
	driverName := d.makeDriverServiceName(processor)

	if err := d.restartServiceByName(ctx, driverName, processor); err != nil {
		return err
	}

	procName := d.makeProcessorServiceName(processor)

	if err := d.restartServiceByName(ctx, procName, processor); err != nil {
		return err
	}

	return nil
}

func (d *DockerSwarmManager) restartServiceByName(ctx context.Context, serviceName string, processor *models.Processor) error {
	// Get the service
	service, _, err := d.dockerClient.ServiceInspectWithRaw(ctx, serviceName, types.ServiceInspectOptions{})
	if err != nil {
		if client.IsErrNotFound(err) {
			// Service doesn't exist, create it
			// Note: This falls back to StartJob which creates both services.
			// This might be redundant if one exists and other doesn't, but acceptable.
			return d.StartJob(ctx, processor)
		}
		return fmt.Errorf("failed to inspect service %s: %w", serviceName, err)
	}

	// Force update the service to trigger a restart
	service.Spec.TaskTemplate.ForceUpdate++
	if strings.HasSuffix(serviceName, "-driver") {
		// Rebuild driver spec
		networkID := service.Spec.TaskTemplate.Networks[0].Target
		service.Spec, err = d.buildDriverServiceSpec(processor, networkID)
		if err != nil {
			return err
		}
	}
	if strings.HasSuffix(serviceName, "-processor") {
		// Rebuild processor spec
		networkID := service.Spec.TaskTemplate.Networks[0].Target
		scriptPath, err := d.generateProcessorScript(processor)
		if err != nil {
			return err
		}
		service.Spec, err = d.buildProcessorServiceSpec(processor, networkID, scriptPath)
		if err != nil {
			return err
		}
	}
	resp, err := d.dockerClient.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, types.ServiceUpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to restart service %s: %w", serviceName, err)
	}

	log.Infow("Restarted Docker Swarm service",
		"processorID", processor.ID,
		"serviceID", service.ID,
		"serviceName", serviceName,
		"warnings", resp.Warnings,
	)
	return nil
}

// DeleteJob deletes a driver job for the given processor
func (d *DockerSwarmManager) DeleteJob(ctx context.Context, processor *models.Processor) error {
	// Delete Driver Service
	driverName := d.makeDriverServiceName(processor)

	if err := d.deleteServiceByName(ctx, driverName, processor.ID); err != nil {
		return err
	}

	// Delete Processor Service
	procName := d.makeProcessorServiceName(processor)
	if err := d.deleteServiceByName(ctx, procName, processor.ID); err != nil {
		return err
	}

	// Delete Network
	networkName := d.makeDriverName(processor) + "-network"
	if err := d.dockerClient.NetworkRemove(ctx, networkName); err != nil {
		if !client.IsErrNotFound(err) {
			log.Warnw("Failed to remove network", "network", networkName, "error", err)
		}
	}

	return nil
}

func (d *DockerSwarmManager) deleteServiceByName(ctx context.Context, serviceName string, processorID string) error {
	err := d.dockerClient.ServiceRemove(ctx, serviceName)
	if err != nil {
		if client.IsErrNotFound(err) {
			log.Debugw("Service not found during deletion",
				"processorID", processorID,
				"serviceName", serviceName,
			)
			return nil
		}
		return fmt.Errorf("failed to delete service %s: %w", serviceName, err)
	}

	log.Infow("Deleted Docker Swarm service",
		"processorID", processorID,
		"serviceName", serviceName,
	)
	return nil
}

// StartOrUpdateDriverJob creates or updates a driver job for the given processor
func (d *DockerSwarmManager) StartOrUpdateDriverJob(ctx context.Context, processor *models.Processor) error {
	// Ensure network exists
	networkID, err := d.createNetwork(ctx, processor)
	if err != nil {
		return fmt.Errorf("failed to create/get network: %w", err)
	}

	// Generate script
	scriptPath, err := d.generateProcessorScript(processor)
	if err != nil {
		return fmt.Errorf("failed to generate script: %w", err)
	}

	// Update Driver
	driverName := d.makeDriverServiceName(processor)
	driverSpec, err := d.buildDriverServiceSpec(processor, networkID)
	if err != nil {
		return err
	}
	if err := d.createOrUpdateService(ctx, driverName, driverSpec, processor); err != nil {
		return err
	}

	// Update Processor
	procName := d.makeProcessorServiceName(processor)
	procSpec, err := d.buildProcessorServiceSpec(processor, networkID, scriptPath)
	if err != nil {
		return err
	}
	if err := d.createOrUpdateService(ctx, procName, procSpec, processor); err != nil {
		return err
	}

	return nil
}

func (d *DockerSwarmManager) createOrUpdateService(ctx context.Context, serviceName string, spec swarm.ServiceSpec, processor *models.Processor) error {
	service, _, err := d.dockerClient.ServiceInspectWithRaw(ctx, serviceName, types.ServiceInspectOptions{})
	if err != nil {
		if client.IsErrNotFound(err) {
			// Create
			resp, err := d.dockerClient.ServiceCreate(ctx, spec, types.ServiceCreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create service %s: %w", serviceName, err)
			}
			log.Infow("Created Docker Swarm service", "serviceName", serviceName, "id", resp.ID)
			return nil
		}
		return fmt.Errorf("failed to inspect service %s: %w", serviceName, err)
	}

	// Update
	resp, err := d.dockerClient.ServiceUpdate(ctx, service.ID, service.Version, spec, types.ServiceUpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update service %s: %w", serviceName, err)
	}
	log.Infow("Updated Docker Swarm service", "serviceName", serviceName, "warnings", resp.Warnings)
	return nil
}

// StartManager starts the Docker Swarm manager
func (d *DockerSwarmManager) StartManager(ctx context.Context) error {
	// Verify Swarm is active
	info, err := d.dockerClient.Info(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Docker info: %w", err)
	}

	if !info.Swarm.ControlAvailable {
		return fmt.Errorf("docker Swarm is not initialized or this node is not a manager")
	}

	log.Infow("Docker Swarm manager started successfully",
		"nodeID", info.Swarm.NodeID,
		"clusterID", info.Swarm.Cluster.ID,
	)

	return nil
}

// IsProcessorRunning checks if the processor is currently running
func (d *DockerSwarmManager) IsProcessorRunning(processorID string, _ int) bool {
	ctx := context.Background()

	services, err := d.findServicesByProcessorID(ctx, processorID)
	if err != nil {
		return false
	}

	// Check if any service has running replicas
	for _, service := range services {
		if service.Spec.Mode.Replicated != nil && service.Spec.Mode.Replicated.Replicas != nil {
			if *service.Spec.Mode.Replicated.Replicas > 0 {
				return true
			}
		}
	}

	return false
}

// RestartProcessorByID restarts a specific processor
func (d *DockerSwarmManager) RestartProcessorByID(ctx context.Context, processorID string, k8sClusterID int) error {
	services, err := d.findServicesByProcessorID(ctx, processorID)
	if err != nil {
		return fmt.Errorf("failed to find services for processor %s: %w", processorID, err)
	}

	for _, service := range services {
		// Force update to trigger restart
		service.Spec.TaskTemplate.ForceUpdate++

		_, err = d.dockerClient.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, types.ServiceUpdateOptions{})
		if err != nil {
			log.Warnw("Failed to restart service", "serviceID", service.ID, "error", err)
			continue
		}
	}

	log.Infow("Restarted processor by ID",
		"processorID", processorID,
		"clusterID", k8sClusterID,
	)

	return nil
}

// RestartWaitingProcessorsByOwner restarts waiting processors owned by the given owner
func (d *DockerSwarmManager) RestartWaitingProcessorsByOwner(ctx context.Context, ownerID, ownerType string) error {
	// List all services with the managed-by label
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("%s=sentio-core", labelManagedBy))

	services, err := d.dockerClient.ServiceList(ctx, types.ServiceListOptions{
		Filters: filterArgs,
	})
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	// Restart each service
	for _, service := range services {
		service.Spec.TaskTemplate.ForceUpdate++
		_, err := d.dockerClient.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, types.ServiceUpdateOptions{})
		if err != nil {
			log.Warnw("Failed to restart service",
				"serviceID", service.ID,
				"error", err,
			)
			continue
		}
	}

	log.Infow("Restarted waiting processors by owner",
		"ownerID", ownerID,
		"ownerType", ownerType,
		"count", len(services),
	)

	return nil
}

// HasCluster checks if the given cluster ID exists
func (d *DockerSwarmManager) HasCluster(clusterID int) bool {
	return clusterID == 0
}

// ListDriverJobPodsByProcessor lists driver job pods for the given processor
func (d *DockerSwarmManager) ListDriverJobPodsByProcessor(ctx context.Context, processorID string) (map[int][]Pod, error) {
	// Find services by processor ID
	services, err := d.findServicesByProcessorID(ctx, processorID)
	if err != nil {
		if strings.Contains(err.Error(), "services not found") {
			return make(map[int][]Pod), nil
		}
		return nil, err
	}

	pods := make([]Pod, 0)

	for _, service := range services {
		// List tasks for this service
		filterArgs := filters.NewArgs()
		filterArgs.Add("service", service.ID)

		tasks, err := d.dockerClient.TaskList(ctx, types.TaskListOptions{
			Filters: filterArgs,
		})
		if err != nil {
			log.Warnw("Failed to list tasks for service", "serviceID", service.ID, "error", err)
			continue
		}

		// Convert tasks to pods
		for _, task := range tasks {
			// Only include running tasks
			if task.Status.State == swarm.TaskStateRunning {
				pods = append(pods, &DockerSwarmTask{task: task})
			}
		}
	}

	clusterID := 0

	result := make(map[int][]Pod)
	result[clusterID] = pods

	return result, nil
}

// GetNamespaceForCluster returns the namespace for the given cluster ID
func (d *DockerSwarmManager) GetNamespaceForCluster(_ int) string {
	return ""
}

// Close closes the Docker client connection
func (d *DockerSwarmManager) Close() error {
	if d.dockerClient != nil {
		return d.dockerClient.Close()
	}
	return nil
}

// findServicesByProcessorID finds services by processor ID using label filtering
func (d *DockerSwarmManager) findServicesByProcessorID(ctx context.Context, processorID string) ([]swarm.Service, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("%s=%s", labelProcessorID, processorID))

	services, err := d.dockerClient.ServiceList(ctx, types.ServiceListOptions{
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("services not found for processor %s", processorID)
	}

	return services, nil
}

func (d *DockerSwarmManager) GetLogs(ctx context.Context, processor *models.Processor, limit int32, after string) ([]Log, string, error) {
	services, err := d.findServicesByProcessorID(ctx, processor.ID)
	if err != nil {
		return nil, "", err
	}

	var allLogs []Log
	for _, service := range services {
		options := container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Timestamps: true,
			Tail:       strconv.Itoa(int(limit)),
		}
		if after != "" {
			options.Since = after
		}

		logsReader, err := d.dockerClient.ServiceLogs(ctx, service.ID, options)
		if err != nil {
			log.Warnw("Failed to get logs for service", "serviceID", service.ID, "error", err)
			continue
		}
		defer logsReader.Close()

		// Parse docker log stream
		// Docker logs have a header: [1 byte stream type, 3 bytes zeros, 4 bytes length]
		header := make([]byte, 8)
		for {
			_, err := logsReader.Read(header)
			if err != nil {
				break
			}
			dataLen := uint32(header[4])<<24 | uint32(header[5])<<16 | uint32(header[6])<<8 | uint32(header[7])
			data := make([]byte, dataLen)
			_, err = logsReader.Read(data)
			if err != nil {
				break
			}

			line := string(data)
			// Docker timestamps are at the beginning of each line if Timestamps: true
			// Format is like "2023-10-27T12:00:00.000000000Z message"
			parts := strings.SplitN(line, " ", 2)
			if len(parts) < 2 {
				continue
			}
			tsStr := parts[0]
			msg := parts[1]

			ts, err := time.Parse(time.RFC3339Nano, tsStr)
			if err != nil {
				continue
			}

			allLogs = append(allLogs, logLine{
				msg: strings.TrimSpace(msg),
				ts:  ts,
			})
		}
	}

	// Sort logs by timestamp
	sort.Slice(allLogs, func(i, j int) bool {
		return allLogs[i].Timestamp().Before(allLogs[j].Timestamp())
	})

	// Apply limit if necessary (we might have more because we fetched 'limit' from each service)
	if len(allLogs) > int(limit) {
		allLogs = allLogs[len(allLogs)-int(limit):]
	}

	nextAfter := ""
	if len(allLogs) > 0 {
		nextAfter = allLogs[len(allLogs)-1].Timestamp().Format(time.RFC3339Nano)
	}

	return allLogs, nextAfter, nil
}

type logLine struct {
	ts  time.Time
	msg string
}

func (l logLine) Message() string {
	return l.msg
}

func (l logLine) Timestamp() time.Time {
	return l.ts
}

func (d *DockerSwarmManager) makeDriverName(processor *models.Processor) string {
	driverName := fmt.Sprintf(
		"driver-%s", CleanupName(processor.ID),
	)

	// Docker service name max length is 64, and we append suffixes like "-processor" (10 chars)
	// So we need to ensure the base driver name is at most 54 characters
	const maxBaseNameLength = 54
	if len(driverName) > maxBaseNameLength {
		driverName = driverName[:maxBaseNameLength]
	}

	return driverName
}

func (d *DockerSwarmManager) makeProcessorServiceName(processor *models.Processor) string {
	return fmt.Sprintf("%s-processor", d.makeDriverName(processor))
}

func (d *DockerSwarmManager) makeDriverServiceName(processor *models.Processor) string {
	return fmt.Sprintf("%s-driver", d.makeDriverName(processor))
}
func CleanupName(name string) string {
	ret := name
	re, err := regexp.Compile(`[^0-9A-Za-z]`)
	if err != nil {
		log.Fatale(err)
	}
	ret = re.ReplaceAllString(ret, "")
	ret = strings.ToLower(ret)
	return ret
}
