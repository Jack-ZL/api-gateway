package consul

import (
	"fmt"
	"net"
	"strconv"

	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

// ServiceInstance 服务实例信息
type ServiceInstance struct {
	ID   string
	Host string
	Port int
	Meta map[string]string //  元数据，可以扩展
}

// ServiceDiscovery 服务发现接口
type ServiceDiscovery interface {
	GetServiceInstances(serviceName string) ([]*ServiceInstance, error)
	RegisterService(serviceName string, host string, port int, healthCheckURL string, meta map[string]string) error
	DeregisterService(serviceID string) error
}

// ConsulServiceDiscovery Consul 服务发现实现
type ConsulServiceDiscovery struct {
	client *api.Client
	logger *zap.Logger
}

// NewConsulServiceDiscovery 创建 ConsulServiceDiscovery 实例
func NewConsulServiceDiscovery(consulAddress string, logger *zap.Logger) (*ConsulServiceDiscovery, error) {
	config := api.DefaultConfig()
	config.Address = consulAddress // Consul 地址

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("创建 Consul 客户端失败: %w", err)
	}

	return &ConsulServiceDiscovery{
		client: client,
		logger: logger,
	}, nil
}

// GetServiceInstances 从 Consul 获取服务实例列表
func (sd *ConsulServiceDiscovery) GetServiceInstances(serviceName string) ([]*ServiceInstance, error) {
	services, _, err := sd.client.Health().Service(serviceName, "", true, nil) //  查询健康的服务实例
	if err != nil {
		return nil, fmt.Errorf("从 Consul 查询服务实例失败: %w", err)
	}

	instances := make([]*ServiceInstance, 0, len(services))
	for _, service := range services {
		instance := &ServiceInstance{
			ID:   service.Service.ID,
			Host: service.Service.Address, //  使用服务注册时提供的地址
			Port: service.Service.Port,
			Meta: service.Service.Meta,
		}
		//  如果 Address 为空，尝试从 Node 地址获取 (例如 Consul Agent 和 Service 运行在同一主机)
		if instance.Host == "" {
			instance.Host = service.Node.Address
		}
		instances = append(instances, instance)
	}
	return instances, nil
}

// RegisterService 将服务注册到 Consul
func (sd *ConsulServiceDiscovery) RegisterService(serviceName string, host string, port int, healthCheckURL string, meta map[string]string) error {
	serviceID := fmt.Sprintf("%s-%s-%d", serviceName, host, port) //  生成 Service ID，确保唯一性

	registration := &api.AgentServiceRegistration{
		ID:      serviceID,
		Name:    serviceName,
		Address: host, //  注册服务实例的 IP 地址或 Hostname，这里使用显式传入的 host
		Port:    port,
		Meta:    meta,
		Check: &api.AgentServiceCheck{
			HTTP:     healthCheckURL, // 健康检查 URL
			Interval: "10s",          // 健康检查间隔
			Timeout:  "5s",           // 健康检查超时
		},
	}

	err := sd.client.Agent().ServiceRegister(registration)
	if err != nil {
		return fmt.Errorf("向 Consul 注册服务失败: %w", err)
	}

	sd.logger.Info("服务注册成功", zap.String("service_id", serviceID), zap.String("service_name", serviceName), zap.String("address", net.JoinHostPort(host, strconv.Itoa(port))))
	return nil
}

// DeregisterService 从 Consul 注销服务
func (sd *ConsulServiceDiscovery) DeregisterService(serviceID string) error {
	err := sd.client.Agent().ServiceDeregister(serviceID)
	if err != nil {
		return fmt.Errorf("从 Consul 注销服务失败: %w", err)
	}
	sd.logger.Info("服务注销成功", zap.String("service_id", serviceID))
	return nil
}
