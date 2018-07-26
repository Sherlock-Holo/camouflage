package frontend

import (
	"fmt"
	"io"
	"net"

	"github.com/Sherlock-Holo/libsocks"
	"github.com/Sherlock-Holo/streamencrypt"
)

type Shadowsocks struct {
	conn        *net.TCPConn
	target      []byte
	readCipher  streamencrypt.Cipher
	writeCipher streamencrypt.Cipher
}

func NewShadowsocks(conn net.Conn, key []byte) (frontend Frontend, err error) {
	cipherInfo := streamencrypt.Ciphers[streamencrypt.CHACHA20_IETF]

	readIV := make([]byte, cipherInfo.IVLen)

	if _, err = io.ReadFull(conn, readIV); err != nil {
		err = fmt.Errorf("read shadowsocks read IV failed: %s", err)
		return
	}

	readCipher, err := cipherInfo.NewCipher(key, readIV)
	if err != nil {
		err = fmt.Errorf("new shadowsocks read cipher failed: %s", err)
		return
	}

	readCipher.InitReader(conn)

	ss := new(Shadowsocks)
	ss.conn = conn.(*net.TCPConn)
	ss.readCipher = readCipher

	writeCipher, err := cipherInfo.NewCipher(key, nil)
	if err != nil {
		err = fmt.Errorf("new shadowsocks write cipher failed: %s", err)
		return
	}

	writeCipher.InitWriter(conn)

	ss.writeCipher = writeCipher

	if _, err = conn.Write(writeCipher.IV()); err != nil {
		err = fmt.Errorf("send shadowsocks write IV failed: %s", err)
		return
	}

	address, err := libsocks.DecodeFrom(ss.readCipher)
	if err != nil {
		err = fmt.Errorf("read shadowsocks address failed: %s", err)
		return
	}

	ss.target = address.Bytes()

	return ss, nil
}

func (ss *Shadowsocks) Read(b []byte) (n int, err error) {
	return ss.readCipher.Read(b)
}

func (ss *Shadowsocks) Write(b []byte) (n int, err error) {
	return ss.writeCipher.Write(b)
}

func (ss *Shadowsocks) CloseWrite() error {
	return ss.conn.CloseWrite()
}

func (ss *Shadowsocks) CloseRead() error {
	return ss.conn.CloseRead()
}

func (ss *Shadowsocks) Close() error {
	return ss.conn.Close()
}

func (ss *Shadowsocks) Target() []byte {
	return ss.target
}

func (ss *Shadowsocks) Handshake(b bool) error {
	if !b {
		return ss.Close()
	}

	return nil
}
