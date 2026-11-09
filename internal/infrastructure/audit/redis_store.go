package audit

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RedisStore 基于 LPUSH 的审计存储。
// 不引入第三方 redis 客户端，直接实现最小 RESP 协议（仅 LPUSH + PING），
// 保持依赖极简；后续如需更复杂结构可替换为 go-redis。
type RedisStore struct {
	mu   sync.Mutex
	key  string
	addr string
	conn net.Conn
}

// NewRedisStore 创建 redis 审计存储（懒连接）。
func NewRedisStore(addr, key string) (*RedisStore, error) {
	if key == "" {
		key = "k8s-mcp:audit"
	}
	return &RedisStore{addr: addr, key: key}, nil
}

func (s *RedisStore) getConn() (net.Conn, error) {
	if s.conn != nil {
		return s.conn, nil
	}
	conn, err := net.DialTimeout("tcp", s.addr, 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	s.conn = conn
	return conn, nil
}

func (s *RedisStore) closeConn() {
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
}

func (s *RedisStore) Record(_ context.Context, e Entry) error {
	data, err := jsonMarshal(e)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.exec("LPUSH", s.key, data); err != nil {
		s.closeConn() // 连接可能已坏，下次重建
		return fmt.Errorf("redis audit lpush: %w", err)
	}
	return nil
}

// exec 发送 RESP 数组命令并读取简单回复。
func (s *RedisStore) exec(args ...string) error {
	conn, err := s.getConn()
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
	}
	if _, err := conn.Write([]byte(b.String())); err != nil {
		return err
	}
	reply, err := readReply(conn)
	if err != nil {
		return err
	}
	if strings.HasPrefix(reply, "-") {
		return errors.New(strings.TrimPrefix(reply, "-"))
	}
	return nil
}

func (s *RedisStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeConn()
	return nil
}

// respReplyTypes
const (
	respSimpleString = '+'
	respError        = '-'
	respInteger      = ':'
	respBulkString   = '$'
	respArray        = '*'
)

// readReply 读取单条 RESP 回复（足够 LPUSH/PING：简单串/错误/整数/批量串）。
func readReply(conn net.Conn) (string, error) {
	buf := make([]byte, 1)
	if _, err := readFull(conn, buf); err != nil {
		return "", err
	}
	t := buf[0]
	line, err := readLine(conn)
	if err != nil {
		return "", err
	}
	switch t {
	case respSimpleString:
		return "+" + line, nil
	case respError:
		return "-" + line, nil
	case respInteger:
		return ":" + line, nil
	case respBulkString:
		n, err := strconv.Atoi(line)
		if err != nil {
			return "", fmt.Errorf("bad bulk length %q", line)
		}
		if n < 0 {
			return "$-1", nil
		}
		payload := make([]byte, n+2) // data + CRLF
		if _, err := readFull(conn, payload); err != nil {
			return "", err
		}
		return "$" + string(payload[:n]), nil
	default:
		return "", fmt.Errorf("unsupported resp type %q", string(t))
	}
}

func readLine(conn net.Conn) (string, error) {
	var sb strings.Builder
	b := make([]byte, 1)
	for {
		if _, err := readFull(conn, b); err != nil {
			return "", err
		}
		if b[0] == '\n' {
			return strings.TrimSuffix(sb.String(), "\r"), nil
		}
		sb.WriteByte(b[0])
	}
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}
