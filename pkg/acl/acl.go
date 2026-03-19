package acl

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"tunnel/pkg/logger"
)

type Mode string

const (
	ModeWhitelist Mode = "whitelist"
	ModeBlacklist Mode = "blacklist"
	ModeBoth      Mode = "both"
)

type ACL struct {
	mu        sync.RWMutex
	enabled   bool
	mode      Mode
	whitelist []*net.IPNet
	blacklist []*net.IPNet
	whiteIPs  []net.IP
	blackIPs  []net.IP
}

type Config struct {
	Enable    bool
	Mode      string
	Whitelist []string
	Blacklist []string
}

func New(cfg Config) (*ACL, error) {
	mode := Mode(strings.ToLower(strings.TrimSpace(cfg.Mode)))
	if mode == "" {
		mode = ModeBoth
	}
	switch mode {
	case ModeWhitelist, ModeBlacklist, ModeBoth:
	default:
		return nil, fmt.Errorf("invalid acl mode: %s", cfg.Mode)
	}

	acl := &ACL{
		enabled: cfg.Enable,
		mode:    mode,
	}

	if !cfg.Enable {
		return acl, nil
	}

	for _, item := range cfg.Whitelist {
		if err := acl.addToWhitelist(item); err != nil {
			return nil, fmt.Errorf("invalid whitelist entry '%s': %w", item, err)
		}
	}

	for _, item := range cfg.Blacklist {
		if err := acl.addToBlacklist(item); err != nil {
			return nil, fmt.Errorf("invalid blacklist entry '%s': %w", item, err)
		}
	}

	modeDesc := string(acl.mode)
	if acl.mode == ModeBoth {
		modeDesc = "both (黑名单优先)"
	}
	logger.Printf("[ACL] 初始化完成，模式: %s，白名单: %d 条，黑名单: %d 条",
		modeDesc, len(acl.whitelist)+len(acl.whiteIPs), len(acl.blacklist)+len(acl.blackIPs))

	return acl, nil
}

func (a *ACL) addToWhitelist(item string) error {
	item = strings.TrimSpace(item)
	if item == "" {
		return nil
	}

	if strings.Contains(item, "/") {
		_, ipNet, err := net.ParseCIDR(item)
		if err != nil {
			return err
		}
		a.whitelist = append(a.whitelist, ipNet)
	} else {
		ip := net.ParseIP(item)
		if ip == nil {
			return fmt.Errorf("invalid IP address")
		}
		a.whiteIPs = append(a.whiteIPs, ip)
	}
	return nil
}

func (a *ACL) addToBlacklist(item string) error {
	item = strings.TrimSpace(item)
	if item == "" {
		return nil
	}

	if strings.Contains(item, "/") {
		_, ipNet, err := net.ParseCIDR(item)
		if err != nil {
			return err
		}
		a.blacklist = append(a.blacklist, ipNet)
	} else {
		ip := net.ParseIP(item)
		if ip == nil {
			return fmt.Errorf("invalid IP address")
		}
		a.blackIPs = append(a.blackIPs, ip)
	}
	return nil
}

func (a *ACL) IsAllowed(addr string) bool {
	if !a.enabled {
		return true
	}

	ip := extractIP(addr)
	if ip == nil {
		logger.Printf("[ACL] 无法解析 IP 地址: %s", addr)
		return false
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	switch a.mode {
	case ModeWhitelist:
		allowed := a.isInWhitelist(ip)
		if !allowed {
			logger.Printf("[ACL] 拒绝访问 (不在白名单): %s", addr)
		}
		return allowed

	case ModeBlacklist:
		blocked := a.isInBlacklist(ip)
		if blocked {
			logger.Printf("[ACL] 拒绝访问 (在黑名单中): %s", addr)
		}
		return !blocked

	case ModeBoth:
		if a.isInBlacklist(ip) {
			logger.Printf("[ACL] 拒绝访问 (在黑名单中): %s", addr)
			return false
		}
		hasWhitelist := len(a.whitelist) > 0 || len(a.whiteIPs) > 0
		if hasWhitelist {
			if !a.isInWhitelist(ip) {
				logger.Printf("[ACL] 拒绝访问 (不在白名单): %s", addr)
				return false
			}
		}
		return true

	default:
		return false
	}
}

func (a *ACL) isInWhitelist(ip net.IP) bool {
	for _, wip := range a.whiteIPs {
		if wip.Equal(ip) {
			return true
		}
	}

	for _, ipNet := range a.whitelist {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

func (a *ACL) isInBlacklist(ip net.IP) bool {
	for _, bip := range a.blackIPs {
		if bip.Equal(ip) {
			return true
		}
	}

	for _, ipNet := range a.blacklist {
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}

func (a *ACL) AddWhitelist(item string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.addToWhitelist(item)
}

func (a *ACL) AddBlacklist(item string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.addToBlacklist(item)
}

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

func (a *ACL) SetMode(mode Mode) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.mode = mode
}

func (a *ACL) SetEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = enabled
}

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

func extractIP(addr string) net.IP {
	if ip := net.ParseIP(addr); ip != nil {
		return ip
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil
	}

	return net.ParseIP(host)
}

func NewDisabled() *ACL {
	return &ACL{
		enabled: false,
	}
}
