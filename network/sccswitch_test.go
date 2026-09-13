// Copyright 2025 Team 254. All Rights Reserved.
// Author: pat@patfairbank.com (Patrick Fairbank)

package network

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ed25519"
	"golang.org/x/crypto/ssh"
)

func TestConfigureSCC(t *testing.T) {
	username := "username"
	password := "password"
	upCommands := []string{
		"up_line1",
		"up line 2",
		"up-line/3",
		"up line 4",
		"up line 5",
		"exit",
	}
	downCommands := []string{
		"down_line1",
		"down line 2",
		"down-line/3",
		"down line 4",
		"down line 5",
		"exit",
	}
	scc := NewSCCSwitch("127.0.0.1", username, password, upCommands, downCommands)
	scc.port = 9150
	scc.connectTimeoutDuration = 10 * time.Millisecond
	scc.configTimeoutDuration = 15 * time.Millisecond

	var receivedUpCommands, receivedDownCommands []string

	// Set the switch to the down state
	mockSSHSwitch(t, scc.port, username, password, &receivedDownCommands)
	assert.Nil(t, scc.SetTeamEthernetEnabled(false))
	assert.Equal(t, downCommands, receivedDownCommands)
	assert.Equal(t, "DISABLED", scc.Status)

	// Set the switch to the up state
	scc.port += 1
	mockSSHSwitch(t, scc.port, username, password, &receivedUpCommands)
	assert.Nil(t, scc.SetTeamEthernetEnabled(true))
	assert.Equal(t, upCommands, receivedUpCommands)
	assert.Equal(t, "ACTIVE", scc.Status)
}

func mockSSHSwitch(t *testing.T, port int, username, password string, commands *[]string) {
	go func() {
		// Create a simple SSH server that accepts a connection with password authentication
		_, privateKey, err := ed25519.GenerateKey(nil)
		assert.Nil(t, err)
		signer, err := ssh.NewSignerFromKey(privateKey)
		assert.Nil(t, err)
		config := &ssh.ServerConfig{
			PasswordCallback: func(conn ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
				assert.Equal(t, username, conn.User())
				assert.Equal(t, password, string(pass))
				return nil, nil
			},
		}
		config.AddHostKey(signer)
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		assert.Nil(t, err)
		nConn, err := listener.Accept()
		assert.Nil(t, err)
		defer nConn.Close()
		conn, chans, reqs, err := ssh.NewServerConn(nConn, config)
		assert.Nil(t, err)
		defer conn.Close()

		// Ignore all client requests
		go ssh.DiscardRequests(reqs)

		// Wait for the client to connect and request a session
		rawChannel := <-chans
		assert.Equal(t, "session", rawChannel.ChannelType())
		channel, requests, err := rawChannel.Accept()
		assert.Nil(t, err)
		defer channel.Close()

		// Wait for the client to request a PTY
		req := <-requests
		assert.Equal(t, "pty-req", req.Type)
		req.Reply(true, nil)

		// Wait for the client to request a shell
		req = <-requests
		assert.Equal(t, "shell", req.Type)
		req.Reply(true, nil)

		// Read all data sent by the client
		var receivedData bytes.Buffer
		done := make(chan struct{})
		go func() {
			defer close(done)
			buffer := make([]byte, 1024)
			for {
				n, err := channel.Read(buffer)
				if err != nil {
					assert.Equal(t, io.EOF, err)
					break
				}
				receivedData.Write(buffer[:n])
			}
		}()

		select {
		case <-done:
			// Client closed the channel
		case <-time.After(5 * time.Millisecond):
			// All data should be read by now. Close the connection
		}

		*commands = strings.Split(receivedData.String(), "\n")
		if len(*commands) > 0 && (*commands)[len(*commands)-1] == "" {
			*commands = (*commands)[:len(*commands)-1] // Remove trailing newline
		}

		// Send an exit command to cleanly close the session
		channel.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
	}()
	time.Sleep(100 * time.Millisecond) // Give it some time to open the socket.
}

// SCC management is one setting covering both alliances, so an alliance with no SCC switch leaves its address
// blank. Without this it dials "dial tcp :22" and logs an error on every match transition.
func TestSCCSwitchWithoutAddressIsSkipped(t *testing.T) {
	for _, address := range []string{"", "   "} {
		scc := NewSCCSwitch(address, "username", "password", []string{"up"}, []string{"down"})
		assert.False(t, scc.IsEnabled())

		// Would block for the connect timeout if it actually dialed.
		startTime := time.Now()
		assert.Nil(t, scc.SetTeamEthernetEnabled(true))
		assert.Equal(t, "DISABLED", scc.Status)
		assert.Nil(t, scc.SetTeamEthernetEnabled(false))
		assert.Equal(t, "DISABLED", scc.Status)
		assert.Less(t, time.Since(startTime), time.Second)
	}

	assert.True(t, NewSCCSwitch("10.0.100.40", "username", "password", nil, nil).IsEnabled())
}
