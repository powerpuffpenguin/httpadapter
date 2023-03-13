package core

import "strconv"

const Flag = "httpadapter"

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
