// Contains the implementation of a LSP client.

package lsp

import "errors"
import "encoding/json"
import "fmt"
// import "os"
import "time"
import "github.com/minhtrangvy/distributed_bitcoin_miner/project2/lspnet"
// import "reflect"

type client struct {
	connID			int
	conn			*lspnet.UDPConn
	connAddr		*lspnet.UDPAddr

	seqNumToSend	int
	expectedSN		int		// the next SN we expect to receive

	connectChan		(chan *Message)

	epochCh			(<-chan time.Time)
	numEpochs 		int
	lastEpoch		int
	epochLimit 		int
	epochMillis 	int

	windowSize 		int
	ackWindow		map[int]Message
	dataWindow 		map[int]Message

	intermediateToRead (chan *Message)
	intermediateToSend (chan *Message)
	toRead			(chan *Message)
	toWrite			(chan *Message)

	closed			bool
	finished		bool
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
	// host, port, split_err = SplitHostPort(hostport)
	serverAddr, resolve_err := lspnet.ResolveUDPAddr("udp", hostport)
	PrintError(resolve_err, 28)

	// Send connect message to server
	connection, dial_err := lspnet.DialUDP("udp", nil, serverAddr)
	PrintError(dial_err, 59)

	current_client := client	{
		connID:			0,
		conn:			connection,
		seqNumToSend:	0,
		expectedSN:		0,		// the next SN we expect to receive

		connectChan:	make(chan *Message, 1),

		numEpochs: 		0,
		lastEpoch:		0,
		epochLimit: 	params.EpochLimit,
		epochMillis: 	params.EpochMillis,

		windowSize: 	params.WindowSize,
		ackWindow:		make(map[int]Message),
		dataWindow: 	make(map[int]Message),

		intermediateToRead: make(chan *Message),
		intermediateToSend: make(chan *Message),
		toRead:			make(chan *Message),
		toWrite:		make(chan *Message),

		closed:			false,
		finished: 		false,
	}

	// for index,_ := range current_client.dataWindow {
	// 	current_client[index].occupiedSlot = 0
	// }

	go current_client.goClient()
	go current_client.goRead()

	// Send connection request to server
	current_client.connectionRequest()

	// Block until we get a connection ack back
	connectAck := <- current_client.connectChan
	curr_client := &current_client
	// Then return the appropriate client
	if connectAck == nil {
		return nil, errors.New("Could not connect!")
	} else {
		current_client.connID = connectAck.ConnID
		return curr_client, nil
	}
}

func (c *client) ConnID() int {
	return c.connID
}

func (c *client) Read() ([]byte, error) {
	msg := <- c.toRead
	if msg == nil {
		return nil, errors.New("(1) the connection has been explicitly closed, or (2) the connection has been lost due to an epoch timeout and no other messages are waiting to be returned.")
	} else {
		return msg.Payload, nil
	}
}

func (c *client) Write(payload []byte) error {
	dataMessage := NewData(c.connID, c.seqNumToSend, payload)

	if !c.closed {
		c.intermediateToSend <- dataMessage
	}
	return nil
}

func (c *client) Close() error {
	c.closed = true
	c.intermediateToRead <- nil
	c.toRead <- nil
	return errors.New("Closing client")
}

// ===================== GO FUNCTIONS ================================

func (c *client) goClient() {
	c.epochCh = time.NewTicker(time.Duration(c.epochMillis)*time.Millisecond).C
	for !(c.closed && c.finished) {
		select {
		case <- c.epochCh:
			c.epoch()
		case msg := <- c.intermediateToRead:
			if msg == nil {
				c.finished = true
				c.toRead <- nil
			} else {
				c.readHelper(*msg)
			}
		case msg := <- c.intermediateToSend:
			sent := false
			for !sent {
				if len(c.dataWindow) < c.windowSize {
					c.writeHelper(*msg)
					sent = true;
				}
			}
		}
	}
	fmt.Println("closing connection")
	c.conn.Close()
}

// always reading from the connection
func (c *client) goRead(){
	for {
		if !(c.closed || c.finished) {
			// received_msg := c.readFromServer()
			buff := make([]byte, 1500)		// since packets should be <= 1500
			num_bytes, _, received_err := c.conn.ReadFromUDP(buff[0:])
			PrintError(received_err,270)

			c.lastEpoch = c.numEpochs

			received_msg := Message{}
			unmarshall_err := json.Unmarshal(buff[0:num_bytes], &received_msg)
			PrintError(unmarshall_err,273)
			c.intermediateToRead <- &received_msg
		} else {
			return
		}
	}
}

// ===================== HELPER FUNCTIONS ================================

func (c *client) writeHelper(msg Message) {
	c.sendToServer(msg)
	c.dataWindow[msg.SeqNum] = msg
}

func (c *client) readHelper(msg Message) {
	switch msg.Type {
	// If we receive a data message, send an ack back
		// Make sure to add it to the ackWindow so we can resend acks during epoch
	case MsgData:
		// Make sure there is a connection
		if c.connID > 0 {
			maxAcceptableSN := c.expectedSN + c.windowSize
			// If not a duplicate, insert into toRead channel for Read to work with
			// Does it have to be within our window?
			if (msg.SeqNum >= c.expectedSN) && (msg.SeqNum <= maxAcceptableSN) {
				if msg.SeqNum == c.expectedSN {
					c.expectedSN++
				}
				c.toRead <- &msg

				// Send back an ack for the message
				ackMsg := NewAck(c.connID,msg.SeqNum)
				c.sendToServer(*ackMsg)
				c.moveAckWindow(*ackMsg)
			}
		} else {
			return
		}
	// If we receive an ack, indicate that the connection has been made if not already
		// If ack sn > 0, make sure that the data msg w/ the same sn leaves the unacked data window (if it was the oldest sn in the window, shift the window, update next expected SN)
	case MsgAck:
		// If this is an ack for the connection request
		if c.connID == 0 {
			c.connectChan <- &msg
			c.connID = msg.ConnID
			c.seqNumToSend = 1
			c.expectedSN = 1
		} else {
			c.deleteFromDataWindow(msg)
		}
	}
}

func (c *client) epoch() {
	c.numEpochs++
	// If no connection, resent connection request
	if c.connID == 0 {
		if (c.numEpochs - c.lastEpoch) > c.epochLimit {
			// lost connection, so close
			c.connectChan <- nil
			c.Close()
		} else {
			c.connectionRequest()
		}
	} else {
		if c.expectedSN == 1 {
			remindAck := NewAck(c.connID, 0)
			c.sendToServer(*remindAck)
		}
		for _, message := range c.dataWindow {
			c.sendToServer(message)
		}
		for _, ack := range c.ackWindow {
			c.sendToServer(ack)
		}
	}
}

func (c *client) connectionRequest() {
	connectMsg := NewConnect()
	connectMsg.ConnID = 0
	connectMsg.SeqNum = 0
	c.sendToServer(*connectMsg)
}

func (c *client) sendToServer(msg Message) {
	m_msg, marshal_err := json.Marshal(msg) // Marshalled connect message
	PrintError(marshal_err, 109)
	_, write_msg_err := c.conn.Write(m_msg)
	PrintError(write_msg_err, 111)
}

// func (c *client) readFromServer() (buffer []byte) {
// 	buff := make([]byte, 1500)		// since packets should be <= 1500
// 	num_bytes, _, received_err := c.conn.ReadFromUDP(buff[0:])
// 	PrintError(received_err,270)
//
// 	c.lastEpoch = c.numEpochs
//
// 	received_msg := Message{}
// 	unmarshall_err := json.Unmarshal(buff[0:num_bytes], &received_msg)
// 	PrintError(unmarshall_err,273)
// 	return &received_msg
// }

func (c *client) moveAckWindow(msg Message) {
	numAcksInWindow := len(c.ackWindow)
	// If the window is full, delete the oldest acknowledgement sent
	if numAcksInWindow == c.windowSize {
		lowestSN := c.findLowestSNinMap("ack")
		delete(c.ackWindow, lowestSN)
	}
	c.ackWindow[msg.SeqNum] = msg
}


func (c *client) deleteFromDataWindow(msg Message) {
	// Delete the data msg that was acked from the dataWindow
	delete(c.dataWindow, msg.SeqNum)
}

func (c *client) findLowestSNinMap(whichMap string) int {
	lowestSN :=  int(^uint(0) >> 1)
	currentMap := c.ackWindow
	if whichMap == "ack" {
		currentMap = c.ackWindow
	} else {
		currentMap = c.dataWindow
	}
	for sn, _ := range currentMap {
		if sn < lowestSN {
			lowestSN = sn
		}
	}
	return lowestSN
}

func PrintError(err error, line int) {
	if err != nil {
		fmt.Println("The error is: ", err, line)
	}
}

func ReturnError(err error, line int) error {
	if err != nil {
		PrintError(err,325)
		return err
	}
	return errors.New("Error")
}
