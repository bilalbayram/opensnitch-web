package api

import (
	"database/sql"
	"strings"

	"github.com/bilalbayram/opensnitch-web/internal/db"
	ruleutil "github.com/bilalbayram/opensnitch-web/internal/rules"
	pb "github.com/bilalbayram/opensnitch-web/proto"
)

func (a *API) isRouterManagedNode(addr string) (bool, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return false, nil
	}

	router, err := a.db.GetRouterByLinkedNodeAddr(addr)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return router.DaemonMode == db.RouterDaemonModeRouterDaemon, nil
}

func (a *API) hasManagedRouterTargets(node string) (bool, error) {
	node = strings.TrimSpace(node)
	if node != "" {
		return a.isRouterManagedNode(node)
	}

	routers, err := a.db.GetRouters()
	if err != nil {
		return false, err
	}
	for _, router := range routers {
		if router.DaemonMode == db.RouterDaemonModeRouterDaemon && strings.TrimSpace(router.LinkedNodeAddr) != "" {
			return true, nil
		}
	}
	return false, nil
}

func (a *API) validateRouterManagedRuleTarget(node string, rule *pb.Rule) error {
	needsValidation, err := a.hasManagedRouterTargets(node)
	if err != nil || !needsValidation {
		return err
	}
	return ruleutil.ValidateRouterManagedRule(rule)
}

func (a *API) validateRouterManagedRuleSet(node string, rules []*pb.Rule) error {
	for _, rule := range rules {
		if err := a.validateRouterManagedRuleTarget(node, rule); err != nil {
			return err
		}
	}
	return nil
}
