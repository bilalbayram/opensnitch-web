package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	pb "github.com/bilalbayram/opensnitch-web/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

type routerAPICredentials struct {
	apiKey string
}

func (c routerAPICredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"x-router-api-key": c.apiKey}, nil
}

func (c routerAPICredentials) RequireTransportSecurity() bool {
	return false
}

var _ credentials.PerRPCCredentials = (*routerAPICredentials)(nil)

func (d *daemon) connect(ctx context.Context) error {
	conn, err := grpc.DialContext(
		ctx,
		d.cfg.GRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithPerRPCCredentials(routerAPICredentials{apiKey: d.cfg.APIKey}),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             20 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return fmt.Errorf("dial grpc: %w", err)
	}

	d.conn = conn
	d.client = pb.NewUIClient(conn)

	if err := d.subscribe(ctx); err != nil {
		return err
	}

	stream, err := d.client.Notifications(ctx)
	if err != nil {
		return fmt.Errorf("open notifications stream: %w", err)
	}
	d.notif = stream
	d.logger.Printf("connected to %s as %s", d.cfg.GRPCAddr, d.cfg.NodeName)
	return nil
}

func (d *daemon) subscribe(ctx context.Context) error {
	_, err := d.client.Subscribe(ctx, &pb.ClientConfig{
		Name:              d.cfg.NodeName,
		Version:           daemonVersion(),
		IsFirewallRunning: d.isFirewallEnabled(),
		Config:            d.configJSON,
		Rules:             d.snapshotRules(),
	})
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	return nil
}

func (d *daemon) askRule(ctx context.Context, flow *localFlow) (*pb.Rule, error) {
	conn := flow.toProto()
	rule, err := d.client.AskRule(ctx, conn)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, fmt.Errorf("AskRule returned no rule")
	}
	if strings.TrimSpace(rule.GetAction()) == "" {
		return nil, fmt.Errorf("AskRule returned rule without action")
	}
	return rule, nil
}

func (d *daemon) runPingLoop(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var seq uint64 = 1
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			stats := d.stats.snapshot(len(d.snapshotRules()))
			if _, err := d.client.Ping(ctx, &pb.PingRequest{
				Id:    seq,
				Stats: stats,
			}); err != nil {
				return fmt.Errorf("ping: %w", err)
			}
			seq++
		}
	}
}

func (d *daemon) runNotifications(ctx context.Context) error {
	for {
		notif, err := d.notif.Recv()
		if err != nil {
			return fmt.Errorf("notifications recv: %w", err)
		}

		reply := &pb.NotificationReply{Id: notif.GetId(), Code: pb.NotificationReplyCode_OK}
		if err := d.handleNotification(notif); err != nil {
			reply.Code = pb.NotificationReplyCode_ERROR
			reply.Data = err.Error()
			d.logger.Printf("notification %d failed: %v", notif.GetId(), err)
		}

		if err := d.notif.Send(reply); err != nil {
			return fmt.Errorf("notifications send reply: %w", err)
		}

		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
}
