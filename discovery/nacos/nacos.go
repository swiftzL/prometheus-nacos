package nacos

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/common/logger"
	model2 "github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"go.uber.org/zap"
)

func init() {
	discovery.RegisterConfig(&SDConfig{})
}

type SDConfig struct {
	Server          string   `yaml:"server,omitempty"`
	NameSpaces      []string `yaml:"namespaces,omitempty"`
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
	clients []naming_client.INamingClient
	mertric *NacosMertrics
}

func (n *NacosDiscoverer) Run(ctx context.Context, up chan<- []*targetgroup.Group) {
	zapLoggerConfig := zap.NewProductionConfig()
	zapLogger, _ := zapLoggerConfig.Build(zap.AddCaller(), zap.AddCallerSkip(1))
	logger.SetLogger(&logger.NacosLogger{zapLogger.Sugar()})
	logger.Infof("start run the nacos discover")

	select {
	case <-ctx.Done():
		return
	default:
	}
	for {
		ticker := time.NewTicker(time.Second * time.Duration(n.config.RefreshInterval))
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		default:

			n.mertric.rpcGetCount.Inc()
			tg, err := n.LookUpServices()
			if err != nil {
				n.logger.Log("look up is error ", err)
			}
			if tg != nil {
				logger.Info("look up size is ", len(tg.Targets))
				up <- []*targetgroup.Group{tg}
			}

			<-ticker.C
		}

	}
}

func (n *NacosDiscoverer) LookUpServices() (*targetgroup.Group, error) {
	if n.config.Debug {
		n.logger.Log("start lookup services", n.config.Services)
	}
	targets := make([]model.LabelSet, 0, 10)
	for _, service := range n.config.Services {
		if n.config.Debug {
			logger.Infof("start lookup services", service)
		}
		for _, client := range n.clients {
			err := n.LookUpService(service, client, &targets)
			if err != nil {
				logger.Infof("add service error ", err)
				return nil, err
			}
		}
	}
	return &targetgroup.Group{Source: "nacos", Targets: targets}, nil

}

func (n *NacosDiscoverer) LookUpService(serviceName string, client naming_client.INamingClient, targets *[]model.LabelSet) error {
	serviceRes, err := client.GetService(vo.GetServiceParam{ServiceName: serviceName})
	if err != nil {
		if n.config.Debug {
			n.logger.Log("lookup server is error")
		}
		return err
	}

	for _, host := range serviceRes.Hosts {
		port_str, ok := host.Metadata["metric_port"]
		if !ok {
			continue
		}
		port, err := strconv.Atoi(port_str)
		if err != nil || port <= 0 {
			continue
		}
		labels := model.LabelSet{
			model.AddressLabel: model.LabelValue(host.Ip + ":" + port_str),
			"instanceId":       model.LabelValue(host.InstanceId),
			"serviceName":      model.LabelValue(serviceName),
		}
		if n.config.Debug {
			logger.Infof("add service ", serviceName, host.Ip)
		}
		*targets = append(*targets, labels)
	}
	return err
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
	for _, nameSpaceId := range S.NameSpaces {
		namingClient, err := clients.NewNamingClient(
			vo.NacosClientParam{
				ClientConfig: &constant.ClientConfig{
					NamespaceId:          nameSpaceId,
					NotLoadCacheAtStart:  true,
					UpdateCacheWhenEmpty: true,
				},
				ServerConfigs: serverConfigs,
			},
		)
		if err != nil {
			return nil, err
		}
		dis.clients = append(dis.clients, namingClient)
	}

	dis.mertric = nacos_metrics
	for _, serviceName := range dis.config.Services {
		for _, client := range dis.clients {
			err = client.Subscribe(&vo.SubscribeParam{
				ServiceName: serviceName,
				SubscribeCallback: func(services []model2.Instance, err error) {
					dis.logger.Log("serveice subscribe size:{}", serviceName, len(services), err)
				},
			})
			if err != nil {
				panic(err)
			}
		}
	}
	return dis, nil
}

func (S SDConfig) NewDiscovererMetrics(registerer prometheus.Registerer, instantiator discovery.RefreshMetricsInstantiator) discovery.DiscovererMetrics {
	return NewNacosMertrics(registerer, instantiator)
}
