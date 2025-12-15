package acl

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
)

// Mode ACL 模式
type Mode string

const (
	ModeWhitelist Mode = "whitelist" // 白名单模式：只允许名单内的 IP
	ModeBlacklist Mode = "blacklist" // 黑名单模式：拒绝名单内的 IP
)

// ACL 访问控制列表
type ACL struct {
	mu        sync.RWMutex
	enabled   bool
	mode      Mode
	whitelist []*net.IPNet
	blacklist []*net.IPNet
	whiteIPs  []net.IP
	blackIPs  []net.IP
}

// Config ACL 配置
type Config struct {
	Enable    bool
	Mode      string   // "whitelist" 或 "blacklist"
	Whitelist []string // IP 或 CIDR
	Blacklist []string // IP 或 CIDR
}

// New 创建新的 ACL
func New(cfg Config) (*ACL, error) {
	acl := &ACL{
		enabled: cfg.Enable,
		mode:    Mode(cfg.Mode),
	}

	if !cfg.Enable {
		return acl, nil
	}

	// 解析白名单
	for _, item := range cfg.Whitelist {
		if err := acl.addToWhitelist(item); err != nil {
			return nil, fmt.Errorf("invalid whitelist entry '%s': %w", item, err)
		}
	}

	// 解析黑名单
	for _, item := range cfg.Blacklist {
		if err := acl.addToBlacklist(item); err != nil {
			return nil, fmt.Errorf("invalid blacklist entry '%s': %w", item, err)
		}
	}

	log.Printf("[ACL] ✅ 初始化完成，模式: %s，白名单: %d 条，黑名单: %d 条",
		acl.mode, len(acl.whitelist)+len(acl.whiteIPs), len(acl.blacklist)+len(acl.blackIPs))

	return acl, nil
}

// addToWhitelist 添加到白名单
func (a *ACL) addToWhitelist(item string) error {
	item = strings.TrimSpace(item)
	if item == "" {
		return nil
	}

	if strings.Contains(item, "/") {
		// CIDR 格式
		_, ipNet, err := net.ParseCIDR(item)
		if err != nil {
			return err
		}
		a.whitelist = append(a.whitelist, ipNet)
	} else {
		// 单个 IP
		ip := net.ParseIP(item)
		if ip == nil {
			return fmt.Errorf("invalid IP address")
		}
		a.whiteIPs = append(a.whiteIPs, ip)
	}
	return nil
}

// addToBlacklist 添加到黑名单
func (a *ACL) addToBlacklist(item string) error {
	item = strings.TrimSpace(item)
	if item == "" {
		return nil
	}

	if strings.Contains(item, "/") {
		// CIDR 格式
		_, ipNet, err := net.ParseCIDR(item)
		if err != nil {
			return err
		}
		a.blacklist = append(a.blacklist, ipNet)
	} else {
		// 单个 IP
		ip := net.ParseIP(item)
		if ip == nil {
			return fmt.Errorf("invalid IP address")
		}
		a.blackIPs = append(a.blackIPs, ip)
	}
	return nil
}

// IsAllowed 检查 IP 是否允许访问
func (a *ACL) IsAllowed(addr string) bool {
	if !a.enabled {
		return true
	}

	// 提取 IP 地址
	ip := extractIP(addr)
	if ip == nil {
		log.Printf("[ACL] ⚠️ 无法解析 IP 地址: %s", addr)
		return false
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	switch a.mode {
	case ModeWhitelist:
		// 白名单模式：必须在白名单中
		allowed := a.isInWhitelist(ip)
		if !allowed {
			log.Printf("[ACL] 🚫 拒绝访问 (不在白名单): %s", addr)
		}
		return allowed

	case ModeBlacklist:
		// 黑名单模式：不能在黑名单中
		blocked := a.isInBlacklist(ip)
		if blocked {
			log.Printf("[ACL] 🚫 拒绝访问 (在黑名单中): %s", addr)
		}
		return !blocked

	default:
		return true
	}
}

// isInWhitelist 检查是否在白名单中
func (a *ACL) isInWhitelist(ip net.IP) bool {
	// 检查单个 IP
	for _, wip := range a.whiteIPs {
		if wip.Equal(ip) {
			return true
		}
	}

	// 检查 CIDR
	for _, ipNet := range a.whitelist {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

// isInBlacklist 检查是否在黑名单中
func (a *ACL) isInBlacklist(ip net.IP) bool {
	// 检查单个 IP
	for _, bip := range a.blackIPs {
		if bip.Equal(ip) {
			return true
		}
	}

	// 检查 CIDR
	for _, ipNet := range a.blacklist {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

// AddWhitelist 动态添加白名单
func (a *ACL) AddWhitelist(item string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.addToWhitelist(item)
}

// AddBlacklist 动态添加黑名单
func (a *ACL) AddBlacklist(item string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.addToBlacklist(item)
}

// RemoveWhitelist 从白名单移除
func (a *ACL) RemoveWhitelist(item string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	item = strings.TrimSpace(item)
	if strings.Contains(item, "/") {
		_, target, err := net.ParseCIDR(item)
		if err != nil {
			return
		}
		for i, ipNet := range a.whitelist {
			if ipNet.String() == target.String() {
				a.whitelist = append(a.whitelist[:i], a.whitelist[i+1:]...)
				return
			}
		}
	} else {
		target := net.ParseIP(item)
		if target == nil {
			return
		}
		for i, ip := range a.whiteIPs {
			if ip.Equal(target) {
				a.whiteIPs = append(a.whiteIPs[:i], a.whiteIPs[i+1:]...)
				return
			}
		}
	}
}

// RemoveBlacklist 从黑名单移除
func (a *ACL) RemoveBlacklist(item string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	item = strings.TrimSpace(item)
	if strings.Contains(item, "/") {
		_, target, err := net.ParseCIDR(item)
		if err != nil {
			return
		}
		for i, ipNet := range a.blacklist {
			if ipNet.String() == target.String() {
				a.blacklist = append(a.blacklist[:i], a.blacklist[i+1:]...)
				return
			}
		}
	} else {
		target := net.ParseIP(item)
		if target == nil {
			return
		}
		for i, ip := range a.blackIPs {
			if ip.Equal(target) {
				a.blackIPs = append(a.blackIPs[:i], a.blackIPs[i+1:]...)
				return
			}
		}
	}
}

// SetMode 设置 ACL 模式
func (a *ACL) SetMode(mode Mode) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.mode = mode
}

// SetEnabled 启用/禁用 ACL
func (a *ACL) SetEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = enabled
}

// Stats 返回 ACL 统计信息
func (a *ACL) Stats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]interface{}{
		"enabled":         a.enabled,
		"mode":            a.mode,
		"whitelist_count": len(a.whitelist) + len(a.whiteIPs),
		"blacklist_count": len(a.blacklist) + len(a.blackIPs),
	}
}

// extractIP 从地址字符串中提取 IP
func extractIP(addr string) net.IP {
	// 尝试直接解析为 IP
	if ip := net.ParseIP(addr); ip != nil {
		return ip
	}

	// 尝试作为 host:port 解析
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil
	}

	return net.ParseIP(host)
}

// NewDisabled 创建一个禁用的 ACL
func NewDisabled() *ACL {
	return &ACL{
		enabled: false,
	}
}

