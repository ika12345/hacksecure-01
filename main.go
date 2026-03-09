package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/ayush00git/lowkey/proto/v1"
)

var (
	targetUUID string
	sdpData    string
)

var rootCmd = &cobra.Command{
	Use:   "lowkey",
	Short: "Lowkey is a CLI for testing the signaling server",
	Long:  `A fast and flexible signaling CLI for WebRTC handshake simulation.`,
}

var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "Register and listen for incoming signaling messages",
	Run: func(cmd *cobra.Command, args []string) {
		id := uuid.New().String()
		fmt.Printf("My UUID: %s\n", id)

		conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("did not connect: %v", err)
		}
		defer conn.Close()

		client := pb.NewSignalingClient(conn)
		stream, err := client.Connect(context.Background())
		if err != nil {
			log.Fatalf("could not connect: %v", err)
		}

		// 1. Register
		err = stream.Send(&pb.SignalRequest{
			Payload: &pb.SignalRequest_Registration{
				Registration: &pb.Identity{
					Uuid: id,
				},
			},
		})
		if err != nil {
			log.Fatalf("registration failed: %v", err)
		}
		fmt.Println("Registered successfully. Waiting for signals...")

		// 2. Listen loop
		for {
			resp, err := stream.Recv()
			if err != nil {
				log.Fatalf("stream recv error: %v", err)
			}

			switch p := resp.Payload.(type) {
			case *pb.SignalResponse_Sdp:
				fmt.Printf("\n[SDP Received] Type: %s\nSDP: %s\nFrom Target: %s\n",
					p.Sdp.Type, p.Sdp.Sdp, p.Sdp.TargetUuid)
			case *pb.SignalResponse_Ice:
				fmt.Printf("\n[ICE Received] Candidate: %s\n", p.Ice.Candidate)
			case *pb.SignalResponse_Error:
				log.Printf("Server Error: %s", p.Error.Message)
			}
		}
	},
}

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a mock SDP Offer to a target peer",
	Run: func(cmd *cobra.Command, args []string) {
		if targetUUID == "" {
			log.Fatal("target UUID is required")
		}

		conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("did not connect: %v", err)
		}
		defer conn.Close()

		client := pb.NewSignalingClient(conn)
		stream, err := client.Connect(context.Background())
		if err != nil {
			log.Fatalf("could not connect: %v", err)
		}

		// 1. Identifying itself (sender)
		senderID := "cli-sender-" + uuid.New().String()[:8]
		err = stream.Send(&pb.SignalRequest{
			Payload: &pb.SignalRequest_Registration{
				Registration: &pb.Identity{
					Uuid: senderID,
				},
			},
		})
		if err != nil {
			log.Fatalf("registration failed: %v", err)
		}

		// 2. Send SDP Offer
		fmt.Printf("Sending SDP Offer to %s...\n", targetUUID)
		err = stream.Send(&pb.SignalRequest{
			Payload: &pb.SignalRequest_Sdp{
				Sdp: &pb.SdpExchange{
					Type:       pb.SdpExchange_TYPE_OFFER,
					Sdp:        sdpData,
					TargetUuid: targetUUID,
				},
			},
		})
		if err != nil {
			log.Fatalf("failed to send SDP: %v", err)
		}

		fmt.Println("SDP Offer sent successfully!")
		// Give the server a moment to receive the message before we close the stream.
		time.Sleep(500 * time.Millisecond)
	},
}

// chatTarget holds the peer UUID for the chat command.
var chatTarget string

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive text chat session with another peer",
	Long: `Enables real-time text messaging between two CLI peers through the signaling server.
No Android Studio or mobile device is required.

Usage:
  Peer A (listener):  ./lowkey chat
  Peer B (initiator): ./lowkey chat --target <UUID printed by Peer A>`,
	Run: func(cmd *cobra.Command, args []string) {
		myID := uuid.New().String()
		fmt.Printf("My UUID: %s\n", myID)

		conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("did not connect: %v", err)
		}
		defer conn.Close()

		client := pb.NewSignalingClient(conn)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		stream, err := client.Connect(ctx)
		if err != nil {
			log.Fatalf("could not connect: %v", err)
		}

		// sendMu guards all stream.Send calls; gRPC streams are not thread-safe.
		var sendMu sync.Mutex

		// safeSend serialises concurrent Send calls on the bidirectional stream.
		safeSend := func(req *pb.SignalRequest) error {
			sendMu.Lock()
			defer sendMu.Unlock()
			return stream.Send(req)
		}

		// Register with the signaling server.
		if err := safeSend(&pb.SignalRequest{
			Payload: &pb.SignalRequest_Registration{
				Registration: &pb.Identity{Uuid: myID},
			},
		}); err != nil {
			log.Fatalf("registration failed: %v", err)
		}

		// startStdinReader reads lines from stdin and forwards them as messages
		// to targetID. It stops when ctx is cancelled.
		startStdinReader := func(targetID string) {
			go func() {
				scanner := bufio.NewScanner(os.Stdin)
				fmt.Print("> ")
				for {
					select {
					case <-ctx.Done():
						return
					default:
					}
					if !scanner.Scan() {
						return
					}
					text := strings.TrimSpace(scanner.Text())
					if text == "" {
						fmt.Print("> ")
						continue
					}
					if err := safeSend(&pb.SignalRequest{
						Payload: &pb.SignalRequest_Sdp{
							Sdp: &pb.SdpExchange{
								Type:       pb.SdpExchange_TYPE_UNSPECIFIED,
								Sdp:        "MSG:" + text,
								TargetUuid: targetID,
							},
						},
					}); err != nil {
						log.Printf("failed to send message: %v", err)
					}
					fmt.Print("> ")
				}
			}()
		}

		if chatTarget != "" {
			// Initiator: announce presence to the listener.
			if err := safeSend(&pb.SignalRequest{
				Payload: &pb.SignalRequest_Sdp{
					Sdp: &pb.SdpExchange{
						Type:       pb.SdpExchange_TYPE_UNSPECIFIED,
						Sdp:        "HELLO:" + myID,
						TargetUuid: chatTarget,
					},
				},
			}); err != nil {
				log.Fatalf("failed to send hello: %v", err)
			}
			fmt.Printf("Connected to peer %s\nType messages and press Enter to send. Press Ctrl+C to exit.\n", chatTarget)
			startStdinReader(chatTarget)
		} else {
			fmt.Println("Waiting for a peer to connect... (share your UUID above)")
		}

		// Main receive loop.
		for {
			resp, err := stream.Recv()
			if err != nil {
				cancel()
				log.Fatalf("stream recv error: %v", err)
			}

			switch p := resp.Payload.(type) {
			case *pb.SignalResponse_Sdp:
				content := p.Sdp.Sdp
				switch {
				case strings.HasPrefix(content, "HELLO:"):
					// Peer has connected; note their ID and start accepting stdin input.
					peerID := strings.TrimPrefix(content, "HELLO:")
					fmt.Printf("\nPeer connected: %s\nType messages and press Enter to send. Press Ctrl+C to exit.\n", peerID)
					startStdinReader(peerID)
				case strings.HasPrefix(content, "MSG:"):
					msg := strings.TrimPrefix(content, "MSG:")
					fmt.Printf("\r[peer] %s\n> ", msg)
				}
			case *pb.SignalResponse_Error:
				log.Printf("Server Error: %s", p.Error.Message)
			}
		}
	},
}

func init() {
	sendCmd.Flags().StringVarP(&targetUUID, "target", "t", "", "Target UUID to send signal to")
	sendCmd.Flags().StringVar(&sdpData, "sdp", "v=0\no=- 12345 12345 IN IP4 127.0.0.1\ns=-\nt=0 0\na=fingerprint:sha-256 ...", "Dummy SDP data")
	chatCmd.Flags().StringVarP(&chatTarget, "target", "t", "", "UUID of the peer to connect to (omit to listen for a peer)")

	rootCmd.AddCommand(listenCmd)
	rootCmd.AddCommand(sendCmd)
	rootCmd.AddCommand(chatCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
