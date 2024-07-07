package nacos

import (
	"context"
	"errors"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"strconv"
	"strings"
	"time"
)

func init() {
	discovery.RegisterConfig(&SDConfig{})
}

type SDConfig struct {
	Server          string   `yaml:"server,omitempty"`
	NameSpace       string   `yaml:"nameSpace,omitempty"`
	Services        []string `yaml:"services"`
	RefreshInterval int64    `yaml:"refresh_interval"`
	Debug           bool     `yaml:"debug"`
}

func (c *SDConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	*c = SDConfig{RefreshInterval: 15, Debug: false}
	type plain SDConfig
	err := unmarshal((*plain)(c))
	if err != nil {
		return err
	}
	return nil
}

type NacosDiscoverer struct {
	config  SDConfig
	logger  log.Logger
	client  naming_client.INamingClient
	mertric *NacosMertrics
}

func (n *NacosDiscoverer) Run(ctx context.Context, up chan<- []*targetgroup.Group) {
	n.logger.Log("start run the nacos discover")
	select {
	case <-ctx.Done():
		return
	default:
	}
	level.Info(n.logger).Log("start run nacos discover")
	for {
		ticker := time.NewTicker(time.Second * time.Duration(n.config.RefreshInterval))
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			tg, err := n.LookUpServices()
			if err != nil {
				n.logger.Log("look up is error ", err)
			}
			up <- tg
		}

	}
}

func (n *NacosDiscoverer) LookUpServices() ([]*targetgroup.Group, error) {
	if n.config.Debug {
		n.logger.Log("start lookup services")
	}
	res := make([]*targetgroup.Group, 0, len(n.config.Services))
	for _, service := range n.config.Services {
		service_group, err := n.LookUpService(service)
		if err != nil {
			return nil, err
		}
		res = append(res, service_group)
	}
	return res, nil

}

func (n *NacosDiscoverer) LookUpService(serviceName string) (*targetgroup.Group, error) {
	serviceRes, err := n.client.GetService(vo.GetServiceParam{ServiceName: serviceName})
	if err != nil {
		if n.config.Debug {
			n.logger.Log("lookup server is error")
		}
		return nil, err
	}
	res := &targetgroup.Group{Source: serviceName}
	for _, host := range serviceRes.Hosts {
		labels := model.LabelSet{
			model.AddressLabel: model.LabelValue(host.Ip + ":" + strconv.Itoa(int(host.Port))),
			"instanceId":       model.LabelValue(host.InstanceId),
		}
		res.Targets = append(res.Targets, labels)
	}
	return res, nil
}

func (S SDConfig) Name() string {
	return "nacos"
}

func (S SDConfig) NewDiscoverer(options discovery.DiscovererOptions) (discovery.Discoverer, error) {
	nacos_metrics, ok := options.Metrics.(*NacosMertrics)
	if !ok {
		return nil, errors.New("nacos mertris is not found")
	}
	dis := &NacosDiscoverer{config: S}
	if options.Logger == nil {
		dis.logger = log.NewNopLogger()
	} else {
		dis.logger = options.Logger
	}
	hostAndPort := strings.Split(S.Server, ":")
	nacosPort, err := strconv.Atoi(hostAndPort[1])
	if err != nil {
		return nil, err
	}
	serverConfigs := []constant.ServerConfig{
		*constant.NewServerConfig(
			hostAndPort[0],
			uint64(nacosPort),
			constant.WithScheme("http"),
			constant.WithContextPath("/nacos"),
		),
	}
	namingClient, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  nil,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		return nil, err
	}
	dis.client = namingClient
	dis.mertric = nacos_metrics
	return dis, nil
}

func (S SDConfig) NewDiscovererMetrics(registerer prometheus.Registerer, instantiator discovery.RefreshMetricsInstantiator) discovery.DiscovererMetrics {
	return NewNacosMertrics(registerer, instantiator)
}
