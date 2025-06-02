package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"log"
	// "mime/multipart"
	"os"
	"os/signal"
	// "sync"
	"time"
	"strings"
	// "github.com/ipfs/boxo/bootstrap"
	"github.com/libp2p/go-libp2p"
	// "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

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
/*func setDHT(ctx context.Context, host host.Host) *dht.IpfsDHT {
	dhttt, err := dht.New(ctx, host)
	if err != nil { 
		panic(err)
	}
	if err = dhttt.Bootstrap(ctx); err != nil {
		panic(err)
	}
	go bootstrapConnect(ctx, host)
	return dhttt
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
				log.Printf("error",peerinfo)
			} else { 
				log.Printf("con", peerinfo)
			}
		}()
	}
	wg.Wait()
} */
func startChat(host host.Host) { 
	scanner := bufio.NewScanner(os.Stdin)
    var currentPeer peer.ID

    fmt.Println("\nAvailable commands:")
    fmt.Println("  /connect <multiaddr> - Connect to a peer")
    fmt.Println("  /exit               - Exit the program")
    fmt.Print("> ")

    for scanner.Scan() {
        input := strings.TrimSpace(scanner.Text())
        
        switch {
        case input == "/exit":
            fmt.Println("Exiting...")
            os.Exit(0)
            
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
func gracufulshutdow() { 
	ch := make(chan os.Signal,1)
	signal.Notify(ch, os.Interrupt)
	<- ch
	fmt.Println("кончаем....)")
}
func main() { 
	
	key, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil { 
		log.Fatal(err)
	}
	host, err := libp2p.New(libp2p.Identity(key),libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),)
	if err != nil { 
		log.Fatal(err)
	}
	defer host.Close()
 	host.SetStreamHandler("/roplax", handleStream)
 	fmt.Println("id", host.ID())
	fmt.Println("слушаем..._) MISTER R.O.B.O.T")
	for _, addr := range host.Addrs() { 
		fmt.Printf("%s/p2p/%s\n",addr,host.ID())
	}
	go startChat(host)
	gracufulshutdow()
}