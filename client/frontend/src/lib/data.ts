export type Server = {
  id: string;
  name: string;
  ipAddress: string;
  port: number;
  status: 'online' | 'offline' | 'pending';
  cpuUsage: number;
  memoryUsage: number;
  storageUsage: number;
  metrics: { time: string; cpu: number; memory: number; networkIn: number; networkOut: number }[];
  alerts: string[];
  isGhost?: boolean;
};

export type User = {
  id: number | string;
  name: string;
  username?: string;
  email: string;
  role: string;
  roleId?: number;
  avatar?: string;
  assignedServers: string[]; // array of server IDs
};

export type Node = {
  id: string;
  name: string;
  location: string;
  status: 'online' | 'offline' | 'deploying';
  isLocal: boolean;
  publicHost: string;
  publicPort: number;
  sftpPort: number;
  useDifferentHost: boolean;
  privateHost?: string;
  privatePort?: number;
  systemInfo: {
    os: string;
    architecture: string;
    version: string;
    hostname: string;
    platform: string;
    uptime: string;
    cpu: {
      model: string;
      physicalCores: number;
      logicalCores: number;
      usage: number;
    };
    memory: {
      total: number; // in GB
      used: number; // in MB
      free: number; // in GB
    };
    disks: {
      path: string;
      usage: number;
      size: string;
    }[];
  };
  serversOnNode: { id: string; name: string; type: string; status: 'online' | 'offline' }[];
};


const generateMetrics = () => {
  const metrics = [];
  // Use a fixed UTC date to prevent hydration errors from Date.now() and locale differences.
  const baseDate = new Date(Date.UTC(2024, 6, 26, 10, 30, 0));

  for (let i = 29; i >= 0; i--) {
    const time = new Date(baseDate.getTime() - i * 60000);

    // Format time manually using UTC to avoid locale differences between server and client.
    const hours = time.getUTCHours().toString().padStart(2, '0');
    const minutes = time.getUTCMinutes().toString().padStart(2, '0');

    // Use deterministic values instead of Math.random() to prevent hydration mismatch
    metrics.push({
      time: `${hours}:${minutes}`,
      cpu: Math.floor(Math.sin(i * 0.5) * 35 + 45),
      memory: Math.floor(Math.cos(i * 0.3) * 30 + 60),
      networkIn: Math.floor(Math.sin(i * 0.7) * 15 + 20), // KB/s
      networkOut: Math.floor(Math.cos(i * 0.4) * 10 + 15), // KB/s
    });
  }
  return metrics;
};

export const servers: Server[] = [
  { id: 'srv-1', name: 'Production Web Server', ipAddress: '192.168.1.101', port: 8080, status: 'online', cpuUsage: 75, memoryUsage: 60, storageUsage: 85, metrics: generateMetrics(), alerts: ['[Error] High CPU usage detected on core 3. Process `worker.js` consuming 98% CPU.', 'CRITICAL: Memory usage exceeds 90% threshold. Swap space is being used.'] },
  { id: 'srv-2', name: 'Staging Database', ipAddress: '192.168.1.102', port: 5432, status: 'online', cpuUsage: 40, memoryUsage: 85, storageUsage: 70, metrics: generateMetrics(), alerts: [] },
  { id: 'srv-3', name: 'Analytics Engine', ipAddress: '192.168.1.103', port: 9090, status: 'offline', cpuUsage: 0, memoryUsage: 0, storageUsage: 95, metrics: generateMetrics().map(m => ({ ...m, cpu: 0, memory: 0, networkIn: 0, networkOut: 0 })), alerts: ['ALERT: Server is unreachable. Last seen 1 hour ago.'] },
  { id: 'srv-4', name: 'Cache-01 (Redis)', ipAddress: '192.168.1.104', port: 6379, status: 'pending', cpuUsage: 10, memoryUsage: 30, storageUsage: 20, metrics: generateMetrics(), alerts: [] },
  { id: 'srv-5', name: 'Internal API Gateway', ipAddress: '192.168.1.105', port: 80, status: 'online', cpuUsage: 25, memoryUsage: 45, storageUsage: 50, metrics: generateMetrics(), alerts: ['[Warning] API latency for endpoint /v1/users has increased by 50ms.'] },
  { id: 'srv-6', name: 'Logging Service (ELK)', ipAddress: '192.168.1.106', port: 9200, status: 'online', cpuUsage: 55, memoryUsage: 70, storageUsage: 80, metrics: generateMetrics(), alerts: [] },
];

export const users: User[] = [
  { id: 'usr-1', name: 'Admin User', email: 'admin@aether.panel', role: 'admin', avatar: 'https://picsum.photos/seed/usr-1/40/40', assignedServers: servers.map(s => s.id) },
  { id: 'usr-2', name: 'DevOps Engineer', email: 'devops@aether.panel', role: 'user', avatar: 'https://picsum.photos/seed/usr-2/40/40', assignedServers: ['srv-1', 'srv-2', 'srv-5'] },
  { id: 'usr-3', name: 'Data Analyst', email: 'analyst@aether.panel', role: 'user', avatar: 'https://picsum.photos/seed/usr-3/40/40', assignedServers: ['srv-3'] },
  { id: 'usr-4', name: 'Support Specialist', email: 'support@aether.panel', role: 'user', avatar: 'https://picsum.photos/seed/usr-4/40/40', assignedServers: ['srv-1', 'srv-4'] },
];

export const nodes: Node[] = [
  {
    id: 'node-local', name: 'Nodo Local', location: 'Local', status: 'online', isLocal: true, publicHost: 'localhost', publicPort: 8080, sftpPort: 5657, useDifferentHost: false,
    systemInfo: {
      os: 'GNU/Linux',
      architecture: 'x86 de 64 bits',
      version: 'dev-docker',
      hostname: 'd234f208825c',
      platform: 'alpine 3.23.2',
      uptime: '2d 11h 50m',
      cpu: { model: 'QEMU Virtual CPU version 2.5+', physicalCores: 4, logicalCores: 4, usage: 0 },
      memory: { total: 5.65, used: 543, free: 3.30 },
      disks: [
        { path: '/etc/AetherPanel', usage: 16.6, size: '15.45 GB / 97.87 GB' },
        { path: '/etc/resolv.conf', usage: 16.6, size: '15.45 GB / 97.87 GB' },
        { path: '/etc/hostname', usage: 16.6, size: '15.45 GB / 97.87 GB' },
        { path: '/etc/hosts', usage: 16.6, size: '15.45 GB / 97.87 GB' },
        { path: '/var/lib/AetherPanel', usage: 16.6, size: '15.45 GB / 97.87 GB' },
        { path: '/var/log/AetherPanel', usage: 16.6, size: '15.45 GB / 97.87 GB' },
        { path: '/var/www/AetherPanel', usage: 16.6, size: '15.45 GB / 97.87 GB' },
      ],
    },
    serversOnNode: [
      { id: 'srv-3', name: 'Analytics Engine', type: 'minecraft-java', status: 'offline' }
    ]
  },
  {
    id: 'node-1', name: 'US-East-1', location: 'N. Virginia, USA', status: 'online', isLocal: false, publicHost: '44.204.87.123', publicPort: 8080, sftpPort: 5657, useDifferentHost: false,
    systemInfo: {
      os: 'Ubuntu 22.04',
      architecture: 'ARM64',
      version: 'prod-arm-1',
      hostname: 'prod-us-east-1a',
      platform: 'Ubuntu 22.04.3 LTS',
      uptime: '31d 4h 12m',
      cpu: { model: 'AWS Graviton2', physicalCores: 8, logicalCores: 8, usage: 45 },
      memory: { total: 16, used: 8192, free: 7.2 },
      disks: [
        { path: '/', usage: 60, size: '60 GB / 100 GB' },
        { path: '/data', usage: 20, size: '100 GB / 500 GB' },
      ],
    },
    serversOnNode: [
      { id: 'srv-1', name: 'Production Web Server', type: 'nextjs', status: 'online' },
      { id: 'srv-5', name: 'Internal API Gateway', type: 'go', status: 'online' },
    ]
  },
  {
    id: 'node-2', name: 'EU-West-1', location: 'Ireland', status: 'online', isLocal: false, publicHost: '52.17.200.10', publicPort: 8080, sftpPort: 5657, useDifferentHost: false,
    systemInfo: {
      os: 'CentOS 9',
      architecture: 'x86_64',
      version: 'prod-x86-2',
      hostname: 'prod-eu-west-1b',
      platform: 'CentOS Stream 9',
      uptime: '120d 1h 5m',
      cpu: { model: 'Intel Xeon Platinum 8275CL', physicalCores: 4, logicalCores: 8, usage: 60 },
      memory: { total: 32, used: 24576, free: 6.1 },
      disks: [
        { path: '/', usage: 80, size: '80 GB / 100 GB' },
      ],
    },
    serversOnNode: [
      { id: 'srv-2', name: 'Staging Database', type: 'postgres', status: 'online' },
      { id: 'srv-6', name: 'Logging Service (ELK)', type: 'java', status: 'online' },
    ]
  },
  {
    id: 'node-3', name: 'AP-South-1', location: 'Mumbai, India', status: 'offline', isLocal: false, publicHost: '13.233.15.221', publicPort: 8080, sftpPort: 5657, useDifferentHost: false,
    systemInfo: {
      os: 'Debian 12',
      architecture: 'x86_64',
      version: 'dev-x86-3',
      hostname: 'dev-ap-south-1a',
      platform: 'Debian GNU/Linux 12 (bookworm)',
      uptime: '0d 0h 0m',
      cpu: { model: 'AMD EPYC 7R32', physicalCores: 2, logicalCores: 4, usage: 0 },
      memory: { total: 8, used: 0, free: 8 },
      disks: [
        { path: '/', usage: 10, size: '5 GB / 50 GB' },
      ],
    },
    serversOnNode: []
  },
];
