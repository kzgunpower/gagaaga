package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"
	"sync"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
)
var chatPeers sync.Map
func connectToPeer(host host.Host, targetAddr string) (peer.ID, error) { 
	addr, err := multiaddr.NewMultiaddr(targetAddr)
	if err != nil { 
		log.Printf("Invalid ti %s",err)
	}
	ip, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil { 
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
	defer cancel()
	if err := host.Connect(ctx, *ip); err != nil { 
		log.Printf("eCOnnet failed %v",err)
	}
	return ip.ID, nil
}
func handleStream(s network.Stream) {
	defer s.Close()
	remotePeer := s.Conn().RemotePeer()
	buf := make([]byte,1024)
	n, err := s.Read(buf)
	if err != nil { 
		log.Printf("error %v", err)
	}
	msg := string(buf[:n])
	log.Printf("\n[%s] %s\n>", remotePeer.ShortString(), msg)
}
func sendmsg(host host.Host, peerID peer.ID, msg string) error { 
	s, err := host.NewStream(context.Background(),peerID,"/roplax")
	if err != nil { 
		return err
	}
	defer s.Close()
	_, err = s.Write([]byte(msg))
	return err
}
func setDHT(ctx context.Context, host host.Host) (*dht.IpfsDHT, error) {
	dhttt, err := dht.New(ctx, host)
	if err != nil {
        return nil, fmt.Errorf("failed to create DHT: %w", err)
    }
	if err = dhttt.Bootstrap(ctx); err != nil {
		return nil, fmt.Errorf("failed bpptstrap")
	}
	go bootstrapConnect(ctx, host)
	return dhttt, nil
}
func bootstrapConnect(ctx context.Context, host host.Host) { 
	bootstrapPeers := dht.DefaultBootstrapPeers
	var wg sync.WaitGroup 
	for _, peerAddr := range bootstrapPeers { 
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() { 
			defer wg.Done()
			if err := host.Connect(ctx, *peerinfo); err != nil { 
				log.Printf("error %v",peerinfo)
			} else { 
				log.Printf("con %v", peerinfo)
			}
		}()
	}
	wg.Wait()
} 
func startChat(host host.Host) { 
	scanner := bufio.NewScanner(os.Stdin)
    var currentPeer peer.ID

    fmt.Println("\nAvailable commands:")
    fmt.Println("  /connect <multiaddr> - Connect to ^_^")
	fmt.Println("  /peer                - list of ^_^")
    fmt.Println("  /exit                - sigma rule 1 ")
	fmt.Println("  /select <peerID>     - Select chat peer by ID (short or full)")
    fmt.Print("> ")

    for scanner.Scan() {
        input := strings.TrimSpace(scanner.Text())
        
        switch {
        case input == "/exit":
            fmt.Println("Exiting...")
            os.Exit(0)
        
		case input == "/peer":
			fmt.Println("Discovered chat peers:")
			chatPeers.Range(func(key, value interface{}) bool {
				peerID := key.(peer.ID)
				if peerID == host.ID() {
					return true
				}
				status := host.Network().Connectedness(peerID)
				switch status {
				case network.Connected:
					fmt.Printf("- %s (connected)\n", peerID.ShortString())
				default:
					fmt.Printf("- %s (not connected)\n", peerID.ShortString())
				}
				return true
			})
		case strings.HasPrefix(input, "/select "):
			peerShortID := strings.TrimSpace(input[8:])
			if peerShortID == "" {
				fmt.Println("Error: Missing peer ID after /select")
				fmt.Print("> ")
				continue
			}

			var foundPeerID peer.ID
			var found bool
			chatPeers.Range(func(key, value interface{}) bool {
				peerID := key.(peer.ID)
				if peerID.ShortString() == peerShortID || peerID.String() == peerShortID {
					foundPeerID = peerID
					found = true
					return false
				}
				return true
			})

			if !found {
				fmt.Printf("Peer %s not found in chat peers\n", peerShortID)
			} else {
				currentPeer = foundPeerID
				fmt.Printf("Selected peer %s\n> ", foundPeerID.ShortString())
			}

			fmt.Print("> ")
        case strings.HasPrefix(input, "/connect "):
            targetAddr := strings.TrimSpace(input[9:])
            if targetAddr == "" {
                fmt.Println("Error: Missing address after /connect")
                fmt.Print("> ")
                continue
            }
            
            peerID, err := connectToPeer(host, targetAddr)
            if err != nil {
                fmt.Printf("Connection error: %v\n", err)
            } else {
                currentPeer = peerID
                fmt.Printf("Connected to %s\n> ", peerID.ShortString())
            }
            
        case currentPeer != "":
            if input != "" {
                if err := sendmsg(host, currentPeer, input); err != nil {
                    fmt.Printf("Send error: %v\n", err)
                }
            }
            fmt.Print("> ")
            
        default:
            if input != "" {
                fmt.Println("Not connected to any peer. Use /connect first")
            }
            fmt.Print("> ")
        }
    }
}
func discoverPeers(ctx context.Context, host host.Host, dht *dht.IpfsDHT) { 
	rD := routing.NewRoutingDiscovery(dht)
	util.Advertise(ctx, rD, "/roplax-chat")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select { 
		case <-ctx.Done():
			return
		case <-ticker.C:
			peers, err := rD.FindPeers(ctx, "/roplax-chat")
			if err != nil { 
				log.Fatal("ftoto notak %v",err)
			}
			for peer := range peers { 
				if peer.ID == host.ID() || len(peer.Addrs) == 0 { 
					continue
				}
				host.Peerstore().AddAddrs(peer.ID, peer.Addrs, peerstore.PermanentAddrTTL)
				if host.Network().Connectedness(peer.ID) != network.Connected { 
					ctx , cancel := context.WithTimeout(ctx, 5 *time.Second)
					defer cancel()
					if err := host.Connect(ctx, peer); err != nil { 
						log.Print("ne smogli podcl")
					} else { 
						chatPeers.Store(peer.ID, struct{}{})
						log.Printf("Connected to chat peer %s", peer.ID.ShortString())
					} 
				}
			}
		}
	}
}
func gracufulshutdow() { 
	ch := make(chan os.Signal,1)
	signal.Notify(ch, os.Interrupt)
	<- ch
	fmt.Println("кончаем....)")
}
func main() { 
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	key, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil { 
		log.Fatal(err)
	}
	host, err := libp2p.New(libp2p.Identity(key),libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),)
	if err != nil { 
		log.Fatal(err)
	}
	defer host.Close()
	kadDHT, err := setDHT(ctx, host)
	if err != nil { 
		log.Fatal("DHT trollll")
	}
	defer kadDHT.Close()
	go discoverPeers(ctx, host, kadDHT)
 	host.SetStreamHandler("/roplax", handleStream)
 	fmt.Println("id", host.ID())
	fmt.Println("слушаем..._) MISTER R.O.B.O.T")
	for _, addr := range host.Addrs() { 
		fmt.Printf("%s/p2p/%s\n",addr,host.ID())
	}
	go startChat(host)
	gracufulshutdow()
}