package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
	"strings"

	"github.com/go-redis/redis/v8"
	// "github.com/joho/godotenv"
	"github.com/kaspanet/kaspad/app/appmessage"
	"github.com/kaspanet/kaspad/cmd/kaspawallet/libkaspawallet"
	"github.com/kaspanet/kaspad/infrastructure/network/rpcclient"
	"github.com/kaspanet/kaspad/util"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// KaspaAPI provides access to the Kaspa RPC client and manages block template requests.
type KaspaAPI struct {
	address       string
	blockWaitTime time.Duration
	kaspad        *rpcclient.RPCClient
	connected     bool
}

// BridgeConfig represents the configuration settings used to connect to Kaspa and Redis.
type BridgeConfig struct {
	RPCServer         []string `json:"node"`
	Network           string   `json:"network"`
	BlockWaitTimeMSec string   `json:"block_wait_time_milliseconds"`
	RedisAddress      string   `json:"redis_address"`
	RedisChannel      string   `json:"redis_channel"`
	MinerInfo         string   `json:"miner_info"`
	CanxiumAddr		 string	  `json:"canxiumAddr"`
}

// NewKaspaAPI creates and returns a new KaspaAPI instance with a configured RPC client.
func NewKaspaAPI(address string, blockWaitTime time.Duration) (*KaspaAPI, error) {
	client, err := rpcclient.NewRPCClient(address)
	if err != nil {
		return nil, err
	}

	return &KaspaAPI{
		address:       address,
		blockWaitTime: blockWaitTime,
		kaspad:        client,
		connected:     true,
	}, nil
}

func fetchKaspaAccountFromPrivateKey(network, privateKeyHex string) (string, error) {
	prefix := util.Bech32PrefixKaspa
	if network == "testnet-10" {
		prefix = util.Bech32PrefixKaspaTest
	}

	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return "", err
	}

	publicKeybytes, err := libkaspawallet.PublicKeyFromPrivateKey(privateKeyBytes)
	if err != nil {
		return "", err
	}

	addressPubKey, err := util.NewAddressPublicKey(publicKeybytes, prefix)
	if err != nil {
		return "", err
	}

	address, err := util.DecodeAddress(addressPubKey.String(), prefix)
	if err != nil {
		return "", err
	}

	return address.EncodeAddress(), nil
}

func ProcessCanxiumAddress(address string) string {
	// Remove 0x prefix if present
	if strings.HasPrefix(address, "0x") {
		address = address[2:]
	} else if strings.HasPrefix(strings.ToLower(address), "canxiuminer:0x") {
		// If it has both prefixes, remove the 0x part
		prefix := address[:len("canxiuminer:")]
		addressPart := address[len("canxiuminer:0x"):]
		address = prefix + addressPart
	}

	// Make sure the address has the canxiuminer: prefix
	if !strings.HasPrefix(strings.ToLower(address), "canxiuminer:") {
		address = "canxiuminer:" + address
	}

	return address
}

// GetBlockTemplate fetches a new block template from the Kaspa daemon using the RPC client.
func (ks *KaspaAPI) GetBlockTemplate(miningAddr string, canxiumAddr string, minerInfo string) (*appmessage.GetBlockTemplateResponseMessage, error) {
	template, err := ks.kaspad.GetBlockTemplate(miningAddr,
		fmt.Sprintf(`Katpool/%s`, canxiumAddr))		

	if err != nil {
		return nil, errors.Wrap(err, "failed fetching new block template from kaspa")
	}
	return template, nil
}

func main() {
	// Step 1: Load .env file
	// err := godotenv.Load(".env")
	// if err != nil {
	// 	log.Fatalf("Error loading .env file: %v", err)
	// }

	// Step 2: Read environment variables
	canxiumAddr := os.Getenv("CANXIUM_ADDR") 
	if canxiumAddr == "" {
		fmt.Println("Error: CANXIUM_ADDR is not set")
		os.Exit(1) // Terminate the program with an error code
	}

	fmt.Println("CANXIUM_ADDR:", canxiumAddr)

	privateKey := os.Getenv("TREASURY_PRIVATE_KEY")

	// Open the JSON file
	file, err := os.Open("./config/config.json")
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Error closing config file: %v", err)
		}
	}()

	// Decode JSON into the struct
	var config BridgeConfig
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		fmt.Printf("Error decoding JSON: %v\n", err)
		return
	}
	log.Printf("Config : %v\n", config)

	address, err := fetchKaspaAccountFromPrivateKey(config.Network, privateKey)
	if err != nil {
		log.Fatalf("failed to retrieve address from private key : %v", err)
	}
	log.Printf("Address : %v\n", address)

	// Initialize Redis client
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr: config.RedisAddress,
	})
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Printf("Error closing Redis client: %v", err)
		}
	}()

	// Test Redis connection
	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("could not connect to Redis: %v", err)
	}

	// Initialize Kaspa API
	num, err := strconv.Atoi(config.BlockWaitTimeMSec)
	if err != nil {
		fmt.Println("Error: Invalid BlockWaitTimeMSec : ", err)
		return
	}

	var rpcURL string
	switch config.Network {
	case "testnet-10":
		rpcURL = "kaspad-test10:16210"
	default:
		rpcURL = "kaspad:16110"
	}

	ksAPI, err := NewKaspaAPI(rpcURL, time.Duration(num)*time.Millisecond)
	if err != nil {
		log.Fatalf("failed to initialize Kaspa API: %v", err)
	}

	var templateMutex sync.Mutex
	var currentTemplate *appmessage.GetBlockTemplateResponseMessage

	// Start a goroutine to continuously fetch block templates and publish them to Redis
	go func() {
		for {
			template, err := ksAPI.GetBlockTemplate(address, ProcessCanxiumAddress(config.CanxiumAddr), ProcessCanxiumAddress(config.CanxiumAddr), config.MinerInfo)
			if err != nil {
				log.Printf("error fetching block template: %v", err)
				time.Sleep(ksAPI.blockWaitTime)
				continue
			}

			// Safely store the template
			templateMutex.Lock()
			currentTemplate = template
			templateMutex.Unlock()

			// Serialize the template to JSON
			templateJSON, err := json.Marshal(template)
			if err != nil {
				log.Printf("error serializing template to JSON: %v", err)
				continue
			}

			// Publish the JSON to Redis
			err = rdb.Publish(ctx, config.RedisChannel, templateJSON).Err()
			if err != nil {
				log.Printf("error publishing to Redis: %v", err)
			} else {
				log.Printf("template published to Redis channel %s", config.RedisChannel)
			}

			time.Sleep(ksAPI.blockWaitTime)
		}
	}()

	go func() {
		type HealthResponse struct {
			Status   string            `json:"status"`
			Services map[string]string `json:"services"`
		}

		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			services := map[string]string{}
			status := "ok"

			// Redis check
			func() {
				defer func() {
					if r := recover(); r != nil {
						services["redis"] = "fail"
						status = "fail"
					}
				}()

				if _, err := rdb.Ping(ctx).Result(); err != nil {
					services["redis"] = "fail"
					status = "fail"
				} else {
					services["redis"] = "ok"
				}
			}()

			// Kaspa RPC check
			func() {
				defer func() {
					if r := recover(); r != nil {
						services["kaspa_rpc"] = "fail"
						status = "fail"
					}
				}()

				if _, err := ksAPI.GetBlockTemplate(address, config.MinerInfo); err != nil {
					services["kaspa_rpc"] = "fail"
					status = "fail"
				} else {
					services["kaspa_rpc"] = "ok"
				}
			}()

			// Respond
			resp := HealthResponse{
				Status:   status,
				Services: services,
			}

			w.Header().Set("Content-Type", "application/json")
			if status == "fail" {
				w.WriteHeader(http.StatusServiceUnavailable)
			} else {
				w.WriteHeader(http.StatusOK)
			}
			_ = json.NewEncoder(w).Encode(resp)
		})

		log.Println("Health check endpoint started on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatalf("Failed to start health server: %v", err)
		}
	}()

	// Output block template in the main function
	for {
		time.Sleep(5 * time.Second) // Adjust the frequency of logging as needed

		templateMutex.Lock()
		if currentTemplate != nil {
		} else {
			fmt.Println("No block template fetched yet.")
		}
		templateMutex.Unlock()
	}
}
