import type { Page } from '@playwright/test';

const MOCK_TOKEN = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.mock';

const nodes = [
  {
    addr: '10.0.1.10:50051',
    hostname: 'gateway-01',
    daemon_version: '1.6.6',
    daemon_uptime: 864000,
    daemon_rules: 24,
    cons: 48721,
    cons_dropped: 312,
    status: 'running',
    last_connection: '2026-03-17T11:42:00Z',
    online: true,
    mode: 'ask',
    tags: ['production', 'gateway'],
    template_sync_pending: false,
    template_sync_error: '',
  },
  {
    addr: '10.0.1.20:50051',
    hostname: 'workstation-02',
    daemon_version: '1.6.6',
    daemon_uptime: 345600,
    daemon_rules: 18,
    cons: 21340,
    cons_dropped: 87,
    status: 'running',
    last_connection: '2026-03-17T11:41:55Z',
    online: true,
    mode: 'silent_allow',
    tags: ['development'],
    template_sync_pending: false,
    template_sync_error: '',
  },
  {
    addr: '10.0.1.30:50051',
    hostname: 'db-server-03',
    daemon_version: '1.6.5',
    daemon_uptime: 1209600,
    daemon_rules: 12,
    cons: 9845,
    cons_dropped: 1203,
    status: 'running',
    last_connection: '2026-03-17T11:40:12Z',
    online: true,
    mode: 'silent_deny',
    tags: ['production', 'database'],
    template_sync_pending: false,
    template_sync_error: '',
  },
];

const connections = [
  { id: 1001, time: '2026-03-17T11:42:00Z', node: '10.0.1.10:50051', protocol: 'tcp', src_ip: '10.0.1.10', src_port: 52341, dst_host: 'api.github.com', dst_ip: '140.82.121.6', dst_port: 443, user_id: 1000, process: '/usr/bin/git', process_args: 'git fetch origin', process_id: 4821, rule: 'allow-git-https', action: 'allow' },
  { id: 1002, time: '2026-03-17T11:41:58Z', node: '10.0.1.20:50051', protocol: 'tcp', src_ip: '10.0.1.20', src_port: 48210, dst_host: 'registry.npmjs.org', dst_ip: '104.16.23.35', dst_port: 443, user_id: 1000, process: '/usr/bin/node', process_args: 'npm install', process_id: 5123, rule: 'allow-npm', action: 'allow' },
  { id: 1003, time: '2026-03-17T11:41:55Z', node: '10.0.1.10:50051', protocol: 'udp', src_ip: '10.0.1.10', src_port: 0, dst_host: 'telemetry.ubuntu.com', dst_ip: '91.189.92.150', dst_port: 443, user_id: 0, process: '/usr/lib/ubuntu-advantage/ua-auto-attach', process_args: '', process_id: 892, rule: 'deny-telemetry', action: 'deny' },
  { id: 1004, time: '2026-03-17T11:41:50Z', node: '10.0.1.30:50051', protocol: 'tcp', src_ip: '10.0.1.30', src_port: 39102, dst_host: 'updates.signal.org', dst_ip: '76.223.92.165', dst_port: 443, user_id: 1000, process: '/opt/Signal/signal-desktop', process_args: '', process_id: 3421, rule: 'allow-signal', action: 'allow' },
  { id: 1005, time: '2026-03-17T11:41:45Z', node: '10.0.1.10:50051', protocol: 'tcp', src_ip: '10.0.1.10', src_port: 41023, dst_host: 'dl.google.com', dst_ip: '142.250.185.238', dst_port: 443, user_id: 0, process: '/usr/bin/google-chrome', process_args: '', process_id: 2910, rule: 'allow-chrome', action: 'allow' },
  { id: 1006, time: '2026-03-17T11:41:40Z', node: '10.0.1.20:50051', protocol: 'tcp', src_ip: '10.0.1.20', src_port: 55012, dst_host: 'tracking.analytics.yahoo.com', dst_ip: '98.136.144.130', dst_port: 443, user_id: 1000, process: '/usr/lib/firefox/firefox', process_args: '', process_id: 6721, rule: 'deny-tracking', action: 'deny' },
  { id: 1007, time: '2026-03-17T11:41:35Z', node: '10.0.1.30:50051', protocol: 'tcp', src_ip: '10.0.1.30', src_port: 33210, dst_host: 'apt.postgresql.org', dst_ip: '87.238.57.227', dst_port: 443, user_id: 0, process: '/usr/bin/apt', process_args: 'apt update', process_id: 1102, rule: 'allow-apt', action: 'allow' },
  { id: 1008, time: '2026-03-17T11:41:30Z', node: '10.0.1.10:50051', protocol: 'tcp', src_ip: '10.0.1.10', src_port: 60123, dst_host: 'slack-msgs.com', dst_ip: '54.192.18.239', dst_port: 443, user_id: 1000, process: '/usr/lib/slack/slack', process_args: '', process_id: 7812, rule: 'allow-slack', action: 'allow' },
  { id: 1009, time: '2026-03-17T11:41:25Z', node: '10.0.1.20:50051', protocol: 'udp', src_ip: '10.0.1.20', src_port: 0, dst_host: 'metrics.ubuntu.com', dst_ip: '91.189.92.152', dst_port: 443, user_id: 0, process: '/usr/lib/snapd/snapd', process_args: '', process_id: 502, rule: 'deny-telemetry', action: 'deny' },
  { id: 1010, time: '2026-03-17T11:41:20Z', node: '10.0.1.30:50051', protocol: 'tcp', src_ip: '10.0.1.30', src_port: 44102, dst_host: 'docker.io', dst_ip: '54.198.211.15', dst_port: 443, user_id: 0, process: '/usr/bin/dockerd', process_args: '', process_id: 1, rule: 'allow-docker', action: 'allow' },
];

const rules = [
  { id: 1, time: '2026-03-10T08:00:00Z', node: '10.0.1.10:50051', name: 'allow-git-https', display_name: 'Allow Git HTTPS', source_kind: 'user', template_id: 0, template_rule_id: 0, enabled: true, precedence: false, action: 'allow', duration: 'always', operator_type: 'simple', operator_sensitive: false, operator_operand: 'process.path', operator_data: '/usr/bin/git', is_compound: false, description: 'Allow git to access HTTPS endpoints', nolog: false, created: '2026-03-10T08:00:00Z' },
  { id: 2, time: '2026-03-10T08:01:00Z', node: '10.0.1.10:50051', name: 'deny-telemetry', display_name: 'Deny Telemetry', source_kind: 'user', template_id: 0, template_rule_id: 0, enabled: true, precedence: true, action: 'deny', duration: 'always', operator_type: 'simple', operator_sensitive: false, operator_operand: 'dest.host', operator_data: '.*telemetry.*|.*metrics.*|.*tracking.*', is_compound: false, description: 'Block all telemetry and tracking domains', nolog: false, created: '2026-03-10T08:01:00Z' },
  { id: 3, time: '2026-03-10T08:02:00Z', node: '10.0.1.20:50051', name: 'allow-npm', display_name: 'Allow npm', source_kind: 'user', template_id: 0, template_rule_id: 0, enabled: true, precedence: false, action: 'allow', duration: 'always', operator_type: 'simple', operator_sensitive: false, operator_operand: 'dest.host', operator_data: 'registry.npmjs.org', is_compound: false, description: 'Allow npm registry access', nolog: false, created: '2026-03-10T08:02:00Z' },
  { id: 4, time: '2026-03-11T10:00:00Z', node: '10.0.1.10:50051', name: 'allow-slack', display_name: 'Allow Slack', source_kind: 'managed', template_id: 1, template_name: 'Productivity Apps', template_rule_id: 1, enabled: true, precedence: false, action: 'allow', duration: 'always', operator_type: 'simple', operator_sensitive: false, operator_operand: 'process.path', operator_data: '/usr/lib/slack/slack', is_compound: false, description: 'Allow Slack desktop app', nolog: false, created: '2026-03-11T10:00:00Z' },
  { id: 5, time: '2026-03-11T10:01:00Z', node: '10.0.1.30:50051', name: 'allow-docker', display_name: 'Allow Docker', source_kind: 'user', template_id: 0, template_rule_id: 0, enabled: true, precedence: false, action: 'allow', duration: 'always', operator_type: 'simple', operator_sensitive: false, operator_operand: 'process.path', operator_data: '/usr/bin/dockerd', is_compound: false, description: 'Allow Docker daemon network access', nolog: true, created: '2026-03-11T10:01:00Z' },
  { id: 6, time: '2026-03-12T14:00:00Z', node: '10.0.1.10:50051', name: 'allow-chrome', display_name: 'Allow Chrome', source_kind: 'user', template_id: 0, template_rule_id: 0, enabled: true, precedence: false, action: 'allow', duration: 'always', operator_type: 'simple', operator_sensitive: false, operator_operand: 'process.path', operator_data: '/usr/bin/google-chrome', is_compound: false, description: 'Allow Google Chrome browser', nolog: true, created: '2026-03-12T14:00:00Z' },
  { id: 7, time: '2026-03-12T14:05:00Z', node: '10.0.1.20:50051', name: 'deny-tracking', display_name: 'Deny Tracking', source_kind: 'managed', template_id: 2, template_name: 'Privacy Shield', template_rule_id: 3, enabled: true, precedence: true, action: 'deny', duration: 'always', operator_type: 'simple', operator_sensitive: false, operator_operand: 'dest.host', operator_data: '.*tracking.*|.*analytics.*', is_compound: false, description: 'Block tracking and analytics domains', nolog: false, created: '2026-03-12T14:05:00Z' },
  { id: 8, time: '2026-03-13T09:00:00Z', node: '10.0.1.30:50051', name: 'allow-apt', display_name: 'Allow APT', source_kind: 'user', template_id: 0, template_rule_id: 0, enabled: false, precedence: false, action: 'allow', duration: 'always', operator_type: 'simple', operator_sensitive: false, operator_operand: 'process.path', operator_data: '/usr/bin/apt', is_compound: false, description: 'Allow system package manager', nolog: false, created: '2026-03-13T09:00:00Z' },
];

const stats = {
  nodes_online: 3,
  connections: 79906,
  accepted: 78301,
  dropped: 1605,
  rules: 24,
  ws_clients: 1,
};

const statsHosts = [
  { what: 'api.github.com', hits: 12450, node: '10.0.1.10:50051' },
  { what: 'registry.npmjs.org', hits: 8920, node: '10.0.1.20:50051' },
  { what: 'dl.google.com', hits: 6340, node: '10.0.1.10:50051' },
  { what: 'slack-msgs.com', hits: 5210, node: '10.0.1.10:50051' },
  { what: 'docker.io', hits: 4890, node: '10.0.1.30:50051' },
  { what: 'apt.postgresql.org', hits: 3210, node: '10.0.1.30:50051' },
  { what: 'updates.signal.org', hits: 2870, node: '10.0.1.30:50051' },
  { what: 'telemetry.ubuntu.com', hits: 1820, node: '10.0.1.10:50051' },
  { what: 'metrics.ubuntu.com', hits: 1105, node: '10.0.1.20:50051' },
  { what: 'tracking.analytics.yahoo.com', hits: 680, node: '10.0.1.20:50051' },
];

const alerts = [
  { id: 1, time: '2026-03-17T10:30:00Z', type: 0, priority: 1, title: 'Node disconnected', message: 'Node db-server-03 (10.0.1.30:50051) lost gRPC connection', node: '10.0.1.30:50051' },
  { id: 2, time: '2026-03-17T09:15:00Z', type: 1, priority: 2, title: 'High deny rate', message: '312 connections denied in the last hour on gateway-01', node: '10.0.1.10:50051' },
  { id: 3, time: '2026-03-17T08:00:00Z', type: 2, priority: 3, title: 'Template synced', message: 'Template "Productivity Apps" synced to 2 nodes', node: '' },
  { id: 4, time: '2026-03-16T22:45:00Z', type: 1, priority: 2, title: 'Unknown process', message: 'Process /tmp/.hidden-bin attempted outbound connection to 185.143.223.1:4444', node: '10.0.1.20:50051' },
  { id: 5, time: '2026-03-16T18:30:00Z', type: 2, priority: 3, title: 'Blocklist updated', message: 'Blocklist "StevenBlack Hosts" synced — 84,210 domains', node: '' },
];

const dnsDomains = [
  { id: 1, domain: 'api.github.com', node: '10.0.1.10:50051', count: 342, first_seen: '2026-03-15T08:00:00Z', last_seen: '2026-03-17T11:42:00Z' },
  { id: 2, domain: 'registry.npmjs.org', node: '10.0.1.20:50051', count: 218, first_seen: '2026-03-15T09:00:00Z', last_seen: '2026-03-17T11:41:58Z' },
  { id: 3, domain: 'docker.io', node: '10.0.1.30:50051', count: 156, first_seen: '2026-03-15T06:00:00Z', last_seen: '2026-03-17T11:41:20Z' },
  { id: 4, domain: 'dl.google.com', node: '10.0.1.10:50051', count: 134, first_seen: '2026-03-16T10:00:00Z', last_seen: '2026-03-17T11:41:45Z' },
  { id: 5, domain: 'slack-msgs.com', node: '10.0.1.10:50051', count: 98, first_seen: '2026-03-16T08:30:00Z', last_seen: '2026-03-17T11:41:30Z' },
  { id: 6, domain: 'telemetry.ubuntu.com', node: '10.0.1.10:50051', count: 67, first_seen: '2026-03-15T04:00:00Z', last_seen: '2026-03-17T11:41:55Z' },
  { id: 7, domain: 'apt.postgresql.org', node: '10.0.1.30:50051', count: 45, first_seen: '2026-03-16T14:00:00Z', last_seen: '2026-03-17T11:41:35Z' },
  { id: 8, domain: 'updates.signal.org', node: '10.0.1.30:50051', count: 32, first_seen: '2026-03-16T12:00:00Z', last_seen: '2026-03-17T11:41:50Z' },
];

const dnsServers = [
  { id: 1, ip: '1.1.1.1', node: '10.0.1.10:50051', queries: 4521, first_seen: '2026-03-15T00:00:00Z', last_seen: '2026-03-17T11:42:00Z' },
  { id: 2, ip: '8.8.8.8', node: '10.0.1.20:50051', queries: 3210, first_seen: '2026-03-15T00:00:00Z', last_seen: '2026-03-17T11:41:58Z' },
  { id: 3, ip: '9.9.9.9', node: '10.0.1.30:50051', queries: 1890, first_seen: '2026-03-15T00:00:00Z', last_seen: '2026-03-17T11:41:35Z' },
];

const blocklists = [
  { id: 1, name: 'StevenBlack Hosts', url: 'https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts', category: 'ads', enabled: true, domain_count: 84210, last_sync: '2026-03-17T06:00:00Z', created_at: '2026-03-10T08:00:00Z' },
  { id: 2, name: 'Energized Protection', url: 'https://energized.pro/unified/formats/hosts', category: 'malware', enabled: true, domain_count: 321450, last_sync: '2026-03-17T06:00:00Z', created_at: '2026-03-10T08:05:00Z' },
  { id: 3, name: 'Pi-hole Telemetry List', url: 'https://v.firebog.net/hosts/Easyprivacy.txt', category: 'telemetry', enabled: true, domain_count: 15230, last_sync: '2026-03-16T18:00:00Z', created_at: '2026-03-11T14:00:00Z' },
  { id: 4, name: 'Malware Domain List', url: 'https://www.malwaredomainlist.com/hostslist/hosts.txt', category: 'malware', enabled: false, domain_count: 1120, last_sync: '2026-03-14T12:00:00Z', created_at: '2026-03-12T09:00:00Z' },
];

const templates = [
  {
    id: 1, name: 'Productivity Apps', description: 'Allow common productivity applications (Slack, VS Code, browsers)',
    created_at: '2026-03-10T08:00:00Z', updated_at: '2026-03-15T10:00:00Z',
    rules: [
      { id: 1, template_id: 1, position: 1, name: 'allow-slack', enabled: true, precedence: false, action: 'allow', duration: 'always', operator_type: 'simple', operator_sensitive: false, operator_operand: 'process.path', operator_data: '/usr/lib/slack/slack', is_compound: false, description: 'Allow Slack', nolog: false, created_at: '2026-03-10T08:00:00Z', updated_at: '2026-03-10T08:00:00Z' },
      { id: 2, template_id: 1, position: 2, name: 'allow-vscode', enabled: true, precedence: false, action: 'allow', duration: 'always', operator_type: 'simple', operator_sensitive: false, operator_operand: 'process.path', operator_data: '/usr/share/code/code', is_compound: false, description: 'Allow VS Code', nolog: false, created_at: '2026-03-10T08:01:00Z', updated_at: '2026-03-10T08:01:00Z' },
    ],
    attachments: [
      { id: 1, template_id: 1, target_type: 'tag', target_ref: 'development', priority: 10, created_at: '2026-03-10T09:00:00Z', updated_at: '2026-03-10T09:00:00Z' },
      { id: 2, template_id: 1, target_type: 'node', target_ref: '10.0.1.10:50051', priority: 5, created_at: '2026-03-11T10:00:00Z', updated_at: '2026-03-11T10:00:00Z' },
    ],
  },
  {
    id: 2, name: 'Privacy Shield', description: 'Block telemetry, tracking, and analytics across all nodes',
    created_at: '2026-03-11T14:00:00Z', updated_at: '2026-03-16T08:00:00Z',
    rules: [
      { id: 3, template_id: 2, position: 1, name: 'deny-tracking', enabled: true, precedence: true, action: 'deny', duration: 'always', operator_type: 'simple', operator_sensitive: false, operator_operand: 'dest.host', operator_data: '.*tracking.*|.*analytics.*', is_compound: false, description: 'Block tracking domains', nolog: false, created_at: '2026-03-11T14:00:00Z', updated_at: '2026-03-11T14:00:00Z' },
      { id: 4, template_id: 2, position: 2, name: 'deny-telemetry', enabled: true, precedence: true, action: 'deny', duration: 'always', operator_type: 'simple', operator_sensitive: false, operator_operand: 'dest.host', operator_data: '.*telemetry.*|.*metrics.*', is_compound: false, description: 'Block telemetry domains', nolog: false, created_at: '2026-03-11T14:01:00Z', updated_at: '2026-03-11T14:01:00Z' },
    ],
    attachments: [
      { id: 3, template_id: 2, target_type: 'tag', target_ref: 'production', priority: 1, created_at: '2026-03-11T14:30:00Z', updated_at: '2026-03-11T14:30:00Z' },
    ],
  },
];

const seenFlows = [
  { id: 1, node: '10.0.1.10:50051', process: '/usr/bin/git', protocol: 'tcp', dst_port: 443, destination_operand: 'dest.host', destination: 'api.github.com', action: 'allow', first_seen: '2026-03-15T08:00:00Z', last_seen: '2026-03-17T11:42:00Z', count: 12450 },
  { id: 2, node: '10.0.1.20:50051', process: '/usr/bin/node', protocol: 'tcp', dst_port: 443, destination_operand: 'dest.host', destination: 'registry.npmjs.org', action: 'allow', first_seen: '2026-03-15T09:00:00Z', last_seen: '2026-03-17T11:41:58Z', count: 8920 },
  { id: 3, node: '10.0.1.10:50051', process: '/usr/lib/ubuntu-advantage/ua-auto-attach', protocol: 'udp', dst_port: 443, destination_operand: 'dest.host', destination: 'telemetry.ubuntu.com', action: 'deny', first_seen: '2026-03-15T04:00:00Z', last_seen: '2026-03-17T11:41:55Z', count: 1820 },
  { id: 4, node: '10.0.1.30:50051', process: '/usr/bin/dockerd', protocol: 'tcp', dst_port: 443, destination_operand: 'dest.host', destination: 'docker.io', action: 'allow', first_seen: '2026-03-15T06:00:00Z', last_seen: '2026-03-17T11:41:20Z', count: 4890 },
  { id: 5, node: '10.0.1.10:50051', process: '/usr/bin/google-chrome', protocol: 'tcp', dst_port: 443, destination_operand: 'dest.host', destination: 'dl.google.com', action: 'allow', first_seen: '2026-03-16T10:00:00Z', last_seen: '2026-03-17T11:41:45Z', count: 6340 },
  { id: 6, node: '10.0.1.20:50051', process: '/usr/lib/firefox/firefox', protocol: 'tcp', dst_port: 443, destination_operand: 'dest.host', destination: 'tracking.analytics.yahoo.com', action: 'deny', first_seen: '2026-03-16T08:00:00Z', last_seen: '2026-03-17T11:41:40Z', count: 680 },
];

const firewall = [
  { name: 'INPUT', policy: 'ACCEPT', rules: 12, node: '10.0.1.10:50051' },
  { name: 'OUTPUT', policy: 'ACCEPT', rules: 24, node: '10.0.1.10:50051' },
  { name: 'FORWARD', policy: 'DROP', rules: 0, node: '10.0.1.10:50051' },
];

const version = {
  current_version: 'v0.5.0',
  build_time: '2026-03-15T10:00:00Z',
  latest_version: 'v0.5.0',
  update_available: false,
  last_check: '2026-03-17T06:00:00Z',
  release: null,
};

export async function setupMocks(page: Page) {
  // Auth
  await page.route('**/api/v1/auth/login', (route) =>
    route.fulfill({ json: { token: MOCK_TOKEN, user: 'admin' } }),
  );
  await page.route('**/api/v1/auth/me', (route) =>
    route.fulfill({ json: { user: 'admin' } }),
  );

  // Nodes
  await page.route('**/api/v1/nodes', (route) => {
    if (route.request().url().includes('/nodes/')) return route.continue();
    return route.fulfill({ json: nodes });
  });

  // Connections
  await page.route('**/api/v1/connections*', (route) =>
    route.fulfill({ json: { data: connections, total: connections.length } }),
  );

  // Seen Flows
  await page.route('**/api/v1/seen-flows*', (route) =>
    route.fulfill({ json: { data: seenFlows, total: seenFlows.length } }),
  );

  // Rules
  await page.route('**/api/v1/rules*', (route) =>
    route.fulfill({ json: rules }),
  );

  // Stats - dashboard
  await page.route('**/api/v1/stats', (route) => {
    if (route.request().url().includes('/stats/')) return route.continue();
    return route.fulfill({ json: stats });
  });

  // Stats - by table
  await page.route('**/api/v1/stats/*', (route) =>
    route.fulfill({ json: statsHosts }),
  );

  // Alerts
  await page.route('**/api/v1/alerts*', (route) =>
    route.fulfill({ json: { data: alerts, total: alerts.length } }),
  );

  // DNS
  await page.route('**/api/v1/dns/domains*', (route) =>
    route.fulfill({ json: { data: dnsDomains, total: dnsDomains.length } }),
  );
  await page.route('**/api/v1/dns/servers*', (route) =>
    route.fulfill({ json: { data: dnsServers, total: dnsServers.length } }),
  );

  // Blocklists
  await page.route('**/api/v1/blocklists*', (route) =>
    route.fulfill({ json: blocklists }),
  );

  // Templates
  await page.route('**/api/v1/templates*', (route) => {
    if (route.request().url().includes('/templates/')) return route.continue();
    return route.fulfill({ json: templates });
  });

  // Firewall
  await page.route('**/api/v1/firewall*', (route) =>
    route.fulfill({ json: firewall }),
  );

  // Prompts
  await page.route('**/api/v1/prompts/pending', (route) =>
    route.fulfill({ json: [] }),
  );

  // Version
  await page.route('**/api/v1/version', (route) =>
    route.fulfill({ json: version }),
  );

  // Process Trust (for nodes page)
  await page.route('**/api/v1/nodes/*/trust*', (route) =>
    route.fulfill({ json: [] }),
  );

  // Block WebSocket upgrade (prevent connection errors)
  await page.route('**/api/v1/ws*', (route) => route.abort());
}

export async function injectAuth(page: Page) {
  await page.addInitScript((token) => {
    localStorage.setItem('token', token);
    localStorage.setItem('user', 'admin');
  }, MOCK_TOKEN);
}
