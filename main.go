package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	shrexpb "github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex/pb"
	"github.com/celestiaorg/go-libp2p-messenger/serde"
	libshare "github.com/celestiaorg/go-square/v2/share"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"strings"
	"sync"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/spf13/afero"
	"github.com/yandex/pandora/cli"
	phttp "github.com/yandex/pandora/components/phttp/import"
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
			fmt.Println("\n\n LAUNCHING::   ", i)
			wg.Add(1)
			g.shoot(context.Background(), customAmmo, g.hosts[i])
			wg.Done()
		}()
	}
	wg.Wait()
}

func (g *Gun) shoot(ctx context.Context, _ *Ammo, h host.Host) {
	ns, err := hex.DecodeString("00000000000000000000000000000000000000005649410000000000")
	if err != nil {
		panic(err)
	}
	namespace, err := libshare.NewNamespace(0, ns)
	if err != nil {
		panic(err)
	}

	ndReq := shwap.NamespaceDataID{
		EdsID: shwap.EdsID{
			6072861,
		},
		DataNamespace: namespace,
	}
	bin, err := ndReq.MarshalBinary()
	if err != nil {
		panic(err)
	}

	protocolID := "/arabica-11" + shrex.ProtocolString + shwap.NamespaceDataName

	fmt.Println("SHOOTING FROM HOST: ", h.ID().String())

	stream, err := h.NewStream(ctx, g.target, protocol.ID(protocolID))
	if err != nil {
		panic(err)
	}

	for {
		_, err = stream.Write(bin)
		if err != nil {
			if strings.Contains(err.Error(), "stream reset") {
				continue
			}
			panic(err)
		}

		var resp shrexpb.Response
		_, err := serde.Read(stream, &resp)
		if err != nil {
			fmt.Println("ERROR: ", err.Error())
			continue
		}
		fmt.Println(" STATUS RESP: ", resp.Status.String())

		nd := new(shwap.NamespaceData)
		_, err = nd.ReadFrom(stream)
		if err != nil {
			panic(err)
		}

		fmt.Println("got namespace data response: ", len(nd.Flatten()))
	}
}

type Ammo struct {
}

//var targetAddr = os.Getenv("CEL_GUN_TARGET_ADDR")

func main() {
	// debug.SetGCPercent(-1)
	// Standard imports.
	fs := afero.NewOsFs()
	coreimport.Import(fs)
	// May not be imported, if you don't need http guns and etc.
	phttp.Import(fs)

	// Custom imports. Integrate your custom types into configuration system.
	coreimport.RegisterCustomJSONProvider("custom_provider", func() core.Ammo { return &Ammo{} })

	register.Gun("nd_gun", NewGun, func() GunConfig {
		addr, err := multiaddr.NewMultiaddr("/dnsaddr/da-bootstrapper-1.celestia-arabica-11.com/p2p/12D3KooWGqwzdEqM54Dce6LXzfFr97Bnhvm6rN7KM7MFwdomfm4S")
		if err != nil {
			panic(err)
		}

		return GunConfig{
			Target: addr,
		}
	})

	cli.Run()
}
