// Contains the implementation of a LSP client.

package lsp

import "errors"
import "encoding/json"
import "fmt"
import "os"
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
	address			*lspnet.UDPAddr
	currWriteSN		int
	lowestUnackSN 	int
	expectedSN	 	int						// SN we expect to receive next

	allAck			bool					// All sent data messages have been ack

	connectCh		(chan *Message)
	readCh			(chan *Message) 		// data messages to be printed
	writeCh			(chan *Message) 		// data messages to be written to server

	closeCh			(chan int)				// Close() has been called
	isClosed		bool

	intermedReadCh  (chan *Message)
	epochCh			(<-chan time.Time)
	dataWindow		map[int]*Message		// map from SN to *Message of unacknowledged data messages we have sent
	ackWindow		map[int]*Message		// map of the last windowSize acks that we have sent

	numEpochs		int 					// number of epochs that have occurred
	windowSize		int
	epochMilli		int
	epochLimit		int
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

	current_client := &client {
		connID:			0,
		currWriteSN: 	1,
		lowestUnackSN: 	0,
		expectedSN: 	0,							// SN we expect to receive next

		allAck:			false,					// All sent data messages have been ack

		connectCh:		make(chan *Message),
		readCh:			make(chan *Message), 		// data messages to be printed
		writeCh:		make(chan *Message), 		// data messages to be written to server

		closeCh:		make(chan int),				// Close() has been called
		isClosed:		false,

		intermedReadCh: make(chan *Message),
		epochCh:		make(<-chan time.Time),
		dataWindow:		make(map[int]*Message),		// map from SN to *Message of unacknowledged data messages we have sent
		ackWindow:		make(map[int]*Message),		// map of the last windowSize acks that we have sent

		numEpochs:		0, 					// number of epochs that have occurred
		windowSize: 	params.WindowSize,
		epochMilli: 	params.EpochMillis,
		epochLimit: 	params.EpochLimit,
	}

	serverAddr, resolve_err := lspnet.ResolveUDPAddr("udp", hostport)
	current_client.PrintError(resolve_err)

	// Send connect message to server
	current_conn, dial_err := lspnet.DialUDP("udp", nil, serverAddr)
	fmt.Println(hostport)
	current_client.PrintError(dial_err)

	current_client.connection = current_conn
	current_client.address = serverAddr

	// Go routines
	go current_client.master()
	go current_client.read()
	go current_client.epoch()

	// Send connection request to server
	connectMsg := NewConnect()
	connectMsg.ConnID = 0
	connectMsg.SeqNum = 0
	m_msg, marshal_err := json.Marshal(connectMsg)
	current_client.PrintError(marshal_err)
	_, write_msg_err := current_conn.Write(m_msg)
	current_client.PrintError(write_msg_err)

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
	msg := <- c.intermedReadCh
	if msg == nil {
		return nil, errors.New("")
	}
	return msg.Payload, nil
}

func (c *client) Write(payload []byte) error {
	if (!c.isClosed) {
		msg := NewData(c.connID, c.currWriteSN, payload)
		fmt.Println("client is in Write()")
		c.writeCh <- msg
		fmt.Println("client is in Write() 2")
		fmt.Printf("%d\n", len(c.writeCh))
		c.currWriteSN++
	}
	return nil
}

func (c *client) Close() error {
	c.isClosed = true			// TODO: Do we need a channel as opposed to a bool?
	return nil
}

// ===================== GO ROUTINES ================================

func (c *client) master() {
	// We want to exit this for loop once all pending messages to the server
	// have been sent and acknowledged.
	fmt.Println("master is running")
	for {
		select {
		// Check to see if Close() has been called
		case <- c.closeCh:
			c.closeCh <- 1
			return
		case msg := <- c.readCh:
			currentSN := msg.SeqNum
			switch msg.Type {
			case MsgAck:
				// If this is a acknowledgement for a connection request
				if currentSN == 0 {
					c.connectCh <- msg
				} else {
					if _, ok := c.dataWindow[currentSN]; ok {
						delete(c.dataWindow,currentSN)

						// If we received an ack for the oldest unacked data msg
						if currentSN == c.lowestUnackSN {
							if len(c.dataWindow) == 0 {
								c.lowestUnackSN = c.findNewMin(c.dataWindow)
							} else {
								c.lowestUnackSN++
							}
						}

						// if Close() is called and writeCh is empty and dataWindow is empty
						if (c.isClosed && len(c.writeCh) == 0 && len(c.dataWindow) == 0) {
							c.closeCh <- 1
						}
					}
				}
			case MsgData:
				// Drop any message that isn't the expectedSN
				if (currentSN == c.expectedSN) {
					c.intermedReadCh <- msg
					c.expectedSN++

					ackMsg := NewAck(c.connID, currentSN)
					c.sendMessage(ackMsg)

					oldestAckSN := c.findNewMin(c.ackWindow)
					delete(c.ackWindow, oldestAckSN)
					c.ackWindow[currentSN] = ackMsg
				}
			}

		case msg := <- c.writeCh:
			fmt.Println("there is something in the write channel")
			msgSent := false
			// If message cannot be sent, then keep trying until it is sent
			for (!msgSent) {
				// Check if we can write the message based on SN
				fmt.Println("trying to send msg in writeCh")
				if (c.lowestUnackSN <= msg.SeqNum && msg.SeqNum <= c.lowestUnackSN + c.windowSize) {
					fmt.Printf("seq num is %d\n lowerunackSN is %d\n", msg.SeqNum,c.lowestUnackSN)
					// c.sendMessage(msg)
					m_msg, marshal_err := json.Marshal(msg)
					c.PrintError(marshal_err)
					_, write_msg_err := c.connection.Write(m_msg)
					if write_msg_err != nil {
						fmt.Println("poop")
						fmt.Fprintf(os.Stderr, "Client failed to write to the server. Exit code 1.", write_msg_err)
						os.Exit(1)
					}
					fmt.Printf("just sent the msg to the server...?")
					msgSent = true
					c.allAck = false

					// Change the data window to include sent message
					c.dataWindow[msg.SeqNum] = msg
				}
			}

		case <- c.epochCh:
			c.epochHelper()
		}
	}
}

func (c *client) read() {
	for {
		select {
			case <- c.closeCh:
				c.closeCh <- 1 					// very hacky: we are putting it back in order to close the other go routines
				return
			default:
				buff := make([]byte, 1500)
				num_bytes_received, _, received_err := c.connection.ReadFromUDP(buff[0:])
				if received_err != nil {
					fmt.Fprintf(os.Stderr, "Client failed to read from the server. Exit code 2.", received_err)
					os.Exit(2)
				}

				received_msg := Message{}
				unmarshal_err := json.Unmarshal(buff[0:num_bytes_received], &received_msg)
				c.PrintError(unmarshal_err)
				c.readCh <- &received_msg

		}
	}
}

func (c *client) epoch() {
	for {
		select {
		case <- c.closeCh:
			c.closeCh <- 1
			return
		default:
			// Once an epoch has been reached, epochCh is notified
			c.epochCh = time.NewTicker(time.Duration(c.epochMilli) * time.Millisecond).C
		}
	}
}

// ===================== HELPER FUNCTIONS ================================

func (c *client) findNewMin(currentMap map[int]*Message) int {
	currentLowest := MaxInt
	for key, _ := range currentMap {
		if key < currentLowest {
			currentLowest = key
		}
	}
	return currentLowest
}

func (c *client) epochHelper() {
	// If client's connection request hasn't been acknowledged,
	// resent the connection request
	if (c.connID <= 0) {
		if c.numEpochs < c.epochLimit {
			connectMsg := NewConnect()
			connectMsg.ConnID = 0
			connectMsg.SeqNum = 0

			c.sendMessage(connectMsg)
		} else {
			c.Close()
		}

		return
	}

	// If connection request is sent and acknowledged, but no data
	// messages have been received, then send an acknowledgement with
	// seqence number 0
	if c.expectedSN == 0 {
		ackMsg := NewAck(c.connID, 0)
		c.sendMessage(ackMsg)

		c.ackWindow[0] = ackMsg
		return
	}

	// For each data message that has been sent but not yet acknowledged,
	// resend the data message
	for _, value := range c.dataWindow {
		c.sendMessage(value)
	}

	// Resend an acknowledgement message for each of the last w (or fewer)
	// distinct data messages that have been received
	for _, value := range c.ackWindow {
		c.sendMessage(value)
	}

	c.numEpochs++
}

func (c *client) sendMessage(msg *Message) {
	m_msg, marshal_err := json.Marshal(msg)
	c.PrintError(marshal_err)
	_, write_msg_err := c.connection.Write(m_msg)
	if write_msg_err != nil {
		fmt.Fprintf(os.Stderr, "Client failed to write to the server. Exit code 1.", write_msg_err)
		os.Exit(1)
	}
}

func (c *client) PrintError(err error) {
	if err != nil {
		fmt.Println("The error is: ", err)
	}
}

func (c *client) ReturnError(err error, line int) error {
	if err != nil {
		c.PrintError(err)
		return err
	}
	return errors.New("Error")
}
