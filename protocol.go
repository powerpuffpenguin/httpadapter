package httpadapter

import "strconv"

const Flag = "httpadapter"

type Hello uint8

const (
	HelloOk              Hello = 0
	HelloInvalidProtocol Hello = 1
	HelloInvalidVersion  Hello = 2
	HelloBusy            Hello = 3
	HelloServerError     Hello = 4
	HelloWindow          Hello = 5
)

func (h Hello) String() string {
	switch h {
	case HelloOk:
		return `Ok`
	case HelloInvalidProtocol:
		return `Invalid Protocol`
	case HelloInvalidVersion:
		return `Invalid Version`
	case HelloBusy:
		return `Server Busy`
	case HelloServerError:
		return `Server Error`
	case HelloWindow:
		return `Invalid Window`
	}
	return `Unknow Error(` + strconv.Itoa(int(h)) + `)`
}

type Command uint8

const (
	CommandPing    Command = 1
	CommandPong    Command = 2
	CommandCreate  Command = 3
	CommandClose   Command = 4
	CommandWrite   Command = 5
	CommandConfirm Command = 6
)

func (c Command) String() string {
	switch c {
	case CommandPing:
		return `Ping`
	case CommandPong:
		return `Pong`
	case CommandCreate:
		return `Create`
	case CommandClose:
		return `Close`
	case CommandWrite:
		return `Write`
	case CommandConfirm:
		return `Confirm`
	}
	return `Unknow Command(` + strconv.Itoa(int(c)) + `)`
}
