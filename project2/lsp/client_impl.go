// Contains the implementation of a LSP client.

package lsp

import "errors"
import "encoding/json"
import "fmt"
// import "os"
import "time"
import "github.com/minhtrangvy/distributed_bitcoin_miner/project2/lspnet"
// import "reflect"

const (
	MaxUint = ^uint(0)
	MaxInt = int(MaxUint >> 1)
)

type client struct {
	connID 			int
	connection		*lspnet.UDPConn
	lowestUnackSN 	int
	expectedSN	 	int						// SN we expect to receive next
	connectCh		(chan *Message)
	readCh			(chan *Message) 		// data messages to be printed
	dataWindow		map[int]*Message		// map from SN to *Message of unacknowledged data messages we have sent
	ackWindow		map[int]*Message		// map of the last windowSize acks that we have sent

	windowSize		int
}

// NewClient creates, initiates, and returns a new client. This function
// should return after a connection with the server has been established
// (i.e., the client has received an Ack message from the server in response
// to its connection request), and should return a non-nil error if a
// connection could not be made (i.e., if after K epochs, the client still
// hasn't received an Ack message from the server in response to its K
// connection requests).
//
// hostport is a colon-separated string identifying the server's host address
// and port number (i.e., "localhost:9999").
func NewClient(hostport string, params *Params) (Client, error) {

	serverAddr, resolve_err := lspnet.ResolveUDPAddr("udp", hostport)
	PrintError(resolve_err, 28)

	// Send connect message to server
	connection, dial_err := lspnet.DialUDP("udp", nil, serverAddr)
	PrintError(dial_err, 59)

	current_client := &client {
	}

	// Go routines
	go current_client.master()
	go current_client.read()
	go current_client.epoch()

	// Send connection request to server
	connectMsg := NewConnect()
	connectMsg.ConnID = 0
	connectMsg.SeqNum = 0
	m_msg, marshal_err := json.Marshal(connectMsg)
	PrintError(marshal_err)
	_, write_msg_err := connection.Write(m_msg)
	PrintError(write_msg_err)

	// Block until we get a connection ack back
	connectAck := <- current_client.connectCh

	// Then return the appropriate client if we get an ack back for the connection request
	if connectAck == nil {
		return nil, errors.New("Could not connect!")
	} else {
		current_client.connID = connectAck.ConnID
		return current_client, nil
	}
}

func (c *client) ConnID() int {
	return c.connID
}

func (c *client) Read() ([]byte, error) {

}

func (c *client) Write(payload []byte) error {
}

func (c *client) Close() error {
}

// ===================== GO ROUTINES ================================

func (c *client) master() {
	select {
	case msg := <- c.readCh:
		currentSN := msg.SeqNum
		switch msg.Type {
		case MsgAck:
			// If this is a acknowledgement for a connection request
			if currentSN == 0 {
				connectCh <- msg
			} else {
				if val, ok := c.dataWindow[currentSN]; ok {
					delete(c.dataWindow,currentSN)
					if currentSN == c.lowestUnackSN {
						c.lowestUnackSN = c.findNewMin(c.dataWindow)
					}
				}
			}
		case MsgData:
			if (currentSN == c.expectedSN) {
				c.readCh <- msg
				c.expectedSN++

				ackMsg := NewAck(c.connID, currentSN)
				m_msg, marshal_err := json.Marshal(ackMsg)
				PrintError(marshal_err)
				_, write_msg_err := connection.Write(m_msg)
				PrintError(write_msg_err)

				oldestAckSN := c.findNewMin(c.ackWindow)
				delete(c.ackWindow, oldestAckSN)
				c.ackWindow[currentSN] = ackMsg
			}
		}
	}
}

func (c *client) read() {
	for {
		buff := make([]byte, 1500)
		num_bytes_received, _, received_err := c.conn.ReadFromUDP(buff[0:])
		PrintError(received_err)

		received_msg := Message{}
		unmarshall_err := json.Unmarshal(buff[0:num_bytes_received], &received_msg)
		PrintError(unmarshall_err)
		c.readCh <- &received_msg
	}
}

func (c *client) epoch() {

}

// ===================== HELPER FUNCTIONS ================================

func (c *client) findNewMin(currentMap map) int {
	currentLowest :=
	for key, _ := range currentMap {
		if key < currentLowest {
			currentLowest = key
		}
	}
	return currentLowest
}

func PrintError(err error) {
	if err != nil {
		fmt.Println("The error is: ", err)
	}
}

func ReturnError(err error, line int) error {
	if err != nil {
		PrintError(err,325)
		return err
	}
	return errors.New("Error")
}
