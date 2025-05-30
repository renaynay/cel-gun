package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/libp2p/go-libp2p/core/network"
	"go.uber.org/zap"
	"strings"
	"sync"
	"time"

	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	shrexpb "github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/pb"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"github.com/spf13/afero"
	"github.com/yandex/pandora/cli"
	"github.com/yandex/pandora/core"
	coreimport "github.com/yandex/pandora/core/import"
	"github.com/yandex/pandora/core/register"
)

type Gun struct {
	GunDeps core.GunDeps
	aggr    core.Aggregator

	conf GunConfig

	hosts  []host.Host
	target peer.ID
}

type GunConfig struct {
	Target   multiaddr.Multiaddr
	Parallel int
}

func NewGun(conf GunConfig) *Gun {
	hosts := make([]host.Host, conf.Parallel)
	for i := 0; i < conf.Parallel; i++ {
		h, err := libp2p.New()
		if err != nil {
			panic(err)
		}
		hosts[i] = h
	}

	return &Gun{
		conf:  conf,
		hosts: hosts,
	}
}

func (g *Gun) Bind(aggr core.Aggregator, deps core.GunDeps) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	info, err := peer.AddrInfoFromP2pAddr(g.conf.Target)
	if err != nil {
		panic(err)
	}
	g.target = info.ID

	for _, h := range g.hosts {
		err = h.Connect(ctx, *info)
		if err != nil {
			panic(err)
		}
	}

	g.aggr = aggr
	g.GunDeps = deps
	return nil
}

func (g *Gun) Shoot(ammo core.Ammo) {
	customAmmo := ammo.(*Ammo)
	var wg sync.WaitGroup
	for i := 0; i < g.conf.Parallel; i++ {
		i := i
		go func() {
			wg.Add(1)
			g.shoot(context.Background(), customAmmo, g.hosts[i])
			wg.Done()
		}()
	}
	wg.Wait()
}

func (g *Gun) shoot(ctx context.Context, _ *Ammo, h host.Host) {
	ns, err := hex.DecodeString("0000000000000000000000000000000000000000726973652d646576")
	if err != nil {
		panic(err)
	}
	namespace, err := libshare.NewNamespace(0, ns)
	if err != nil {
		panic(err)
	}

	ndReq := shwap.NamespaceDataID{
		EdsID: shwap.EdsID{
			Height: 895225,
		},
		DataNamespace: namespace,
	}

	bin, err := ndReq.MarshalBinary()
	if err != nil {
		panic(err)
	}

	protocolID := "/mamo-1" + shrex.ProtocolString + shwap.NamespaceDataName

	fmt.Println("SHOOTING FROM HOST: ", h.ID().String())

	var stream network.Stream
	for i := 0; i < 5; i++ {
		startTime := time.Now()

		if stream == nil {
			stream, err = h.NewStream(ctx, g.target, protocol.ID(protocolID))
			if err != nil {
				panic(err)
			}
		}

		err = stream.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err != nil {
			fmt.Println("set read deadline err: ", err.Error())
		}

		_, err = stream.Write(bin)
		if err != nil {
			if strings.Contains(err.Error(), "stream reset") || strings.Contains((err.Error()), "stream closed") {
				stream = nil
			}
			continue
		}
		stream.CloseWrite()

		err = stream.SetReadDeadline(time.Now().Add(time.Minute))
		if err != nil {
			fmt.Println("set read deadline err: ", err.Error())
		}

		var resp shrexpb.Response
		_, err := serde.Read(stream, &resp)
		if err != nil {
			if strings.Contains(err.Error(), "stream reset") || strings.Contains((err.Error()), "stream closed") {
				stream = nil
			}
			fmt.Println("ERR reading shrexpb resp status from stream : ", err.Error())
			continue
		}
		fmt.Println(" STATUS RESP: ", resp.Status.String())

		nd := new(shwap.NamespaceData)
		_, err = nd.ReadFrom(stream)
		if err != nil {
			if strings.Contains(err.Error(), "stream reset") || strings.Contains((err.Error()), "stream closed") {
				stream = nil
			}
			fmt.Println("ERR reading nd from stream: ", err.Error())
			continue
		}
		stream.CloseRead()

		endTime := time.Since(startTime)
		latencySeconds := float64(endTime.Seconds())
		responseBytes := int64(len(nd.Flatten()))

		fmt.Println("got namespace data response: ", responseBytes, "      in ", latencySeconds, " seconds")
	}
}

type Ammo struct {
}

func main() {
	// Standard imports
	fs := afero.NewOsFs()
	coreimport.Import(fs)

	// Custom imports. Integrate your custom types into configuration system.
	coreimport.RegisterCustomJSONProvider("custom_provider", func() core.Ammo { return &Ammo{} })

	register.Gun("nd_gun", NewGun, func() GunConfig {
		addr, err := multiaddr.NewMultiaddr("/ip4/51.159.144.71/tcp/2121/p2p/12D3KooWLVXFhiPZdsgVazpmpfKBjAzUrnkLtUY6p6oHKcGjVuhp")
		if err != nil {
			panic(err)
		}

		return GunConfig{
			Target: addr,
		}
	})

	cli.Run()

	// suppress logs from pandora as they are not useful to me right now
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zap.ErrorLevel) // discard all INFO  logs from pandora
	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	zap.ReplaceGlobals(logger)
}
