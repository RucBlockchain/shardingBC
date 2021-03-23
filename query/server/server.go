package server

import (
	"fmt"
	"go.etcd.io/etcd/embed"
	"log"
	"net/url"
)

type Server struct {
	cfg  *embed.Config
	Etcd *embed.Etcd
}
type Config struct {
	Name           string
	Dir            string
	InitialCluster string
}

func NewConfig(Name string, Dir string, Init string) *Config {
	Cfg := &Config{
		Name:           Name,
		Dir:            Dir,
		InitialCluster: Init,
	}
	return Cfg
}
func NewServer(c *Config) *Server {
	Svr := &Server{
		cfg: embed.NewConfig(),
	}
	Svr.Modify(c)
	return Svr
}
func (Svr *Server) Start() error {
	e, err := embed.StartEtcd(Svr.cfg)
	fmt.Println(Svr.cfg.InitialCluster)
	Svr.Etcd = e
	if err != nil {
		log.Fatal(err)
	}
	select {
	// Wait etcd until it is ready to use
	case <-Svr.Etcd.Server.ReadyNotify():
		log.Printf("Server is ready!")
	}
	return nil
}
func (Svr *Server) Stop() {
	Svr.Etcd.Close()
}
func (Svr *Server) Modify(c *Config) {

	Svr.cfg = embed.NewConfig()
	lpurl, _ := url.Parse("http://0.0.0.0:2380")
	apurl, _ := url.Parse("http://" + c.Name + ":2380")
	lcurl, _ := url.Parse("http://0.0.0.0:2379")
	acurl, _ := url.Parse("http://" + c.Name + ":2379")
	Svr.cfg.LPUrls = []url.URL{*lpurl}
	Svr.cfg.LCUrls = []url.URL{*lcurl}
	Svr.cfg.APUrls = []url.URL{*apurl}
	Svr.cfg.ACUrls = []url.URL{*acurl}
	Svr.cfg.Name = c.Name
	Svr.cfg.Dir = c.Dir + ".etcd"
	Svr.cfg.InitialCluster = c.InitialCluster
}
