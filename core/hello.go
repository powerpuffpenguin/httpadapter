package core

import (
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
)

var ErrUnknowProtocol = errors.New("unknow protocol")

type Hello uint8

const (
	HelloOk              Hello = 0
	HelloInvalidProtocol Hello = 1
	HelloInvalidVersion  Hello = 2
	HelloBusy            Hello = 3
	HelloServerError     Hello = 4
	HelloInvalidWindow   Hello = 5
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
	case HelloInvalidWindow:
		return `Invalid Window`
	}
	return `Unknow(` + strconv.Itoa(int(h)) + `)`
}

type HelloError Hello

func (e HelloError) Error() string {
	return Hello(e).String()
}

// 客戶端發送的 hello 消息
type ClientHello struct {
	// Flag string = Flag

	// 客戶端窗口大小
	Window uint32

	// 客戶端支持的 協議版本
	Version []string
}

// 使用 buf 作爲讀取緩衝區，從 Reader 中讀取一個 客戶端發送的 hello 消息
//
// 只有當協議不匹配時，flag 才會存儲已讀取的數據，以便可以傳遞給其它的協議處理
func ReadClientHello(r io.Reader, buf []byte) (hello ClientHello,
	code Hello,
	flag []byte,
	e error,
) {
	flagsize := len(Flag)
	bufsize := len(buf)
	autobuf := bufsize == 0
	if autobuf {
		buf = make([]byte, 256)
		bufsize = len(buf)
	} else if bufsize < flagsize /* || bufsize < 4*/ {
		e = io.ErrShortBuffer
		return
	}
	n, e := io.ReadFull(r, buf[:flagsize])
	if e != nil {
		return
	}
	if BytesToString(buf[:flagsize]) != Flag {
		flag = buf[:n]
		code = HelloInvalidProtocol
		return
	}
	buf = buf[flagsize:]
	_, e = io.ReadFull(r, buf[:6])
	if e != nil {
		return
	}
	window := ByteOrder.Uint32(buf)
	if window < 1 {
		code = HelloInvalidWindow
		return
	}

	vs := int(ByteOrder.Uint16(buf[4:]))
	if vs < 1 {
		code = HelloInvalidVersion
		return
	} else if bufsize < vs {
		if autobuf {
			buf = make([]byte, vs)
		} else {
			e = io.ErrShortBuffer
			return
		}
	}

	buf = buf[:vs]
	_, e = io.ReadFull(r, buf)
	if e != nil {
		return
	}
	hello = ClientHello{
		Window:  window,
		Version: strings.Split(BytesToString(buf), `,`),
	}
	return
}
func (m *ClientHello) verify() (size int, e error) {
	if m.Window == 0 {
		e = HelloError(HelloInvalidWindow)
		return
	}
	vs := len(m.Version)
	if vs == 0 {
		e = HelloError(HelloInvalidVersion)
		return
	} else if vs == 1 {
		vs = 0
	} else {
		vs--
	}
	for _, v := range m.Version {
		vs += len(v)
	}
	size = len(Flag) + 6 + vs
	return
}
func (m *ClientHello) marshalTo(b []byte) (e error) {
	// flag
	n := copy(b, StringToBytes(Flag))
	b = b[n:]

	// window
	ByteOrder.PutUint32(b, m.Window)
	b = b[4:]

	// version
	vs := strings.Join(m.Version, ",")
	if len(vs) > math.MaxUint16 {
		e = HelloError(HelloInvalidVersion)
		return
	}
	ByteOrder.PutUint16(b, uint16(len(vs)))
	b = b[2:]
	copy(b, vs)
	return
}

// 編碼消息到網路傳輸二進制數據
func (m *ClientHello) Marshal() (data []byte, e error) {
	size, e := m.verify()
	if e != nil {
		return
	}
	buf := make([]byte, size)
	e = m.marshalTo(buf)
	if e != nil {
		return
	}
	data = buf
	return
}

// 編碼消息到網路傳輸二進制數據到 b 中
func (m *ClientHello) MarshalTo(b []byte) (data []byte, e error) {
	size, e := m.verify()
	if e != nil {
		return
	}
	if len(b) < size {
		e = io.ErrShortBuffer
		return
	}
	buf := b[:size]
	e = m.marshalTo(buf)
	if e != nil {
		return
	}
	data = buf
	return
}

// 服務器發送的 hello 消息
type ServerHello struct {
	// Flag string = Flag

	// 服務器返回的響應代碼
	Code Hello
	// 服務器窗口大小
	Window uint32
	// 返回的 message 或者 version
	Message string
}

func ReadServerHello(r io.Reader, buf []byte) (hello ServerHello, e error) {
	flagsize := len(Flag)
	bufsize := len(buf)
	autobuf := bufsize == 0
	if autobuf {
		buf = make([]byte, 128)
		bufsize = 128
	} else if bufsize < flagsize /* || bufsize < 5*/ {
		e = io.ErrShortBuffer
		return
	}
	_, e = io.ReadFull(r, buf[:flagsize])
	if e != nil {
		return
	}
	if BytesToString(buf[:flagsize]) != Flag {
		e = ErrUnknowProtocol
		return
	}
	_, e = io.ReadFull(r, buf[:7])
	if e != nil {
		return
	}
	code := Hello(buf[0])
	window := ByteOrder.Uint32(buf[1:])
	if window < 1 {
		e = HelloError(HelloInvalidWindow)
		return
	}

	msglen := int(ByteOrder.Uint16(buf[5:]))
	if bufsize < msglen {
		if autobuf {
			buf = make([]byte, msglen)
		} else {
			e = io.ErrShortBuffer
			return
		}
	} else {
		buf = buf[:msglen]
	}

	var msg string
	if msglen > 0 {
		_, e = io.ReadFull(r, buf)
		if e != nil {
			return
		}
		msg = BytesToString(buf)
	}
	if code == HelloOk && msg == `` {
		e = HelloError(HelloInvalidVersion)
		return
	}

	hello = ServerHello{
		Code:    code,
		Window:  window,
		Message: msg,
	}
	return
}

func (m *ServerHello) verify() (size int, e error) {
	if m.Window == 0 {
		e = HelloError(HelloInvalidWindow)
		return
	}
	if m.Code == HelloOk && m.Message == `` {
		e = HelloError(HelloInvalidVersion)
		return
	} else if len(m.Message) > math.MaxUint16 {
		e = errors.New(`invalid message`)
		return
	}
	size = len(Flag) + 7 + len(m.Message)
	return
}
func (m *ServerHello) marshalTo(b []byte) (e error) {
	// flag
	n := copy(b, StringToBytes(Flag))
	b = b[n:]

	// code
	b[0] = byte(m.Code)
	b = b[1:]

	// window
	ByteOrder.PutUint32(b, m.Window)
	b = b[4:]

	// message
	size := len(m.Message)
	ByteOrder.PutUint16(b, uint16(size))
	if size > 0 {
		b = b[2:]
		copy(b, StringToBytes(m.Message))
	}
	return
}

// 編碼消息到網路傳輸二進制數據
func (m *ServerHello) Marshal() (data []byte, e error) {
	size, e := m.verify()
	if e != nil {
		return
	}
	buf := make([]byte, size)
	e = m.marshalTo(buf)
	if e != nil {
		return
	}
	data = buf
	return
}

// 編碼消息到網路傳輸二進制數據到 b 中
func (m *ServerHello) MarshalTo(b []byte) (data []byte, e error) {
	size, e := m.verify()
	if e != nil {
		return
	}
	if len(b) < size {
		e = io.ErrShortBuffer
		return
	}
	buf := b[:size]
	e = m.marshalTo(buf)
	if e != nil {
		return
	}
	data = buf
	return
}
