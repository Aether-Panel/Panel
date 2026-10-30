export type Server = {
  id: string;
  name: string;
  ipAddress: string;
  port: number;
  ports?: number[];
  status: 'online' | 'offline' | 'pending';
  /** CPU as a share of what the server may use, 0-100 (its limit, or every core of the node) */
  cpuUsage: number;
  /** CPU in percent of one core as the node reports it (200 = two cores), and the server's
   *  allowance in the same unit */
  cpuRaw?: number;
  maxCpu?: number;
  memoryUsage: number;
  storageUsage: number;
  storageUsed: number;
  storageMax: number;
  memoryUsed?: number;
  /** live samples: `ts` in ms, CPU and memory as shares of the server's limits */
  metrics: { ts: number; cpu: number; memory: number; networkIn: number; networkOut: number }[];
  alerts: string[];
  isGhost?: boolean;
  parentServerId?: string;
  totalCpu?: number;
  totalMemory?: number;
  totalDisk?: number;
  suspended?: boolean;
  nodeId?: number | string;
  playersOnline?: number;
  maxPlayers?: number;
  /** The game answered a player query (Minecraft server list ping or Steam A2S). */
  hasPlayerQuery?: boolean;
  /** Names the game shares, when it shares them. */
  playerNames?: string[];
  /** Everything the game reported about its players (map, game name...). */
  playerInfo?: import('./players').PlayerInfo;
  /** Availability for the last 30 days, as reported by `/api/uptime`. */
  uptimePercent?: number;
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
