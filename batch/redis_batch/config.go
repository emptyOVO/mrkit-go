package redis_batch

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

type ConnConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

func (c *ConnConfig) WithDefaults() {
	if c.Host == "" {
		c.Host = "127.0.0.1"
	}
	if c.Port == 0 {
		c.Port = 6379
	}
}

type client struct {
	conn net.Conn
	rd   *bufio.Reader
}

func openRedis(ctx context.Context, cfg ConnConfig) (*client, error) {
	cfg.WithDefaults()
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	d := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	c := &client{conn: conn, rd: bufio.NewReader(conn)}

	if cfg.Password != "" {
		if _, err := c.do("AUTH", cfg.Password); err != nil {
			conn.Close()
			return nil, err
		}
	}
	if cfg.DB > 0 {
		if _, err := c.do("SELECT", strconv.Itoa(cfg.DB)); err != nil {
			conn.Close()
			return nil, err
		}
	}
	if _, err := c.do("PING"); err != nil {
		conn.Close()
		return nil, err
	}
	return c, nil
}

func (c *client) close() error {
	return c.conn.Close()
}

func (c *client) do(cmd string, args ...string) (interface{}, error) {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, strings.ToUpper(cmd))
	parts = append(parts, args...)

	var b strings.Builder
	b.WriteString("*")
	b.WriteString(strconv.Itoa(len(parts)))
	b.WriteString("\r\n")
	for _, p := range parts {
		b.WriteString("$")
		b.WriteString(strconv.Itoa(len(p)))
		b.WriteString("\r\n")
		b.WriteString(p)
		b.WriteString("\r\n")
	}
	if _, err := c.conn.Write([]byte(b.String())); err != nil {
		return nil, err
	}

	v, err := c.readResp()
	if err != nil {
		return nil, err
	}
	if e, ok := v.(respErr); ok {
		return nil, fmt.Errorf(string(e))
	}
	return v, nil
}

type respErr string

func (c *client) readResp() (interface{}, error) {
	prefix, err := c.rd.ReadByte()
	if err != nil {
		return nil, err
	}
	switch prefix {
	case '+':
		s, err := c.rd.ReadString('\n')
		if err != nil {
			return nil, err
		}
		return strings.TrimSuffix(strings.TrimSuffix(s, "\n"), "\r"), nil
	case '-':
		s, err := c.rd.ReadString('\n')
		if err != nil {
			return nil, err
		}
		return respErr(strings.TrimSuffix(strings.TrimSuffix(s, "\n"), "\r")), nil
	case ':':
		s, err := c.rd.ReadString('\n')
		if err != nil {
			return nil, err
		}
		n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return nil, err
		}
		return n, nil
	case '$':
		s, err := c.rd.ReadString('\n')
		if err != nil {
			return nil, err
		}
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return nil, err
		}
		if n < 0 {
			return nil, nil
		}
		buf := make([]byte, n+2)
		if _, err := io.ReadFull(c.rd, buf); err != nil {
			return nil, err
		}
		return string(buf[:n]), nil
	case '*':
		s, err := c.rd.ReadString('\n')
		if err != nil {
			return nil, err
		}
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return nil, err
		}
		if n < 0 {
			return nil, nil
		}
		out := make([]interface{}, 0, n)
		for i := 0; i < n; i++ {
			v, err := c.readResp()
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unknown redis resp type: %q", string(prefix))
	}
}

func toString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return fmt.Sprint(t)
	}
}
