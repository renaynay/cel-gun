package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/celestiaorg/celestia-node/share/shwap"
	"github.com/celestiaorg/celestia-node/share/shwap/p2p/shrex"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
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

	host   host.Host
	target peer.ID
}

type GunConfig struct {
	Target   multiaddr.Multiaddr
	Parallel int
}

func NewGun(conf GunConfig) *Gun {
	h, err := libp2p.New()
	if err != nil {
		panic(err)
	}

	return &Gun{
		conf: conf,
		host: h,
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

	err = g.host.Connect(ctx, *info)
	if err != nil {
		panic(err)
	}

	g.aggr = aggr
	g.GunDeps = deps
	return nil
}

func (g *Gun) Shoot(ammo core.Ammo) {
	customAmmo := ammo.(*Ammo)
	var wg sync.WaitGroup
	for i := 0; i < g.conf.Parallel; i++ {
		wg.Add(1)
		go g.shoot(context.Background(), customAmmo)
	}
	wg.Wait()
}

func (g *Gun) shoot(ctx context.Context, _ *Ammo) {
	ns, err := hex.DecodeString("0000000000000000000000000000000000000000000000004365726f41")
	if err != nil {
		panic(err)
	}
	roothash, err := hex.DecodeString("76A4524A20CC58991783163D4457BD3F3D11C11533AF7FD6A0722D3668C36F0D")
	if err != nil {
		panic(err)
	}

	ndReq := shwap.NamespaceDataID{
		Namespace: ns,
		RootHash:  roothash,
	}
	bin, err := ndReq.Marshal()
	if err != nil {
		panic(err)
	}

	protocolID := shrex.ProtocolString + shwap.NamespaceDataName

	stream, err := g.host.NewStream(ctx, g.target, protocol.ID(protocolID))
	if err != nil {
		panic(err)
	}

	for {
		_, err = stream.Write(bin)
		if err != nil {
			panic(err)
		}

		nd := new(shwap.NamespaceData)
		_, err = nd.ReadFrom(stream)
		if err != nil {
			panic(err)
		}

		fmt.Println("got namespace data response: ", len(nd.Flatten()))
	}
}

type Ammo struct {
	NDRequest *ndtypes.GetSharesByNamespaceRequest
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
			Target:   addr,
			Parallel: 16, // we choose this as the rate limiting middleware upper bound in shrex server
		}
	})
	register.Gun("my-custom/no-default", NewGun)

	cli.Run()
}
