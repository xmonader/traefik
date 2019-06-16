package kv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/containous/traefik/pkg/config"
	"github.com/containous/traefik/pkg/log"
	"github.com/containous/traefik/pkg/provider"
	"github.com/containous/traefik/pkg/safe"
	"github.com/containous/traefik/pkg/tls"
	"io/ioutil"
	"strings"
	"time"

	"github.com/abronan/valkeyrie"
	"github.com/abronan/valkeyrie/store"
	"github.com/abronan/valkeyrie/store/redis"
)

const providerName = "kv"
var validBackends = map[string]store.Backend{
	"redis": store.REDIS,
	"boltdb": store.BOLTDB,
	"etcd": store.ETCDV3,

}

type MyTCPService struct {
	Name string `json:"name"`
	Addr string `json:"addr"`
	SNI  string `json:"sni"`
}
type MyTCPRouter struct {

}

type MyBackend struct {
	Addr string
	Port int
}

var _ provider.Provider = (*Provider)(nil)

// Provider holds configurations of the provider.
type Provider struct {
	DbBackend	string
	Username    string
	Password	string
	Addr		string
	Port 		uint
	PullTime    uint
	kvStore     store.Store
}

func init(){

	fmt.Println(" kv provider pkg..")

}
// Init the provider
func (p *Provider) Init() error {
	redis.Register()

	fmt.Println(p)
	redis.Register()

	//kvStore, _ := validBackends[p.DbBackend] // at this point backend exists or the app would have exited.
	// Initialize a new store with dbbackendtype
	kv, err := valkeyrie.NewStore(
		store.REDIS,
		//[]string{fmt.Sprintf("%s:%d" , p.Addr, p.Port)},
		[]string{"127.0.0.1:6379"},
		&store.Config{
			ConnectionTimeout: 10 * time.Second,
		},
	)
	if err != nil {
		fmt.Println("ERR INIT AGAIN: ", err)
		return err
	}
	p.kvStore = kv

	fmt.Println("Init kv provider..")

	return nil
}

// Provide allows the file provider to provide configurations to traefik
// using the given configuration channel.
func (p *Provider) Provide(configurationChan chan<- config.Message, pool *safe.Pool) error {
	configuration, err := p.BuildConfiguration()
	fmt.Println("in kv provide config")
	if err != nil {
		return err
	}
	for {

		backendPairs, err := p.kvStore.List("tcprouter/service/", nil)
		// fmt.fmt.Println("backendPairs", backendPairs, " err: ", err)
		fmt.Println(err)
		fmt.Println(len(backendPairs))
		for _, backendPair := range backendPairs {
			serviceName := backendPair.Key
			tcpService := &MyTCPService{}
			err = json.Unmarshal(backendPair.Value, tcpService)
			if err != nil {
				fmt.Println("invalid service content.")
			}
			fmt.Println(tcpService)
			//backendName := tcpService.Name
			backendSNI := tcpService.SNI

			backendAddr := tcpService.Addr
			parts := strings.Split(string(backendAddr), ":")
			addr, portStr := parts[0], parts[1]
			//port, err := strconv.Atoi(portStr)
			//if err != nil {
			//	continue
			//}
			svc := &config.TCPService{}
			svc.LoadBalancer.Servers = []config.TCPServer{{
				Address: addr,
				Port:    portStr,
				Weight:  0,
			}}
			router := &config.TCPRouter{EntryPoints:[]string{"tcp"}}
			router.Rule = fmt.Sprintf("HostSNI(%s)", backendSNI)
			router.Service = serviceName

			configuration.TCP.Routers[serviceName] = router
			configuration.TCP.Services[serviceName] = svc

		}
		fmt.Println(configuration.TCP)
		sendConfigToChannel(configurationChan, configuration)
		time.Sleep(time.Duration(p.PullTime)*time.Second)

	}

	// pull every 10 seconds.

	return nil
}

// BuildConfiguration loads configuration either from file or a directory specified by 'Filename'/'Directory'
// and returns a 'Configuration' object
func (p *Provider) BuildConfiguration() (*config.Configuration, error) {
	ctx := log.With(context.Background(), log.Str(log.ProviderName, providerName))
	fmt.Println(ctx)

	return nil, errors.New("error using kv configuration backend, no filename defined")
}

func sendConfigToChannel(configurationChan chan<- config.Message, configuration *config.Configuration) {
	configurationChan <- config.Message{
		ProviderName:  "kv",
		Configuration: configuration,
	}
}

func readFile(filename string) (string, error) {
	if len(filename) > 0 {
		buf, err := ioutil.ReadFile(filename)
		if err != nil {
			return "", err
		}
		return string(buf), nil
	}
	return "", fmt.Errorf("invalid filename: %s", filename)
}

func (p *Provider) loadFileConfig(filename string, parseTemplate bool) (*config.Configuration, error) {
	fileContent, err := readFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading configuration file: %s - %s", filename, err)
	}
	fmt.Println("fileContent: ", fileContent)
	var configuration *config.Configuration
	configuration, err = p.NewConfigurationObject(fileContent)
	if err != nil {
		return nil, err
	}

	var tlsConfigs []*tls.Configuration
	for _, conf := range configuration.TLS {
		bytes, err := conf.Certificate.CertFile.Read()
		if err != nil {
			log.Error(err)
			continue
		}
		conf.Certificate.CertFile = tls.FileOrContent(string(bytes))

		bytes, err = conf.Certificate.KeyFile.Read()
		if err != nil {
			log.Error(err)
			continue
		}
		conf.Certificate.KeyFile = tls.FileOrContent(string(bytes))
		tlsConfigs = append(tlsConfigs, conf)
	}
	configuration.TLS = tlsConfigs

	return configuration, nil
}


func (p *Provider) NewConfigurationObject(content string) (*config.Configuration, error) {
	configuration := &config.Configuration{
		HTTP: &config.HTTPConfiguration{
			Routers:     make(map[string]*config.Router),
			Middlewares: make(map[string]*config.Middleware),
			Services:    make(map[string]*config.Service),
		},
		TCP: &config.TCPConfiguration{
			Routers:  make(map[string]*config.TCPRouter),
			Services: make(map[string]*config.TCPService),
		},
		TLS:        make([]*tls.Configuration, 0),
		TLSStores:  make(map[string]tls.Store),
		TLSOptions: make(map[string]tls.TLS),
	}
	return configuration, nil
}
