// The structure of how to add new method types to the system.
// -----------------------------------------------------------
// All methods need 3 things:
//  - A type definition
//  - The type needs a getKind method
//  - The type needs a handler method
// Overall structure example shown below.
//
// ---
// type methodCommandCLICommandRequest struct {
// 	commandOrEvent CommandOrEvent
// }
//
// func (m methodCommandCLICommandRequest) getKind() CommandOrEvent {
// 	return m.commandOrEvent
// }
//
// func (m methodCommandCLICommandRequest) handler(s *server, message Message, node string) ([]byte, error) {
//  ...
//  ...
// 	ackMsg := []byte(fmt.Sprintf("confirmed from node: %v: messageID: %v\n---\n%s---", node, message.ID, out))
// 	return ackMsg, nil
// }
//
// ---
// You also need to make a constant for the Method, and add
// that constant as the key in the map, where the value is
// the actual type you want to map it to with a handler method.
// You also specify if it is a Command or Event, and if it is
// ACK or NACK.
// Check out the existing code below for more examples.

package steward

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

// Method is used to specify the actual function/method that
// is represented in a typed manner.
type Method string

// ------------------------------------------------------------
// The constants that will be used throughout the system for
// when specifying what kind of Method to send or work with.
const (
	// Initial parent method used to start other processes.
	REQInitial Method = "REQInitial"
	// Get a list of all the running processes.
	REQOpProcessList Method = "REQOpProcessList"
	// Start up a process.
	REQOpProcessStart Method = "REQOpProcessStart"
	// Stop up a process.
	REQOpProcessStop Method = "REQOpProcessStop"
	// Execute a CLI command in for example bash or cmd.
	// This is an event type, where a message will be sent to a
	// node with the command to execute and an ACK will be replied
	// if it was delivered succesfully. The output of the command
	// ran will be delivered back to the node where it was initiated
	// as a new message.
	// The data field is a slice of strings where the first string
	// value should be the command, and the following the arguments.
	REQCliCommand Method = "REQCliCommand"
	// REQCliCommandCont same as normal Cli command, but can be used
	// when running a command that will take longer time and you want
	// to send the output of the command continually back as it is
	// generated, and not wait until the command is finished.
	REQCliCommandCont Method = "REQCliCommandCont"
	// Send text to be logged to the console.
	// The data field is a slice of strings where the first string
	// value should be the command, and the following the arguments.
	REQToConsole Method = "REQToConsole"
	// REQTuiToConsole
	REQTuiToConsole Method = "REQTuiToConsole"
	// Send text logging to some host by appending the output to a
	// file, if the file do not exist we create it.
	// A file with the full subject+hostName will be created on
	// the receiving end.
	// The data field is a slice of strings where the values of the
	// slice will be written to the log file.
	REQToFileAppend Method = "REQToFileAppend"
	// Send text to some host by overwriting the existing content of
	// the fileoutput to a file. If the file do not exist we create it.
	// A file with the full subject+hostName will be created on
	// the receiving end.
	// The data field is a slice of strings where the values of the
	// slice will be written to the file.
	REQToFile Method = "REQToFile"
	// REQToFileNACK same as REQToFile but NACK.
	REQToFileNACK Method = "REQToFileNACK"
	// Read the source file to be copied to some node.
	REQCopyFileFrom Method = "REQCopyFileFrom"
	// Write the destination copied to some node.
	REQCopyFileTo Method = "REQCopyFileTo"
	// Send Hello I'm here message.
	REQHello Method = "REQHello"
	// Error log methods to centralError node.
	REQErrorLog Method = "REQErrorLog"
	// Echo request will ask the subscriber for a
	// reply generated as a new message, and sent back to where
	// the initial request was made.
	REQPing Method = "REQPing"
	// Will generate a reply for a ECHORequest
	REQPong Method = "REQPong"
	// Http Get
	REQHttpGet Method = "REQHttpGet"
	// Http Get Scheduled
	// The second element of the MethodArgs slice holds the timer defined in seconds.
	REQHttpGetScheduled Method = "REQHttpGetScheduled"
	// Tail file
	REQTailFile Method = "REQTailFile"
	// Write to steward socket
	REQRelay Method = "REQRelay"
	// The method handler for the first step in a relay chain.
	REQRelayInitial Method = "REQRelayInitial"
	// REQNone is used when there should be no reply.
	REQNone Method = "REQNone"
	// REQTest is used only for testing to be able to grab the output
	// of messages.
	REQTest Method = "REQTest"

	// REQPublicKey will get the public ed25519 key from a node.
	REQPublicKey Method = "REQPublicKey"
	// REQKeysRequestUpdate will get all the public keys from central if an update is available.
	REQKeysRequestUpdate Method = "REQKeysRequestUpdate"
	// REQKeysDeliverUpdate will deliver the public from central to a node.
	REQKeysDeliverUpdate Method = "REQKeysDeliverUpdate"
	// REQKeysAllow
	REQKeysAllow Method = "REQKeysAllow"
	// REQKeysDelete
	REQKeysDelete Method = "REQKeysDelete"

	// REQAclRequestUpdate will get all node acl's from central if an update is available.
	REQAclRequestUpdate Method = "REQAclRequestUpdate"
	// REQAclDeliverUpdate will deliver the acl from central to a node.
	REQAclDeliverUpdate Method = "REQAclDeliverUpdate"

	// REQAclAddCommand
	REQAclAddCommand = "REQAclAddCommand"
	// REQAclDeleteCommand
	REQAclDeleteCommand = "REQAclDeleteCommand"
	// REQAclDeleteSource
	REQAclDeleteSource = "REQAclDeleteSource"
	// REQGroupNodesAddNode
	REQAclGroupNodesAddNode = "REQAclGroupNodesAddNode"
	// REQAclGroupNodesDeleteNode
	REQAclGroupNodesDeleteNode = "REQAclGroupNodesDeleteNode"
	// REQAclGroupNodesDeleteGroup
	REQAclGroupNodesDeleteGroup = "REQAclGroupNodesDeleteGroup"
	// REQAclGroupCommandsAddCommand
	REQAclGroupCommandsAddCommand = "REQAclGroupCommandsAddCommand"
	// REQAclGroupCommandsDeleteCommand
	REQAclGroupCommandsDeleteCommand = "REQAclGroupCommandsDeleteCommand"
	// REQAclGroupCommandsDeleteGroup
	REQAclGroupCommandsDeleteGroup = "REQAclGroupCommandsDeleteGroup"
	// REQAclExport
	REQAclExport = "REQAclExport"
	// REQAclImport
	REQAclImport = "REQAclImport"
)

// The mapping of all the method constants specified, what type
// it references, and the kind if it is an Event or Command, and
// if it is ACK or NACK.
//  Allowed values for the Event field are:
//   - EventACK
//   - EventNack
//
// The primary use of this table is that messages are not able to
// pass the actual type of the request since it is sent as a string,
// so we use the below table to find the actual type based on that
// string type.
func (m Method) GetMethodsAvailable() MethodsAvailable {

	ma := MethodsAvailable{
		Methodhandlers: map[Method]methodHandler{
			REQInitial: methodREQInitial{
				event: EventACK,
			},
			REQOpProcessList: methodREQOpProcessList{
				event: EventACK,
			},
			REQOpProcessStart: methodREQOpProcessStart{
				event: EventACK,
			},
			REQOpProcessStop: methodREQOpProcessStop{
				event: EventACK,
			},
			REQCliCommand: methodREQCliCommand{
				event: EventACK,
			},
			REQCliCommandCont: methodREQCliCommandCont{
				event: EventACK,
			},
			REQToConsole: methodREQToConsole{
				event: EventACK,
			},
			REQTuiToConsole: methodREQTuiToConsole{
				event: EventACK,
			},
			REQToFileAppend: methodREQToFileAppend{
				event: EventACK,
			},
			REQToFile: methodREQToFile{
				event: EventACK,
			},
			REQToFileNACK: methodREQToFile{
				event: EventNACK,
			},
			REQCopyFileFrom: methodREQCopyFileFrom{
				event: EventACK,
			},
			REQCopyFileTo: methodREQCopyFileTo{
				event: EventACK,
			},
			REQHello: methodREQHello{
				event: EventNACK,
			},
			REQErrorLog: methodREQErrorLog{
				event: EventACK,
			},
			REQPing: methodREQPing{
				event: EventACK,
			},
			REQPong: methodREQPong{
				event: EventACK,
			},
			REQHttpGet: methodREQHttpGet{
				event: EventACK,
			},
			REQHttpGetScheduled: methodREQHttpGetScheduled{
				event: EventACK,
			},
			REQTailFile: methodREQTailFile{
				event: EventACK,
			},
			REQRelay: methodREQRelay{
				event: EventACK,
			},
			REQRelayInitial: methodREQRelayInitial{
				event: EventACK,
			},
			REQPublicKey: methodREQPublicKey{
				event: EventACK,
			},
			REQKeysRequestUpdate: methodREQKeysRequestUpdate{
				event: EventNACK,
			},
			REQKeysDeliverUpdate: methodREQKeysDeliverUpdate{
				event: EventNACK,
			},
			REQKeysAllow: methodREQKeysAllow{
				event: EventACK,
			},
			REQKeysDelete: methodREQKeysDelete{
				event: EventACK,
			},

			REQAclRequestUpdate: methodREQAclRequestUpdate{
				event: EventNACK,
			},
			REQAclDeliverUpdate: methodREQAclDeliverUpdate{
				event: EventNACK,
			},

			REQAclAddCommand: methodREQAclAddCommand{
				event: EventACK,
			},
			REQAclDeleteCommand: methodREQAclDeleteCommand{
				event: EventACK,
			},
			REQAclDeleteSource: methodREQAclDeleteSource{
				event: EventACK,
			},
			REQAclGroupNodesAddNode: methodREQAclGroupNodesAddNode{
				event: EventACK,
			},
			REQAclGroupNodesDeleteNode: methodREQAclGroupNodesDeleteNode{
				event: EventACK,
			},
			REQAclGroupNodesDeleteGroup: methodREQAclGroupNodesDeleteGroup{
				event: EventACK,
			},
			REQAclGroupCommandsAddCommand: methodREQAclGroupCommandsAddCommand{
				event: EventACK,
			},
			REQAclGroupCommandsDeleteCommand: methodREQAclGroupCommandsDeleteCommand{
				event: EventACK,
			},
			REQAclGroupCommandsDeleteGroup: methodREQAclGroupCommandsDeleteGroup{
				event: EventACK,
			},
			REQAclExport: methodREQAclExport{
				event: EventACK,
			},
			REQAclImport: methodREQAclImport{
				event: EventACK,
			},
			REQTest: methodREQTest{
				event: EventACK,
			},
		},
	}

	return ma
}

// Reply methods. The slice generated here is primarily used within
// the Stew client for knowing what of the req types are generally
// used as reply methods.
func (m Method) GetReplyMethods() []Method {
	rm := []Method{REQToConsole, REQTuiToConsole, REQCliCommand, REQCliCommandCont, REQToFile, REQToFileAppend, REQNone}
	return rm
}

// getHandler will check the methodsAvailable map, and return the
// method handler for the method given
// as input argument.
func (m Method) getHandler(method Method) methodHandler {
	ma := m.GetMethodsAvailable()
	mh := ma.Methodhandlers[method]

	return mh
}

// getContextForMethodTimeout, will return a context with cancel function
// with the timeout set to the method timeout in the message.
// If the value of timeout is set to -1, we don't want it to stop, so we
// return a context with a timeout set to 200 years.
func getContextForMethodTimeout(ctx context.Context, message Message) (context.Context, context.CancelFunc) {
	// If methodTimeout == -1, which means we don't want a timeout, set the
	// time out to 200 years.
	if message.MethodTimeout == -1 {
		return context.WithTimeout(ctx, time.Hour*time.Duration(8760*200))
	}

	return context.WithTimeout(ctx, time.Second*time.Duration(message.MethodTimeout))
}

// ----

// Initial parent method used to start other processes.
type methodREQInitial struct {
	event Event
}

func (m methodREQInitial) getKind() Event {
	return m.event
}

func (m methodREQInitial) handler(proc process, message Message, node string) ([]byte, error) {
	// proc.procFuncCh <- message
	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// ----

// MethodsAvailable holds a map of all the different method types and the
// associated handler to that method type.
type MethodsAvailable struct {
	Methodhandlers map[Method]methodHandler
}

// Check if exists will check if the Method is defined. If true the bool
// value will be set to true, and the methodHandler function for that type
// will be returned.
func (ma MethodsAvailable) CheckIfExists(m Method) (methodHandler, bool) {
	mFunc, ok := ma.Methodhandlers[m]
	if ok {
		return mFunc, true
	} else {
		return nil, false
	}
}

// newReplyMessage will create and send a reply message back to where
// the original provided message came from. The primary use of this
// function is to report back to a node who sent a message with the
// result of the request method of the original message.
//
// The method to use for the reply message when reporting back should
// be specified within a message in the  replyMethod field. We will
// pick up that value here, and use it as the method for the new
// request message. If no replyMethod is set we default to the
// REQToFileAppend method type.
//
// There will also be a copy of the original message put in the
// previousMessage field. For the copy of the original message the data
// field will be set to nil before the whole message is put in the
// previousMessage field so we don't copy around the original data in
// the reply response when it is not needed anymore.
func newReplyMessage(proc process, message Message, outData []byte) {
	// If REQNone is specified, we don't want to send a reply message
	// so we silently just return without sending anything.
	if message.ReplyMethod == "REQNone" {
		return
	}

	// If no replyMethod is set we default to writing to writing to
	// a log file.
	if message.ReplyMethod == "" {
		message.ReplyMethod = REQToFileAppend
	}

	// Make a copy of the message as it is right now to use
	// in the previous message field, but set the data field
	// to nil so we don't copy around the original data when
	// we don't need to for the reply message.
	thisMsg := message
	thisMsg.Data = nil

	// Create a new message for the reply, and put it on the
	// ringbuffer to be published.
	// TODO: Check that we still got all the fields present that are needed here.
	newMsg := Message{
		ToNode:        message.FromNode,
		FromNode:      message.ToNode,
		Data:          outData,
		Method:        message.ReplyMethod,
		MethodArgs:    message.ReplyMethodArgs,
		MethodTimeout: message.ReplyMethodTimeout,
		IsReply:       true,
		ACKTimeout:    message.ReplyACKTimeout,
		Retries:       message.ReplyRetries,
		Directory:     message.Directory,
		FileName:      message.FileName,

		// Put in a copy of the initial request message, so we can use it's properties if
		// needed to for example create the file structure naming on the subscriber.
		PreviousMessage: &thisMsg,
	}

	sam, err := newSubjectAndMessage(newMsg)
	if err != nil {
		// In theory the system should drop the message before it reaches here.
		er := fmt.Errorf("error: newSubjectAndMessage : %v, message: %v", err, message)
		proc.errorKernel.errSend(proc, message, er)
	}

	proc.toRingbufferCh <- []subjectAndMessage{sam}
}

// selectFileNaming will figure out the correct naming of the file
// structure to use for the reply data.
// It will return the filename, and the tree structure for the folders
// to create.
func selectFileNaming(message Message, proc process) (string, string) {
	var fileName string
	var folderTree string

	switch {
	case message.PreviousMessage == nil:
		// If this was a direct request there are no previous message to take
		// information from, so we use the one that are in the current mesage.
		fileName = message.FileName
		folderTree = filepath.Join(proc.configuration.SubscribersDataFolder, message.Directory, string(message.ToNode))
	case message.PreviousMessage.ToNode != "":
		fileName = message.PreviousMessage.FileName
		folderTree = filepath.Join(proc.configuration.SubscribersDataFolder, message.PreviousMessage.Directory, string(message.PreviousMessage.ToNode))
	case message.PreviousMessage.ToNode == "":
		fileName = message.PreviousMessage.FileName
		folderTree = filepath.Join(proc.configuration.SubscribersDataFolder, message.PreviousMessage.Directory, string(message.FromNode))
	}

	return fileName, folderTree
}

// ------------------------------------------------------------
// Subscriber method handlers
// ------------------------------------------------------------

// The methodHandler interface.
type methodHandler interface {
	handler(proc process, message Message, node string) ([]byte, error)
	getKind() Event
}
